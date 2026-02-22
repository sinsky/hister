package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/model"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/pkg/browser"
)

const Banner = `
░█░█░▀█▀░█▀▀░▀█▀░█▀▀░█▀▄
░█▀█░░█░░▀▀█░░█░░█▀▀░█▀▄
░▀░▀░▀▀▀░▀▀▀░░▀░░▀▀▀░▀░▀
`

var (
	blue       = lipgloss.Color("12")
	white      = lipgloss.Color("15")
	gray       = lipgloss.Color("245")
	red        = lipgloss.Color("9")
	green      = lipgloss.Color("10")
	cyan       = lipgloss.Color("14")
	magenta    = lipgloss.Color("205")
	darkGray   = lipgloss.Color("240")
	yellow     = lipgloss.Color("11")
	lightGray  = lipgloss.Color("244")
	bgSelected = lipgloss.Color("236")

	bannerStyle = lipgloss.NewStyle().
			Foreground(blue).
			Bold(true).
			Align(lipgloss.Center)

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(white)
	urlStyle     = lipgloss.NewStyle().Foreground(blue)
	histStyle    = lipgloss.NewStyle().Foreground(yellow)
	selTitle     = lipgloss.NewStyle().Bold(true).Foreground(blue)
	grayStyle    = lipgloss.NewStyle().Foreground(gray)
	secTextStyle = lipgloss.NewStyle().Foreground(lightGray).Faint(true).Italic(true)
	dialogStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(red).Padding(1, 2)
	helpStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(blue).Padding(1, 2)
	statusStyle  = lipgloss.NewStyle().Foreground(white)
	connStyle    = lipgloss.NewStyle().Foreground(green).Bold(true)
	discStyle    = lipgloss.NewStyle().Foreground(red).Bold(true)
	focusStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(blue)
	blurStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(darkGray)

	thumbStyle = lipgloss.NewStyle().Foreground(magenta).Bold(true)
	trackStyle = lipgloss.NewStyle().Foreground(darkGray)

	modeStyles = map[viewState]lipgloss.Style{
		stateInput:   lipgloss.NewStyle().Foreground(yellow).Bold(true),
		stateResults: lipgloss.NewStyle().Foreground(blue).Bold(true),
		stateHelp:    lipgloss.NewStyle().Foreground(cyan).Bold(true),
		stateDialog:  lipgloss.NewStyle().Foreground(red).Bold(true),
	}

	itemStyle         = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderForeground(blue).
				PaddingLeft(1)

	loadMoreStyle         = lipgloss.NewStyle().Foreground(yellow).Bold(true)
	loadMoreSelectedStyle = lipgloss.NewStyle().
				Foreground(yellow).
				Bold(true).
				Background(bgSelected)
)

type viewState int

const (
	stateInput viewState = iota
	stateResults
	stateDialog
	stateHelp
)

func (s viewState) String() string {
	return []string{"INPUT", "RESULTS", "DIALOG", "HELP"}[s]
}

type searchQuery struct {
	Text      string `json:"text"`
	Highlight string `json:"highlight"`
	Limit     int    `json:"limit"`
}

type resultsMsg struct{ results *indexer.Results }
type errMsg struct{ err error }
type wsConnectedMsg struct{ conn *websocket.Conn }
type wsDisconnectedMsg struct{ err error }
type reconnectMsg struct{}

type tuiModel struct {
	textInput     textinput.Model
	viewport      viewport.Model
	state         viewState
	prevState     viewState
	cfg           *config.Config
	results       *indexer.Results
	selectedIdx   int
	limit         int
	width, height int
	ready         bool
	lineOffsets   []int
	totalLines    int
	conn          *websocket.Conn
	wsChan        chan tea.Msg
	wsDone        chan struct{}
	wsReady       bool
	dialogMsg     string
	dialogConfirm func() tea.Cmd
	connError     error
}

func initialModel(cfg *config.Config) *tuiModel {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 50
	return &tuiModel{
		textInput:   ti,
		state:       stateInput,
		prevState:   stateInput,
		cfg:         cfg,
		selectedIdx: -1,
		limit:       10,
		wsChan:      make(chan tea.Msg, 10),
		wsDone:      make(chan struct{}),
	}
}

