package indexer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/model"

	"golang.org/x/net/idna"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/single"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/registry"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/highlight"
	"github.com/blevesearch/bleve/v2/search/highlight/format/ansi"
	simpleFragmenter "github.com/blevesearch/bleve/v2/search/highlight/fragmenter/simple"
	simpleHighlighter "github.com/blevesearch/bleve/v2/search/highlight/highlighter/simple"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/blevesearch/bleve/v2/search/searcher"
	index "github.com/blevesearch/bleve_index_api"
	"github.com/rs/zerolog/log"
)

var Version = 0

type indexer struct {
	idx bleve.Index
}

type Query struct {
	Text      string   `json:"text"`
	Highlight string   `json:"highlight"`
	Fields    []string `json:"fields"`
	Limit     int      `json:"limit"`
	Sort      string   `json:"sort"`
	DateFrom  int64    `json:"date_from"`
	DateTo    int64    `json:"date_to"`
	base      query.Query
	boostVal  *query.Boost
	cfg       *config.Config
}

type Document struct {
	URL                string  `json:"url"`
	Domain             string  `json:"domain"`
	HTML               string  `json:"html"`
	Title              string  `json:"title"`
	Text               string  `json:"text"`
	Favicon            string  `json:"favicon"`
	Score              float64 `json:"score"`
	Added              int64   `json:"added"`
	faviconURL         string
	processed          bool
	skipSensitiveCheck bool
}

type Results struct {
	Total           uint64            `json:"total"`
	Query           *Query            `json:"query"`
	Documents       []*Document       `json:"documents"`
	History         []*model.URLCount `json:"history"`
	SearchDuration  string            `json:"search_duration"`
	QuerySuggestion string            `json:"query_suggestion"`
}

var (
	i                   *indexer
	allFields           []string = []string{"url", "title", "text", "favicon", "html", "domain", "added"}
	ErrSensitiveContent          = errors.New("document contains sensitive data")
	sensitiveContentRe  *regexp.Regexp
)

func Init(cfg *config.Config) error {
	sp := make([]string, 0, len(cfg.SensitiveContentPatterns))
	for _, v := range cfg.SensitiveContentPatterns {
		sp = append(sp, v)
	}
	sensitiveContentRe = regexp.MustCompile(fmt.Sprintf("(%s)", strings.Join(sp, "|")))
	idx, err := bleve.Open(cfg.IndexPath())
	if err != nil {
		mapping := createMapping()
		idx, err = bleve.New(cfg.IndexPath(), mapping)
		if err != nil {
			return err
		}
	}
	i = &indexer{
		idx: idx,
	}
	registry.RegisterHighlighter("ansi", invertedAnsiHighlighter)
	registry.RegisterHighlighter("tui", tuiHighlighter)
	return nil
}

func Reindex(idxPath, tmpIdxPath string, rules *config.Rules, skipSensitiveChecks bool) error {
	idx, err := bleve.Open(idxPath)
	if err != nil {
		return err
	}
	mapping := createMapping()
	tmpIdx, err := bleve.New(tmpIdxPath, mapping)
	if err != nil {
		return err
	}
	q := query.NewMatchAllQuery()
	resultNum := 20
	page := 0
	for {
		req := bleve.NewSearchRequest(q)
		req.Size = resultNum
		req.From = page * resultNum
		req.Fields = allFields
		res, err := idx.Search(req)
		if err != nil || len(res.Hits) < 1 {
			break
		}
		for _, h := range res.Hits {
			d := docFromHit(h)
			d.skipSensitiveCheck = skipSensitiveChecks
			if err := d.Process(); err != nil {
				if errors.Is(err, ErrSensitiveContent) {
					log.Warn().Err(err).Str("URL", d.URL).Msg("Skipping document")
					continue
				} else {
					tmpIdx.Close()
					os.RemoveAll(tmpIdxPath)
					return err
				}
			}
			if rules.IsSkip(d.URL) {
				log.Info().Str("URL", d.URL).Msg("Dropping URL that has since been added to skip rules.")
				continue
			}
			// priority/score are updated implicitly by bleve
			if err := tmpIdx.Index(d.URL, d); err != nil {
				tmpIdx.Close()
				os.RemoveAll(tmpIdxPath)
				return err
			}
		}
		page += 1
		log.Debug().Int("Page", page).Msg("Reindexed")
	}
	idx.Close()
	tmpIdx.Close()
	if err := os.RemoveAll(idxPath); err != nil {
		return nil
	}
	return os.Rename(tmpIdxPath, idxPath)
}

