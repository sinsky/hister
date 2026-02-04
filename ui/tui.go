package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/model"

	"github.com/gorilla/websocket"
	"github.com/jroimartin/gocui"
)

const Banner = `
░█░█░▀█▀░█▀▀░▀█▀░█▀▀░█▀▄
░█▀█░░█░░▀▀█░░█░░█▀▀░█▀▄
░▀░▀░▀▀▀░▀▀▀░░▀░░▀▀▀░▀░▀
`

type tui struct {
	SearchInput *gocui.View
	ResultsView *gocui.View
	conn        *websocket.Conn
	g           *gocui.Gui
}

type singleLineEditor struct {
	editor   gocui.Editor
	callback func()
}

type query struct {
	Text      string `json:"text"`
	Highlight string `json:"highlight"`
}

func newTUI(cfg *config.Config) (*tui, error) {
	t := &tui{}
	return t, t.init(cfg.WebSocketURL())
}

func (e singleLineEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case (ch != 0 || key == gocui.KeySpace) && mod == 0:
		e.editor.Edit(v, key, ch, mod)
		if e.callback != nil {
			e.callback()
		}
		return
	case key == gocui.KeyEnter:
		return
	case key == gocui.KeyArrowRight:
		ox, _ := v.Cursor()
		if ox >= len(v.Buffer())-1 {
			return
		}
	case key == gocui.KeyHome || key == gocui.KeyArrowUp:
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
		return
	case key == gocui.KeyEnd || key == gocui.KeyArrowDown:
		width, _ := v.Size()
		lineWidth := len(v.Buffer()) - 1
		if lineWidth > width {
			v.SetOrigin(lineWidth-width, 0)
			lineWidth = width - 1
		}
		v.SetCursor(lineWidth, 0)
		return
	}
	e.editor.Edit(v, key, ch, mod)
	if e.callback != nil {
		e.callback()
	}
}

func SearchTUI(cfg *config.Config) error {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return err
	}
	defer g.Close()

	g.Cursor = true

	t, err := newTUI(cfg)
	if err != nil {
		return err
	}
	defer t.close()

	t.g = g

	g.SetManagerFunc(t.layout)

	quit := func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	}

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}
	return nil
}

func (t *tui) init(wsurl string) error {
	var err error
	t.conn, _, err = websocket.DefaultDialer.Dial(wsurl, nil)
	if err != nil {
		return err
	}
	go func() {
		for {
			_, msg, err := t.conn.ReadMessage()
			if err != nil {
				// TODO
				return
			}
			t.renderResults(msg)
		}
	}()
	return nil
}

func (t *tui) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	bannerW := 24
	bannerH := 5
	if v, err := g.SetView("banner", maxX/2-bannerW/2-1, 0, maxX/2+bannerW/2, bannerH); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.FgColor = gocui.ColorBlue
		fmt.Fprintf(v, "%s", Banner)
	}
	if v, err := g.SetView("search-input", 5, 5, maxX-6, 7); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Search"
		v.Editable = true
		v.Editor = singleLineEditor{
			editor:   gocui.DefaultEditor,
			callback: t.search,
		}
		t.SearchInput = v
		g.SetCurrentView(v.Name())
	}
	if v, err := g.SetView("results", 0, 8, maxX-2, maxY-1); err != nil {
		v.Frame = false
		v.Wrap = true
		v.Overwrite = true
		t.ResultsView = v
	}
	return nil
}

func (t *tui) search() {
	if t.Query() == "" {
		t.ResultsView.Clear()
		return
	}
	q := query{
		Text:      t.Query(),
		Highlight: "text",
	}
	b, err := json.Marshal(q)
	// TODO error handling
	if err != nil {
		return
	}
	t.conn.WriteMessage(websocket.TextMessage, b)
}

func (t *tui) renderResults(msg []byte) {
	var res *indexer.Results
	if err := json.Unmarshal(msg, &res); err != nil {
		// TODO
		return
	}
	t.g.Update(func(_ *gocui.Gui) error {
		t.ResultsView.Clear()
		if len(res.Documents) == 0 && len(res.History) == 0 {
			if t.Query() != "" {
				fmt.Fprintf(t.ResultsView, "No results found")
			}
			return nil
		}
		for _, r := range res.History {
			t.renderHistoryItem(r)
		}
		for _, r := range res.Documents {
			t.renderResult(r)
		}
		return nil
	})
}

func (t *tui) renderHistoryItem(d *model.URLCount) {
}

func (t *tui) renderResult(d *indexer.Document) {
	//t.ResultsView.FgColor = gocui.ColorDefault | gocui.AttrBold
	fmt.Fprintf(t.ResultsView, "\033[1m%s\033[0m\n", consolidateSpaces(d.Title))
	fmt.Fprintf(t.ResultsView, "\033[34m%s\033[0m\n", d.URL)
	if d.Text != "" {
		fmt.Fprintln(t.ResultsView, consolidateSpaces(d.Text))
	}
	fmt.Fprintln(t.ResultsView, "")
}

func consolidateSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func (t *tui) close() {
	t.conn.Close()
}

func (t *tui) Query() string {
	return strings.TrimSpace(t.SearchInput.Buffer())
}