func (m *tuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.connectWebSocket(), m.listenToWebSocket())
}

func (m *tuiModel) listenToWebSocket() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-m.wsChan:
			return msg
		case <-m.wsDone:
			return nil
		}
	}
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		// Exact fixed heights:
		// Input(3) + ViewportBorders(2) + Status(1) = 6
		fixedH := 6
		if m.height >= 15 {
			// Banner text(3) = 3 extra lines
			fixedH += 3
		}

		vpH := max(0, m.height-fixedH)
		vpW := max(1, m.width-4)
		m.textInput.Width = max(1, m.width-4)

		if !m.ready {
			m.viewport = viewport.New(vpW, vpH)
			m.viewport.SetContent("")
			m.ready = true
		} else {
			m.viewport.Width, m.viewport.Height = vpW, vpH
			m.viewport.SetContent(m.renderResults())
			m.scrollToSelected()
		}
		return m, tea.ClearScreen
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.state {
		case stateDialog:
			return m.handleDialogKeys(msg)
		case stateInput:
			return m.handleInputKeys(msg)
		case stateResults:
			return m.handleResultsKeys(msg)
		case stateHelp:
			m.state = m.prevState
			var cmd tea.Cmd
			if m.state == stateInput {
				cmd = m.textInput.Focus()
			}
			return m, cmd
		}
	case resultsMsg:
		m.results = msg.results
		if m.selectedIdx >= m.getTotalResults() {
			m.selectedIdx = m.getTotalResults() - 1
		}
		if m.selectedIdx < 0 && m.getTotalResults() > 0 {
			m.selectedIdx = 0
		}
		m.viewport.SetContent(m.renderResults())
		m.scrollToSelected()
		return m, m.listenToWebSocket()
	case wsConnectedMsg:
		if msg.conn != nil {
			m.conn = msg.conn
			m.wsReady = true
		}
		return m, m.listenToWebSocket()
	case wsDisconnectedMsg:
		m.wsReady = false
		if msg.err != nil {
			m.connError = msg.err
		}
		return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return reconnectMsg{} })
	case reconnectMsg:
		return m, m.connectWebSocket()
	case errMsg:
		return m, m.listenToWebSocket()
	}
	return m, nil
}

func (m *tuiModel) handleInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.cfg.Hotkeys.TUI[msg.String()]
	if msg.Type == tea.KeyRunes {
		action = ""
	}
	switch action {
	case "quit":
		return m, tea.Quit
	case "toggle_help":
		m.prevState, m.state = m.state, stateHelp
		m.textInput.Blur()
		return m, nil
	case "toggle_focus":
		if m.getTotalResults() > 0 {
			m.state = stateResults
			m.textInput.Blur()
			if m.selectedIdx < 0 {
				m.selectedIdx = 0
			}
			m.viewport.SetContent(m.renderResults())
			m.scrollToSelected()
		}
		return m, nil
	}
	var cmd tea.Cmd
	oldVal := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	if m.textInput.Value() != oldVal {
		m.limit = 10
		return m, tea.Batch(cmd, m.search())
	}
	return m, cmd
}

func (m *tuiModel) handleResultsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.cfg.Hotkeys.TUI[msg.String()]
	switch action {
	case "quit":
		return m, tea.Quit
	case "toggle_help":
		m.prevState, m.state = m.state, stateHelp
		return m, nil
	case "toggle_focus":
		m.state = stateInput
		m.textInput.Focus()
		m.viewport.SetContent(m.renderResults())
		return m, textinput.Blink
	case "scroll_up":
		if m.selectedIdx > 0 {
			m.selectedIdx--
			m.viewport.SetContent(m.renderResults())
			m.scrollToSelected()
		}
		return m, nil
	case "scroll_down":
		if m.selectedIdx < m.getTotalResults()-1 {
			m.selectedIdx++
			m.viewport.SetContent(m.renderResults())
			m.scrollToSelected()
		}
		return m, nil
	case "open_result":
		if m.selectedIdx == m.limit {
			m.limit += 10
			m.viewport.SetContent(m.renderResults())
			m.scrollToSelected()
			return m, m.search()
		} else if u := m.getSelectedURL(); u != "" {
			browser.OpenURL(u)
		}
		return m, nil
	case "delete_result":
		if u := m.getSelectedURL(); u != "" {
			m.state = stateDialog
			m.dialogMsg = "Delete this result? (y/n)"
			u := u
			m.dialogConfirm = func() tea.Cmd {
				return m.deleteURL(u)
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *tuiModel) handleDialogKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "y":
		if m.dialogConfirm != nil {
			cmd = m.dialogConfirm()
		}
		m.state = stateResults
		m.dialogConfirm = nil
		m.viewport.SetContent(m.renderResults())
	case "n", "esc":
		m.state = stateResults
		m.dialogConfirm = nil
	}
	return m, cmd
}

