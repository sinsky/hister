package querybuilder

import (
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

var weights = map[string]float64{
	"text":   1,
	"url":    4,
	"domain": 8,
	"title":  12,
}

func Build(s string) query.Query {
	if strings.TrimSpace(s) == "" {
		return query.NewMatchNoneQuery()
	}

	qt, err := Tokenize(s)
	if err != nil {
		return createSimpleQuery(s)
	}

	qs := []query.Query{}

	for _, t := range qt {
		qs = append(qs, getTokenQuery(t))
	}
	return bleve.NewConjunctionQuery(qs...)
}

func createSimpleQuery(s string) query.Query {
	return bleve.NewQueryStringQuery(s)
}

func getTokenQuery(t Token) query.Query {
	switch t.Type {
	case TokenQuoted:
		titleq := bleve.NewPhraseQuery(strings.Fields(t.Value), "title")
		titleq.SetBoost(weights["title"])
		textq := bleve.NewPhraseQuery(strings.Fields(t.Value), "text")
		textq.SetBoost(weights["text"])
		return bleve.NewDisjunctionQuery(titleq, textq)
	case TokenWord:
		var field string
		for f := range weights {
			if strings.HasPrefix(t.Value, f+":") {
				field = f
				break
			}
		}
		if field != "" {
			v := t.Value[len(field)+1:]
			if strings.Contains(v, "*") {
				q := bleve.NewWildcardQuery(v)
				q.SetField(field)
				q.SetBoost(weights[field])
				return q
			}
			q := bleve.NewTermQuery(v)
			q.SetField(field)
			q.SetBoost(weights[field])
			return q
		}

		qs := []query.Query{}
		for _, f := range []string{"title", "text"} {
			if strings.Contains(t.Value, "*") {
				q := bleve.NewWildcardQuery(t.Value)
				q.SetField(f)
				q.SetBoost(weights[f])
				qs = append(qs, q)
			} else {
				q := bleve.NewTermQuery(t.Value)
				q.SetField(f)
				q.SetBoost(weights[f])
				qs = append(qs, q)
			}
		}

		wcq := t.Value
		if !strings.Contains(t.Value, "*") {
			wcq = "*" + wcq + "*"
		}

		urlq := bleve.NewWildcardQuery(wcq)
		urlq.SetField("url")
		urlq.SetBoost(weights["url"])
		qs = append(qs, urlq)

		domainq := bleve.NewWildcardQuery(wcq)
		domainq.SetField("domain")
		domainq.SetBoost(weights["domain"])
		qs = append(qs, domainq)

		return bleve.NewDisjunctionQuery(qs...)
	case TokenAlternation:
		qs := []query.Query{}
		for _, p := range t.Parts {
			qs = append(qs, getTokenQuery(p))
		}
		return bleve.NewDisjunctionQuery(qs...)
	}
	return bleve.NewQueryStringQuery(t.Value)
}