func Add(d *Document) error {
	if !d.processed {
		if err := d.Process(); err != nil {
			return err
		}
	}
	return i.idx.Index(d.URL, d)
}

func Delete(u string) error {
	return i.idx.Delete(u)
}

func Search(cfg *config.Config, q *Query) (*Results, error) {
	q.cfg = cfg
	req := bleve.NewSearchRequest(q.create())
	req.Fields = q.Fields

	if q.Limit > 0 {
		req.Size = q.Limit
	} else {
		req.Size = 100
	}

	switch q.Highlight {
	case "HTML":
		req.Highlight = bleve.NewHighlight()
	case "text":
		req.Highlight = bleve.NewHighlightWithStyle("ansi")
	case "tui":
		req.Highlight = bleve.NewHighlightWithStyle("tui")
	}
	switch q.Sort {
	case "domain":
		req.SortBy([]string{"domain"})
	}
	res, err := i.idx.Search(req)
	if err != nil {
		return nil, err
	}
	matches := make([]*Document, len(res.Hits))
	for j, v := range res.Hits {
		d := &Document{
			URL: v.ID,
		}

		if t, ok := v.Fragments["text"]; ok {
			d.Text = t[0]
		}
		if t, ok := v.Fragments["title"]; ok {
			d.Title = t[0]
		} else {
			s, ok := v.Fields["title"].(string)
			if ok {
				d.Title = s
			}
		}
		if i, ok := v.Fields["favicon"].(string); ok {
			d.Favicon = i
		}
		if t, ok := v.Fields["added"].(float64); ok {
			d.Added = int64(t)
		}
		matches[j] = d
	}
	r := &Results{
		Total:     res.Total,
		Query:     q,
		Documents: matches,
	}
	return r, nil
}

func GetByURL(u string) *Document {
	q := query.NewTermQuery(strings.ToLower(u))
	q.SetField("url")
	req := bleve.NewSearchRequest(q)
	req.Fields = allFields
	res, err := i.idx.Search(req)
	if err != nil || len(res.Hits) < 1 {
		return nil
	}
	return docFromHit(res.Hits[0])
}

func (d *Document) Process() error {
	if d.processed {
		return nil
	}
	if !d.skipSensitiveCheck && sensitiveContentRe != nil && sensitiveContentRe.MatchString(d.HTML) {
		log.Debug().Msg("Matching sensitive content: " + strings.Join(sensitiveContentRe.FindAllString(d.HTML, -1), ","))
		return ErrSensitiveContent
	}
	if d.URL == "" {
		return errors.New("missing URL")
	}
	pu, err := url.Parse(d.URL)
	if err != nil {
		return err
	}
	if pu.Scheme == "" || pu.Host == "" {
		return errors.New("invalid URL: missing scheme/host")
	}
	if pu.Fragment != "" {
		pu.Fragment = ""
		d.URL = pu.String()
	}
	d.Added = time.Now().Unix()
	q := pu.Query()
	qChange := false
	for k := range q {
		if k == "utm" || strings.HasPrefix(k, "utm_") {
			qChange = true
			q.Del(k)
		}
	}
	if qChange {
		pu.RawQuery = q.Encode()
		d.URL = pu.String()
	}
	d.Domain = pu.Host
	if err := d.extractHTML(); err != nil {
		return err
	}
	d.Title = html.EscapeString(d.Title)
	d.processed = true
	return nil
}

func Iterate(fn func(*Document)) {
	q := query.NewMatchAllQuery()
	resultNum := 20
	page := 0
	for {
		req := bleve.NewSearchRequest(q)
		req.Size = resultNum
		req.From = page * resultNum
		req.Fields = allFields
		res, err := i.idx.Search(req)
		if err != nil || len(res.Hits) < 1 {
			return
		}
		for _, h := range res.Hits {
			d := docFromHit(h)
			fn(d)
		}
		page += 1
	}
}