func (m *tuiModel) View() string {
	if !m.ready {
		return "Loading..."
	}
	if m.width < 20 || m.height < 10 {
		return "Terminal too small"
	}

	var sections []string
	if m.height >= 15 {
		bannerText := bannerStyle.Width(m.width - 1).Render(strings.TrimSpace(Banner))
		sections = append(sections, bannerText)
	}

	is := blurStyle
	if m.state == stateInput {
		is = focusStyle
	}
	sections = append(sections, is.Width(max(1, m.width-2)).Render(m.textInput.View()))

	vp := m.viewport.View()

	vpLines := strings.Split(vp, "\n")
	if len(vpLines) > m.viewport.Height {
		vpLines = vpLines[:m.viewport.Height]
	}
	for len(vpLines) < m.viewport.Height {
		vpLines = append(vpLines, "")
	}
	vp = strings.Join(vpLines, "\n")

	if m.totalLines > m.viewport.Height && m.viewport.Height > 0 {
		vp = lipgloss.JoinHorizontal(lipgloss.Top, vp, " ", m.renderScrollbar())
	}

	vs := blurStyle
	if m.state == stateResults {
		vs = focusStyle
	}
	vpWrapped := vs.Width(max(1, m.width-2)).Render(vp)

	sections = append(sections, vpWrapped, m.renderStatusBar())
	result := strings.Join(sections, "\n")

	if m.state == stateHelp {
		help := helpStyle.Render(generateHelpText(m.cfg))
		return lipgloss.Place(m.width-1, m.height, lipgloss.Center, lipgloss.Center, help)
	}
	if m.state == stateDialog {
		dialog := dialogStyle.Render(m.dialogMsg + "\n\n[y/n]")
		return lipgloss.Place(m.width-1, m.height, lipgloss.Center, lipgloss.Center, dialog)
	}

	return result
}

