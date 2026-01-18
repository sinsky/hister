package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/static"
	"github.com/asciimoo/hister/server/templates"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var tpls map[string]*template.Template
var fs http.Handler

type tArgs map[string]any

type historyItem struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Query string `json:"query"`
}

var tFns = template.FuncMap{
	"FormatDate": func(t time.Time) string { return t.Format("2006-01-02") },
	"FormatTime": func(t time.Time) string { return t.Format("2006-01-02 15:04:05") },
	"ToHTML":     func(s string) template.HTML { return template.HTML(s) },
	"Join":       func(s []string, delim string) string { return strings.Join(s, delim) },
	"Truncate": func(s string, maxLen int) string {
		if len(s) > maxLen {
			return s[:maxLen] + "[..]"
		}
		return s
	},
}

var ws = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type webContext struct {
	Request  *http.Request
	Response http.ResponseWriter
	Config   *config.Config
}

func init() {
	fs = http.StripPrefix("/static/", http.FileServerFS(static.FS))
	tpls = make(map[string]*template.Template)
	addTemplate("index", "layout/base.tpl", "index.tpl")
	addTemplate("add", "layout/base.tpl", "add.tpl")
	addTemplate("rules", "layout/base.tpl", "rules.tpl")
	addTemplate("help", "layout/base.tpl", "help.tpl")
	addTemplate("about", "layout/base.tpl", "about.tpl")
	addTemplate("history", "layout/base.tpl", "history.tpl")
}

func Listen(cfg *config.Config) {
	http.HandleFunc("/", createRouter(cfg))
	log.Info().Str("Address", cfg.Server.Address).Str("URL", cfg.BaseURL("/")).Msg("Starting webserver")
	http.ListenAndServe(cfg.Server.Address, nil)
}

func addTemplate(name string, paths ...string) {
	t, err := template.New("").Funcs(tFns).ParseFS(templates.FS, paths...)
	if err != nil {
		panic(err)
	}
	tpls[name] = t
}

func createRouter(cfg *config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer log.Info().Str("Method", r.Method).Dur("LoadTimeMS", time.Since(start)*1000).Str("URL", r.RequestURI).Msg("WEB")
		c := &webContext{
			Request:  r,
			Response: w,
			Config:   cfg,
		}
		switch r.URL.Path {
		case "/":
			serveIndex(c)
			return
		case "/search":
			serveSearch(c)
			return
		case "/add":
			serveAdd(c)
			return
		case "/rules":
			serveRules(c)
			return
		case "/help":
			serveHelp(c)
			return
		case "/history":
			serveHistory(c)
			return
		case "/delete_alias":
			serveDeleteAlias(c)
			return
		case "/add_alias":
			serveAddAlias(c)
			return
		case "/about":
			serveAbout(c)
			return
		case "/readable":
			serveReadable(c)
			return
		case "/favicon.ico":
			i, err := static.FS.ReadFile("favicon.ico")
			if err != nil {
				serve500(c)
				return
			}
			w.Header().Add("Content-Type", "image/vnd.microsoft.icon")
			w.Write(i)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/static/") {
			fs.ServeHTTP(w, r)
			return
		}
		serve404(c)
	}
}

func serveIndex(c *webContext) {
	c.Render("index", nil)
}

func serveSearch(c *webContext) {
	conn, err := ws.Upgrade(c.Response, c.Request, nil)
	if err != nil {
		serve500(c)
		return
	}
	defer conn.Close()
	for {
		_, q, err := conn.ReadMessage()
		start := time.Now()
		if err != nil {
			log.Error().Err(err).Msg("failed to read websocket message")
			break
		}
		var query *indexer.Query
		err = json.Unmarshal(q, &query)
		if err != nil {
			log.Error().Err(err).Msg("failed to parse query")
			continue
		}
		oq := query.Text
		query.Text = c.Config.Rules.ResolveAliases(query.Text)
		res, err := indexer.Search(c.Config, query)
		if err != nil {
			log.Error().Err(err).Msg("failed to get indexer results")
		}
		if res == nil {
			res = &indexer.Results{}
		}
		hr, err := model.GetURLsByQuery(query.Text)
		if err == nil && len(hr) > 0 {
			res.History = hr
		}
		if oq != "" {
			res.QuerySuggestion = model.GetQuerySuggestion(oq)
		}
		res.SearchDuration = fmt.Sprintf("%v", time.Since(start))
		jr, err := json.Marshal(res)
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal indexer results")
		}
		if err := conn.WriteMessage(websocket.TextMessage, jr); err != nil {
			log.Error().Err(err).Msg("failed to write websocket message")
			break
		}
	}
}