func docFromHit(h *search.DocumentMatch) *Document {
	d := &Document{}
	if s, ok := h.Fields["title"].(string); ok {
		d.Title = s
	}
	if s, ok := h.Fields["url"].(string); ok {
		d.URL = s
	}
	if s, ok := h.Fields["text"].(string); ok {
		d.Text = s
	}
	if s, ok := h.Fields["html"].(string); ok {
		d.HTML = s
	}
	if s, ok := h.Fields["favicon"].(string); ok {
		d.Favicon = s
	}
	if s, ok := h.Fields["domain"].(string); ok {
		d.Domain = s
	}
	if t, ok := h.Fields["added"].(float64); ok {
		d.Added = int64(t)
	}
	return d
}

func (d *Document) extractHTML() error {
	for _, e := range extractors {
		if e.Match(d) {
			return e.Extract(d)
		}
	}
	return errors.New("no extractor found")
}

func (d *Document) DownloadFavicon() error {
	if d.faviconURL == "" {
		d.faviconURL = fullURL(d.URL, "/favicon.ico")
	}
	if strings.HasPrefix(d.faviconURL, "data:") {
		d.Favicon = d.faviconURL
		return nil
	}
	cli := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", d.faviconURL, nil)
	req.Header.Set("User-Agent", "Hister")
	if err != nil {
		return err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("invalid status code")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	d.Favicon = fmt.Sprintf("data:%s;base64,%s", resp.Header.Get("Content-Type"), base64.StdEncoding.EncodeToString(data))
	return nil
}

func (q *Query) create() query.Query {
	if len(q.Fields) == 0 {
		q.Fields = allFields
	}

	// add + to phrases to force matching phrases
	qp := strings.Fields(q.Text)

	if len(qp) == 0 {
		q.base = query.NewMatchNoneQuery()
		return q
	}

	domainCandidate := qp[0]

	inQuote := false
	for i, s := range qp {
		if len(s) == 0 {
			continue
		}
		if !inQuote && (s[0] == '-' || s[0] == '+') {
			continue
		}
		if !inQuote {
			qp[i] = "+" + qp[i]
		}
		quotes := strings.Count(s, "\"")
		if quotes%2 == 1 {
			inQuote = !inQuote
		}
	}

	sqs := strings.Join(qp, " ")
	var sq query.Query
	sq = bleve.NewQueryStringQuery(sqs)

	if d, err := idna.Lookup.ToASCII(domainCandidate); err == nil {
		dq := bleve.NewRegexpQuery(fmt.Sprintf(".*%s.*", d))
		dq.SetField("domain")
		dq.SetBoost(100)
		if len(qp) == 1 {
			// match domain part or QueryString in title/text
			sq = bleve.NewDisjunctionQuery(
				sq,
				dq,
			)
		} else {
			// match QueryString in title/text OR (first word as domain part AND the rest of the query as QueryString in title/text)
			sq = bleve.NewDisjunctionQuery(
				sq,
				bleve.NewConjunctionQuery(
					dq,
					bleve.NewQueryStringQuery(strings.Join(qp[1:], "")),
				),
			)
		}
	}

	q.base = sq

	if q.DateFrom != 0 || q.DateTo != 0 {
		if q.DateFrom != 0 && q.DateTo == 0 {
			q.DateTo = time.Now().Unix()
		}
		var min, max *float64
		if q.DateFrom != 0 {
			min = new(float64)
			*min = float64(q.DateFrom)
		}
		if q.DateTo != 0 {
			max = new(float64)
			*max = float64(q.DateTo)
		}
		dateQuery := bleve.NewNumericRangeQuery(min, max)
		dateQuery.SetField("added")
		q.base = bleve.NewConjunctionQuery(q.base, dateQuery)
	}

	return q
}

func createMapping() mapping.IndexMapping {
	im := bleve.NewIndexMapping()
	im.AddCustomAnalyzer("url", map[string]any{
		"type":         custom.Name,
		"char_filters": []string{},
		"tokenizer":    single.Name,
		"token_filters": []string{
			"to_lower",
		},
	})

	fm := bleve.NewTextFieldMapping()
	fm.Store = true
	fm.Index = true
	fm.IncludeTermVectors = true
	fm.IncludeInAll = true

	um := bleve.NewTextFieldMapping()
	um.Analyzer = "url"

	noIdxMap := bleve.NewTextFieldMapping()
	noIdxMap.Index = false

	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("title", fm)
	docMapping.AddFieldMappingsAt("url", um)
	docMapping.AddFieldMappingsAt("domain", um)
	docMapping.AddFieldMappingsAt("text", fm)
	docMapping.AddFieldMappingsAt("favicon", noIdxMap)
	docMapping.AddFieldMappingsAt("html", noIdxMap)
	docMapping.AddFieldMappingsAt("added", bleve.NewNumericFieldMapping())

	im.DefaultMapping = docMapping

	return im
}

func (q *Query) SetBoost(b float64) {
	boost := query.Boost(b)
	q.boostVal = &boost
}

func (q *Query) Boost() float64 {
	return q.boostVal.Value()
}

func (q *Query) ToJSON() []byte {
	r, _ := json.Marshal(q)
	return r
}

func (q *Query) Searcher(ctx context.Context, i index.IndexReader, m mapping.IndexMapping, options search.SearcherOptions) (search.Searcher, error) {
	bs, err := q.base.Searcher(ctx, i, m, options)
	if err != nil {
		return nil, err
	}
	dvReader, err := i.DocValueReader(q.Fields)
	if err != nil {
		return nil, err
	}
	return searcher.NewFilteringSearcher(ctx, bs, q.makeFilter(dvReader)), nil
}

func (q *Query) makeFilter(dvReader index.DocValueReader) searcher.FilterFunc {
	boost := q.Boost()
	return func(sctx *search.SearchContext, d *search.DocumentMatch) bool {
		isPartOfMatch := make(map[string]bool, len(d.FieldTermLocations))
		for _, ftloc := range d.FieldTermLocations {
			isPartOfMatch[ftloc.Field] = true
		}
		seenFields := make(map[string]any, len(d.Fields))
		_ = dvReader.VisitDocValues(d.IndexInternalID, func(field string, term []byte) {
			if _, seen := seenFields[field]; seen {
				return
			}
			seenFields[field] = struct{}{}
			b := q.score(field, term, isPartOfMatch[field])
			d.Score *= boost * b
		})
		return true
	}
}

func (q *Query) score(field string, term []byte, match bool) float64 {
	var s float64 = 1
	if field == "title" && match {
		s *= 10
	}
	if field == "url" && q.cfg.Rules.IsPriority(string(term)) {
		s *= 10
	}
	return s
}

func fullURL(base, u string) string {
	if strings.HasPrefix(u, "data:") {
		return u
	}
	pu, err := url.Parse(u)
	if err != nil {
		return ""
	}
	pb, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return pb.ResolveReference(pu).String()
}

func invertedAnsiHighlighter(config map[string]any, cache *registry.Cache) (highlight.Highlighter, error) {
	fragmenter, err := cache.FragmenterNamed(simpleFragmenter.Name)
	if err != nil {
		return nil, fmt.Errorf("error building fragmenter: %v", err)
	}

	formatter := ansi.NewFragmentFormatter(ansi.Reverse)
	return simpleHighlighter.NewHighlighter(
		fragmenter,
		formatter,
		simpleHighlighter.DefaultSeparator,
	), nil
}

func tuiHighlighter(config map[string]any, cache *registry.Cache) (highlight.Highlighter, error) {
	fragmenter, err := cache.FragmenterNamed(simpleFragmenter.Name)
	if err != nil {
		return nil, fmt.Errorf("error building fragmenter: %v", err)
	}

	// Explicitly split the color (\x1b[38;5;205m) and bold (\x1b[1m) codes
	// this prevents Lipgloss's ANSI parser from choking and dropping the ESC
	// when it attempts to re-apply the Underline style on hover
	seq := "\x1b[38;5;205m\x1b[1m"

	formatter := ansi.NewFragmentFormatter(seq)
	return simpleHighlighter.NewHighlighter(
		fragmenter,
		formatter,
		simpleHighlighter.DefaultSeparator,
	), nil
}