func generateHelpText(cfg *config.Config) string {
	bindings := make(map[string][]string)
	for k, v := range cfg.Hotkeys.TUI {
		bindings[v] = append(bindings[v], k)
	}
	fmtAct := func(action, label string) string {
		keys := bindings[action]
		if len(keys) == 0 {
			return ""
		}
		return fmt.Sprintf("  %-20s %s", strings.Join(keys, ", "), label)
	}
	lines := []string{"Configured Shortcuts:\n", "General:"}
	for _, a := range []struct{ act, lbl string }{
		{"quit", "Quit application"}, {"toggle_help", "Toggle this help"},
	} {
		if s := fmtAct(a.act, a.lbl); s != "" {
			lines = append(lines, s)
		}
	}
	lines = append(lines, "\nInput Mode:")
	if s := fmtAct("toggle_focus", "Go to results list"); s != "" {
		lines = append(lines, s)
	}
	lines = append(lines, "\nResults Mode:")
	for _, a := range []struct{ act, lbl string }{
		{"toggle_focus", "Go back to input"}, {"scroll_up", "Navigate up"},
		{"scroll_down", "Navigate down"}, {"open_result", "Open selected item"},
		{"delete_result", "Delete selected item"},
	} {
		if s := fmtAct(a.act, a.lbl); s != "" {
			lines = append(lines, s)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *tuiModel) renderStatusBar() string {
	cs := discStyle.Render("● disconnected")
	if m.wsReady {
		cs = connStyle.Render("● connected")
	}
	mode := modeStyles[m.state].Render(fmt.Sprintf(" [%s] ", strings.ToUpper(m.state.String())))
	count := 0
	if m.results != nil {
		count = int(m.results.Total)
	}
	left := " " + cs + mode + "  " + fmt.Sprintf("%d results", count)
	if m.connError != nil {
		left += " - " + discStyle.Render(m.connError.Error())
	}
	right := "Press ? for help "

	targetW := max(1, m.width-1)
	pad := max(0, targetW-lipgloss.Width(left)-lipgloss.Width(right))
	sb := left + strings.Repeat(" ", pad) + right

	if lipgloss.Width(sb) > targetW {
		return statusStyle.MaxWidth(targetW).MaxHeight(1).Render(sb)
	}
	return statusStyle.Render(sb)
}

func (m *tuiModel) renderResults() string {
	if m.results == nil || (len(m.results.Documents) == 0 && len(m.results.History) == 0) {
		m.lineOffsets, m.totalLines = nil, 0
		if m.textInput.Value() != "" {
			return grayStyle.Render("No results found")
		}
		return grayStyle.Render("Type to search...")
	}
	var items []string
	var lineOffsets []int
	currentLine, currentIdx := 0, 0

	w := max(1, m.viewport.Width-2)
	style := lipgloss.NewStyle().MaxWidth(w)
	for _, h := range m.results.History {
		if currentIdx >= m.limit {
			break
		}
		lineOffsets = append(lineOffsets, currentLine)
		item := style.Render(m.renderHistoryItem(h, currentIdx == m.selectedIdx))
		items = append(items, item)
		currentLine += lipgloss.Height(item)
		currentIdx++
	}
	for _, d := range m.results.Documents {
		if currentIdx >= m.limit {
			break
		}
		lineOffsets = append(lineOffsets, currentLine)
		item := style.Render(m.renderDocument(d, currentIdx == m.selectedIdx))
		items = append(items, item)
		currentLine += lipgloss.Height(item)
		currentIdx++
	}
	totalItems := len(m.results.History) + len(m.results.Documents)
	if totalItems > m.limit {
		lineOffsets = append(lineOffsets, currentLine)
		rem := max(0, int(m.results.Total)+len(m.results.History)-m.limit)
		var content string
		if currentIdx == m.selectedIdx {
			content = loadMoreSelectedStyle.Render(fmt.Sprintf("[ ▼ Load 10 more results (%d remaining in index) ]", rem))
		} else {
			content = loadMoreStyle.Render(fmt.Sprintf("[ ▼ Load 10 more results (%d remaining in index) ]", rem))
		}
		var item string
		if currentIdx == m.selectedIdx {
			item = style.Render(selectedItemStyle.Render(content))
		} else {
			item = style.Render(itemStyle.Render(content))
		}
		items = append(items, item)
		currentLine += lipgloss.Height(item)
	}
	m.lineOffsets, m.totalLines = lineOffsets, currentLine
	return strings.Join(items, "\n")
}

func (m *tuiModel) renderHistoryItem(h *model.URLCount, sel bool) string {
	ts := titleStyle
	if sel {
		ts = selTitle
	}
	content := histStyle.Render("[History] ") + ts.Render(strings.Join(strings.Fields(h.Title), " ")) + "\n" + urlStyle.Render(h.URL)

	if sel {
		return selectedItemStyle.Render(content)
	}
	return itemStyle.Render(content)
}

func (m *tuiModel) renderDocument(d *indexer.Document, sel bool) string {
	ts := titleStyle
	if sel {
		ts = selTitle
	}
	var sb strings.Builder
	sb.WriteString(ts.Render(strings.Join(strings.Fields(d.Title), " ")))
	sb.WriteString("\n")
	sb.WriteString(urlStyle.Render(d.URL))
	if d.Text != "" {
		sb.WriteString("\n")
		sb.WriteString(secTextStyle.Render("└ "))
		sb.WriteString(secTextStyle.Render(strings.Join(strings.Fields(d.Text), " ")))
	}

	if sel {
		return selectedItemStyle.Render(sb.String())
	}
	return itemStyle.Render(sb.String())
}

func (m *tuiModel) renderScrollbar() string {
	maxScroll := m.totalLines - m.viewport.Height
	pct := 0.0
	if maxScroll > 0 {
		pct = float64(m.viewport.YOffset) / float64(maxScroll)
	}
	pct = max(0, min(1, pct))
	thumbPos := int(pct * float64(m.viewport.Height-1))

	thumbChar := thumbStyle.Render("█")
	trackChar := trackStyle.Render("│")

	var sb strings.Builder
	for i := 0; i < m.viewport.Height; i++ {
		sb.WriteString(map[bool]string{true: thumbChar, false: trackChar}[i == thumbPos])
		if i < m.viewport.Height-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (m *tuiModel) scrollToSelected() {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.lineOffsets) {
		return
	}
	target := m.lineOffsets[m.selectedIdx]
	vpH := m.viewport.Height
	curY := m.viewport.YOffset
	if target < curY {
		m.viewport.SetYOffset(target)
	}
	if target >= curY+vpH {
		m.viewport.SetYOffset(target - vpH + 3)
	}
}

func (m *tuiModel) getTotalResults() int {
	if m.results == nil {
		return 0
	}
	c := len(m.results.History) + len(m.results.Documents)
	if c > m.limit {
		return m.limit + 1
	}
	return c
}

func (m *tuiModel) getSelectedURL() string {
	if m.results == nil || m.selectedIdx < 0 || m.selectedIdx == m.limit {
		return ""
	}
	if m.selectedIdx < len(m.results.History) {
		return m.results.History[m.selectedIdx].URL
	}
	docIdx := m.selectedIdx - len(m.results.History)
	if docIdx < len(m.results.Documents) {
		return m.results.Documents[docIdx].URL
	}
	return ""
}

func (m *tuiModel) connectWebSocket() tea.Cmd {
	return func() tea.Msg {
		wsURL := m.cfg.WebSocketURL()
		header := http.Header{}
		header.Set("Origin", m.cfg.BaseURL(""))
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			return wsDisconnectedMsg{err: err}
		}
		wsDone := m.wsDone
		wsChan := m.wsChan
		go func() {
			defer conn.Close()
			for {
				select {
				case <-wsDone:
					return
				default:
					_, data, err := conn.ReadMessage()
					if err != nil {
						select {
						case wsChan <- wsDisconnectedMsg{err: err}:
						case <-wsDone:
						}
						return
					}
					var res *indexer.Results
					if err := json.Unmarshal(data, &res); err != nil {
						continue
					}
					if len(res.Documents) == 0 && len(res.History) == 0 {
						res = &indexer.Results{}
					}
					select {
					case wsChan <- resultsMsg{results: res}:
					case <-wsDone:
						return
					}
				}
			}
		}()
		return wsConnectedMsg{conn: conn}
	}
}

func (m *tuiModel) search() tea.Cmd {
	return func() tea.Msg {
		if !m.wsReady || m.conn == nil {
			return nil
		}
		qt := strings.TrimSpace(m.textInput.Value())
		if qt == "" {
			return resultsMsg{results: &indexer.Results{}}
		}
		b, err := json.Marshal(searchQuery{Text: qt, Highlight: "tui", Limit: m.limit + 1})
		if err != nil {
			return nil
		}
		m.conn.WriteMessage(websocket.TextMessage, b)
		return nil
	}
}

func (m *tuiModel) deleteURL(u string) tea.Cmd {
	return func() tea.Msg {
		formData := url.Values{"url": {u}}
		req, _ := http.NewRequest("POST", m.cfg.BaseURL("/delete"), strings.NewReader(formData.Encode()))
		req.Header.Set("Origin", "hister://")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		http.DefaultClient.Do(req)
		return m.search()()
	}
}

func (m *tuiModel) close() {
	close(m.wsDone)
}

func SearchTUI(cfg *config.Config) error {
	m := initialModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if fm, ok := finalModel.(*tuiModel); ok {
		fm.close()
	}
	return nil
}