func serveAdd(c *webContext) {
	m := c.Request.Method
	if m == http.MethodGet {
		c.Render("add", nil)
		return
	}
	if m != http.MethodPost {
		serve500(c)
		return
	}
	d := &indexer.Document{}
	jsonData := false
	if strings.Contains(c.Request.Header.Get("Content-Type"), "json") {
		jsonData = true
		err := json.NewDecoder(c.Request.Body).Decode(d)
		if err != nil {
			serve500(c)
			return
		}
	} else {
		err := c.Request.ParseForm()
		if err != nil {
			serve500(c)
			return
		}
		f := c.Request.Form
		d.URL = f.Get("url")
		d.Title = f.Get("title")
		d.Text = f.Get("text")
	}
	if err := d.Process(); err != nil {
		log.Error().Err(err).Str("URL", d.URL).Msg("failed to process document")
		serve500(c)
		return
	}
	if !c.Config.Rules.IsSkip(d.URL) && !strings.HasPrefix(d.URL, c.Config.BaseURL("/")) {
		err := indexer.Add(d)
		if err != nil {
			log.Error().Err(err).Msg("failed to create index")
			serve500(c)
			return
		}
	} else {
		log.Debug().Str("url", d.URL).Msg("skip indexing")
	}
	c.Response.WriteHeader(http.StatusCreated)
	if jsonData {
		return
	}
	c.Render("add", nil)
	return
}

func serveHistory(c *webContext) {
	m := c.Request.Method
	if m == http.MethodGet {
		hs, err := model.GetLatestHistoryItems(40)
		if err != nil {
			serve500(c)
			return
		}
		c.Render("history", tArgs{
			"History": hs,
		})
		return
	}
	if m != http.MethodPost {
		serve500(c)
		return
	}
	h := &historyItem{}
	err := json.NewDecoder(c.Request.Body).Decode(h)
	if err != nil {
		serve500(c)
		return
	}
	err = model.UpdateHistory(strings.TrimSpace(h.Query), strings.TrimSpace(h.URL), strings.TrimSpace(h.Title))
	if err != nil {
		log.Error().Err(err).Msg("failed to update history")
		serve500(c)
		return
	}
}

func serveRules(c *webContext) {
	m := c.Request.Method
	if m == http.MethodGet {
		c.Render("rules", nil)
		return
	}
	if m != http.MethodPost {
		serve500(c)
		return
	}
	err := c.Request.ParseForm()
	if err != nil {
		serve500(c)
		return
	}
	f := c.Request.Form
	c.Config.Rules.Skip.ReStrs = strings.Fields(f.Get("skip"))
	c.Config.Rules.Priority.ReStrs = strings.Fields(f.Get("priority"))
	err = c.Config.SaveRules()
	if err != nil {
		log.Error().Err(err).Msg("failed to save rules")
		serve500(c)
		return
	}
	c.Render("rules", tArgs{
		"Success": "Rules saved",
	})
	return
}

func serveReadable(c *webContext) {
	u := c.Request.URL.Query().Get("url")
	doc := indexer.GetByURL(u)
	if doc == nil {
		serve500(c)
		return
	}
	pu, err := url.Parse(u)
	if err != nil {
		serve500(c)
		return
	}
	r, err := readability.FromReader(strings.NewReader(doc.HTML), pu)
	if err != nil {
		serve500(c)
		return
	}
	r.RenderHTML(c.Response)
}

func serveHelp(c *webContext) {
	c.Render("help", nil)
	return
}

func serveAbout(c *webContext) {
	c.Render("about", nil)
	return
}

func serveAddAlias(c *webContext) {
	err := c.Request.ParseForm()
	if err != nil {
		serve500(c)
		return
	}
	f := c.Request.Form
	if f.Get("alias-keyword") != "" && f.Get("alias-value") != "" {
		c.Config.Rules.Aliases[f.Get("alias-keyword")] = f.Get("alias-value")
	}
	err = c.Config.SaveRules()
	if err != nil {
		log.Error().Err(err).Msg("failed to save rules")
		serve500(c)
		return
	}
	c.Redirect("/rules")
}

func serveDeleteAlias(c *webContext) {
	err := c.Request.ParseForm()
	if err != nil {
		serve500(c)
		return
	}
	a := c.Request.Form.Get("alias")
	if _, ok := c.Config.Rules.Aliases[a]; !ok {
		serve500(c)
		return
	}
	delete(c.Config.Rules.Aliases, a)
	if err := c.Config.SaveRules(); err != nil {
		log.Error().Err(err).Msg("failed to save rules")
		serve500(c)
	}
	c.Redirect("/rules")
}

func serve404(c *webContext) {
	c.Response.WriteHeader(http.StatusNotFound)
}

func serve500(c *webContext) {
	http.Error(c.Response, "Internal Server Error", http.StatusInternalServerError)
}

func (c *webContext) Render(tpl string, args tArgs) {
	if args == nil {
		args = make(tArgs)
	}
	args["Config"] = c.Config
	t, ok := tpls[tpl]
	if !ok {
		log.Error().Str("template", tpl).Msg("template not found")
		serve500(c)
		return
	}
	err := t.ExecuteTemplate(c.Response, "base", args)
	if err != nil {
		log.Error().Err(err).Msg("template render error")
		serve500(c)
		return
	}
}

func (c *webContext) Redirect(u string) {
	http.Redirect(c.Response, c.Request, u, http.StatusFound)
}
