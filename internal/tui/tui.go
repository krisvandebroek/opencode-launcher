package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"oc/internal/config"
	"oc/internal/opencodestorage"
)

type Input struct {
	StorageRoot  string
	Projects     []opencodestorage.Project
	Models       []config.Model
	DefaultModel config.Model
}

type LaunchPlan struct {
	ProjectDir string
	Model      config.Model
	SessionID  string // empty means new session
}

func Run(in Input) (*LaunchPlan, error) {
	m := newModel(in)
	// Enable mouse reporting so the terminal doesn't scroll the alternate screen.
	// We ignore all mouse events in Update.
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	res, err := p.Run()
	if err != nil {
		return nil, err
	}
	final := res.(model)
	return final.plan, nil
}

type focus int

const (
	focusProjects focus = iota
	focusSessions
	focusModels
)

type sessionsLoadedMsg struct {
	projectID string
	sessions  []opencodestorage.Session
	err       error
}

type model struct {
	storageRoot string

	projectsAll       []opencodestorage.Project
	sessionsByProject map[string][]opencodestorage.Session
	loadingSessions   map[string]bool

	models          []config.Model
	defaultModelIdx int

	plan *LaunchPlan

	focus focus

	width  int
	height int

	panelHeight int
	colWProj    int
	colWSes     int
	colWModel   int

	projFilter    textinput.Model
	sesFilter     textinput.Model
	lastProjQuery string
	lastSesQuery  string

	projList  list.Model
	modelList list.Model
	sesList   list.Model

	styles styles
}

const (
	colGapSpaces       = 1
	outerMarginLeft    = 1
	outerMarginRight   = 2
	defaultSafetySlack = 0
	ghosttySafetySlack = 7
	minColWProjects    = 28
	minColWSessions    = 34
	minColWModel       = 26
	maxColWModel       = 44
	maxColWSessionsCap = 70
	maxColWProjectsCap = 86
	maxProjectDescLen  = 96
)

type styles struct {
	titleActive lipgloss.Style
	titleIdle   lipgloss.Style
	panel       lipgloss.Style
	panelActive lipgloss.Style
	muted       lipgloss.Style
}

func newModel(in Input) model {
	st := styles{
		titleActive: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")),
		titleIdle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245")),
		panel:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1),
		panelActive: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("205")).Padding(0, 1),
		muted:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}

	projFilter := textinput.New()
	projFilter.Placeholder = "type to filter"
	projFilter.Prompt = ""
	projFilter.CharLimit = 100
	projFilter.Focus()

	sesFilter := textinput.New()
	sesFilter.Placeholder = "type to filter"
	sesFilter.Prompt = ""
	sesFilter.CharLimit = 100
	sesFilter.Focus()

	projList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	projList.SetShowStatusBar(false)
	projList.SetFilteringEnabled(false)
	projList.SetShowHelp(false)
	projList.SetShowTitle(false)
	projList.DisableQuitKeybindings()

	modelList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	modelList.SetShowStatusBar(false)
	modelList.SetFilteringEnabled(false)
	modelList.SetShowHelp(false)
	modelList.SetShowTitle(false)
	modelList.DisableQuitKeybindings()

	sesList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	sesList.SetShowStatusBar(false)
	sesList.SetFilteringEnabled(false)
	sesList.SetShowHelp(false)
	sesList.SetShowTitle(false)
	sesList.DisableQuitKeybindings()

	items := make([]list.Item, 0, len(in.Projects))
	for _, p := range in.Projects {
		items = append(items, projectItem{p})
	}
	projList.SetItems(items)

	modelItems := make([]list.Item, 0, len(in.Models))
	defaultIdx := 0
	for i, m := range in.Models {
		modelItems = append(modelItems, modelItem{m})
		if m.Model == in.DefaultModel.Model && m.Name == in.DefaultModel.Name {
			defaultIdx = i
		}
	}
	modelList.SetItems(modelItems)
	modelList.Select(defaultIdx)

	// Sessions start with only the "New session" choice.
	sesList.SetItems([]list.Item{sessionNewItem{}})
	sesList.Select(0)

	return model{
		storageRoot:       in.StorageRoot,
		projectsAll:       in.Projects,
		sessionsByProject: map[string][]opencodestorage.Session{},
		loadingSessions:   map[string]bool{},
		models:            in.Models,
		defaultModelIdx:   defaultIdx,
		focus:             focusProjects,
		projFilter:        projFilter,
		sesFilter:         sesFilter,
		projList:          projList,
		modelList:         modelList,
		sesList:           sesList,
		styles:            st,
	}
}

func (m model) Init() tea.Cmd {
	return m.loadSessionsForSelectedProjectCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case tea.MouseMsg:
		// Mouse events are intentionally ignored (prevents scroll/click from
		// messing up the UX; keyboard-first only).
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.plan = nil
			return m, tea.Quit
		case "tab":
			m.focus = m.nextFocus(1)
			m.ensureValidFocus()
			m.updateFocus()
			return m, nil
		case "shift+tab":
			m.focus = m.nextFocus(-1)
			m.ensureValidFocus()
			m.updateFocus()
			return m, nil
		case "enter":
			if m.setPlanFromSelection() {
				return m, tea.Quit
			}
			return m, nil
		}
	case sessionsLoadedMsg:
		delete(m.loadingSessions, msg.projectID)
		if msg.err == nil {
			m.sessionsByProject[msg.projectID] = msg.sessions
			if p := m.selectedProject(); p != nil && p.ID == msg.projectID {
				m.applySessionFilter(true)
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	var cmd2 tea.Cmd
	var cmd3 tea.Cmd

	switch m.focus {
	case focusProjects:
		oldID := ""
		if p := m.selectedProject(); p != nil {
			oldID = p.ID
		}

		// Route navigation keys to the list; everything else to the filter.
		if km, ok := msg.(tea.KeyMsg); ok && isNavKey(km) {
			m.projList, cmd2 = m.projList.Update(msg)
		} else {
			before := m.projFilter.Value()
			m.projFilter, cmd = m.projFilter.Update(msg)
			after := m.projFilter.Value()
			if after != before {
				m.applyProjectFilter(true)
			}
			m.projList, cmd2 = m.projList.Update(msg)
		}

		newID := ""
		if p := m.selectedProject(); p != nil {
			newID = p.ID
		}
		if newID != oldID {
			m.sesFilter.SetValue("")
			m.applySessionFilter(true)
		}
		cmd3 = m.loadSessionsForSelectedProjectCmd()
		return m, tea.Batch(cmd, cmd2, cmd3)
	case focusSessions:
		oldSessionID := m.selectedSessionID()
		if km, ok := msg.(tea.KeyMsg); ok && isNavKey(km) {
			m.sesList, cmd2 = m.sesList.Update(msg)
		} else {
			before := m.sesFilter.Value()
			m.sesFilter, cmd = m.sesFilter.Update(msg)
			after := m.sesFilter.Value()
			if after != before {
				m.applySessionFilter(true)
			}
			m.sesList, cmd2 = m.sesList.Update(msg)
		}

		if m.selectedSessionID() != oldSessionID {
			m.ensureValidFocus()
			m.updateFocus()
		}
		return m, tea.Batch(cmd, cmd2)
	case focusModels:
		if m.modelLocked() {
			return m, nil
		}
		m.modelList, cmd = m.modelList.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func isNavKey(k tea.KeyMsg) bool {
	s := k.String()
	switch s {
	case "up", "down", "left", "right", "pgup", "pgdown", "home", "end":
		return true
	default:
		return false
	}
}

func (m model) View() string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
		"tab: next  shift+tab: prev  enter: launch  ctrl+c: quit  (type to filter)",
	)

	projTitle := m.title("Projects", m.focus == focusProjects)
	sesTitle := m.title("Sessions", m.focus == focusSessions)
	modelTitle := m.title("Model", m.focus == focusModels)
	if m.modelLocked() {
		modelTitle = m.title("Model (locked)", false)
	}

	sesPanel := m.panelW(m.focus == focusSessions, m.colWSes, m.panelHeight, sesTitle+"\n"+m.filterLine(m.sesFilter.Value())+"\n"+m.sesList.View())

	modelBody := ""
	if m.modelLocked() {
		sel := m.selectedModel()
		modelBody = strings.TrimSpace(m.styles.muted.Render("Locked by session") + "\n" +
			m.styles.muted.Render("To change model, pick 'New session'.") + "\n\n" +
			m.styles.muted.Render(fmt.Sprintf("Selected for new sessions: %s", sel.Name)))
	} else {
		modelBody = m.modelList.View()
	}
	modelPanel := m.panelW(m.focus == focusModels && !m.modelLocked(), m.colWModel, m.panelHeight, modelTitle+"\n"+modelBody)

	projPanel := m.panelW(m.focus == focusProjects, m.colWProj, m.panelHeight, projTitle+"\n"+m.filterLine(m.projFilter.Value())+"\n"+m.projList.View())

	content := m.layout(projPanel, sesPanel, modelPanel)
	return strings.TrimRight(m.inset(header+"\n\n"+content), "\n")
}

func (m model) inset(s string) string {
	innerW := m.width - outerMarginLeft - outerMarginRight
	if innerW <= 0 {
		return s
	}

	lines := strings.Split(s, "\n")
	for i := range lines {
		// Bubble Tea's renderer diffs lines; if a new frame produces a shorter
		// line than the previous frame, the leftover characters can remain on
		// screen unless we fully clear the row.
		trimmed := ansi.Truncate(lines[i], innerW, "")
		padInner := innerW - ansi.StringWidth(trimmed)
		if padInner < 0 {
			padInner = 0
		}
		lines[i] = strings.Repeat(" ", outerMarginLeft) + trimmed + strings.Repeat(" ", padInner) + strings.Repeat(" ", outerMarginRight)
	}
	return strings.Join(lines, "\n")
}

func (m model) safetySlack() int {
	if v := strings.TrimSpace(os.Getenv("OC_TUI_SAFETY_SLACK")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n < 0 {
				return 0
			}
			return n
		}
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM_PROGRAM")), "ghostty") {
		return ghosttySafetySlack
	}
	return defaultSafetySlack
}

func (m model) filterLine(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return m.styles.muted.Render("type to filter")
	}
	return m.styles.muted.Render("filter: " + v)
}

func (m *model) resize() {
	// Reserve a bit of space for the header.
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	m.panelHeight = height

	// When stacked (narrow terminal), let panels fill the width.
	if m.width < 110 {
		m.colWProj, m.colWSes, m.colWModel = 0, 0, 0
		innerW := maxInt(20, m.width-4)
		m.projList.SetSize(innerW, maxInt(3, height-2))
		m.sesList.SetSize(innerW, maxInt(3, height-2))
		m.modelList.SetSize(innerW, maxInt(3, height-1))
		return
	}

	// Available width for the three panels plus the gaps between them.
	// Some terminals (notably Ghostty) may crop the last border; we support a
	// configurable safety slack via OC_TUI_SAFETY_SLACK.
	available := m.width - 2*colGapSpaces - outerMarginLeft - outerMarginRight - m.safetySlack()
	if available < 0 {
		available = 0
	}

	// Width policy:
	// - Model should stay readable and stable, but must shrink when space is tight.
	// - Sessions can be long but shouldn't balloon and starve Model.
	// - Projects can take what's left, capped.
	maxModelAllowed := minInt(maxColWModel, available-minColWProjects-minColWSessions)
	if maxModelAllowed < minColWModel {
		maxModelAllowed = minColWModel
	}
	modelW := clampInt(maxModelLineLen(m.models)+8, minColWModel, maxModelAllowed)
	projW := clampInt(maxProjectLineLen(m.projList.Items())+8, minColWProjects, maxColWProjectsCap)
	remaining := available - modelW
	if remaining < 0 {
		remaining = 0
	}
	maxSesW := maxInt(minColWSessions, minInt(maxColWSessionsCap, remaining-minColWProjects))
	sesW := clampInt(int(float64(remaining)*0.55), minColWSessions, maxSesW)
	projW = clampInt(remaining-sesW, minColWProjects, maxColWProjectsCap)
	sesW = remaining - projW
	if sesW < minColWSessions {
		// Borrow from projects first.
		need := minColWSessions - sesW
		projW = clampInt(projW-need, minColWProjects, maxColWProjectsCap)
		sesW = remaining - projW
	}

	// Final safety clamp: if we can't satisfy mins, fall back to equal thirds.
	if projW+sesW+modelW > available || projW < minColWProjects || sesW < minColWSessions || modelW < minColWModel {
		colW := available / 3
		if colW < 20 {
			colW = 20
		}
		projW, sesW, modelW = colW, colW, colW
	}

	m.colWProj, m.colWSes, m.colWModel = projW, sesW, modelW

	// Approximate panel chrome: border(2) + padding(2) = 4.
	innerProjW := maxInt(10, projW-4)
	innerSesW := maxInt(10, sesW-4)
	innerModelW := maxInt(10, modelW-4)

	projListH := maxInt(3, height-2)
	sesListH := maxInt(3, height-2)
	modelListH := maxInt(3, height-1)

	m.projList.SetSize(innerProjW, projListH)
	m.sesList.SetSize(innerSesW, sesListH)
	m.modelList.SetSize(innerModelW, modelListH)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxModelLineLen(models []config.Model) int {
	maxLen := 0
	for _, m := range models {
		if l := len(m.Name); l > maxLen {
			maxLen = l
		}
		if l := len(m.Model); l > maxLen {
			maxLen = l
		}
	}
	return maxLen
}

func maxProjectLineLen(items []list.Item) int {
	maxLen := 0
	for _, it := range items {
		pi, ok := it.(projectItem)
		if !ok {
			continue
		}
		if l := len(pi.Title()); l > maxLen {
			maxLen = l
		}
		if l := len(pi.Description()); l > maxLen {
			maxLen = l
		}
	}
	return maxLen
}

func (m *model) updateFocus() {
	// Intentionally no visible cursor/focus for filter inputs.
	// Input is captured based on the focused column.
}

func (m model) modelLocked() bool {
	return m.selectedSessionID() != ""
}

func (m model) isLoadingSelectedProject() bool {
	p := m.selectedProject()
	if p == nil {
		return false
	}
	return m.loadingSessions[p.ID]
}

func (m *model) ensureValidFocus() {
	if m.modelLocked() && m.focus == focusModels {
		m.focus = focusSessions
	}
}

func (m model) nextFocus(delta int) focus {
	order := []focus{focusProjects, focusSessions, focusModels}
	allowed := make([]focus, 0, len(order))
	for _, f := range order {
		if f == focusModels && m.modelLocked() {
			continue
		}
		allowed = append(allowed, f)
	}
	if len(allowed) == 0 {
		return focusProjects
	}
	idx := 0
	for i := range allowed {
		if allowed[i] == m.focus {
			idx = i
			break
		}
	}
	if delta > 0 {
		return allowed[(idx+1)%len(allowed)]
	}
	return allowed[(idx+len(allowed)-1)%len(allowed)]
}

func (m model) title(text string, active bool) string {
	if active {
		return m.styles.titleActive.Render(text)
	}
	return m.styles.titleIdle.Render(text)
}

func (m model) panel(active bool, content string) string {
	if active {
		return m.styles.panelActive.Render(content)
	}
	return m.styles.panel.Render(content)
}

func (m model) panelW(active bool, w, h int, content string) string {
	st := m.styles.panel
	if active {
		st = m.styles.panelActive
	}
	if w > 0 {
		st = st.Width(w)
	}
	if h > 0 {
		st = st.Height(h)
	}
	return st.Render(content)
}

func (m model) layout(a, b, c string) string {
	// If the terminal is too narrow, stack panels.
	if m.width < 110 {
		return lipgloss.JoinVertical(lipgloss.Top, a, b, c)
	}
	gap := lipgloss.NewStyle().MarginRight(colGapSpaces)
	return lipgloss.JoinHorizontal(lipgloss.Top, gap.Render(a), gap.Render(b), c)
}

func (m *model) applyProjectFilter(resetSelection bool) {
	q := strings.ToLower(strings.TrimSpace(m.projFilter.Value()))
	if !resetSelection && q == m.lastProjQuery {
		return
	}
	m.lastProjQuery = q

	oldID := ""
	if !resetSelection {
		if p := m.selectedProject(); p != nil {
			oldID = p.ID
		}
	}

	items := make([]list.Item, 0, len(m.projectsAll))
	for _, p := range m.projectsAll {
		pi := projectItem{p}
		if q == "" {
			items = append(items, pi)
			continue
		}
		if strings.Contains(strings.ToLower(pi.Title()), q) || strings.Contains(strings.ToLower(pi.Description()), q) {
			items = append(items, pi)
		}
	}
	m.projList.SetItems(items)
	if len(items) == 0 {
		return
	}
	if resetSelection {
		m.projList.Select(0)
		return
	}
	if oldID == "" {
		return
	}
	for i := range items {
		pi, ok := items[i].(projectItem)
		if ok && pi.ID == oldID {
			m.projList.Select(i)
			return
		}
	}
}

func (m *model) applySessionFilter(resetSelection bool) {
	p := m.selectedProject()
	items := []list.Item{sessionNewItem{}}
	if p == nil {
		m.sesList.SetItems(items)
		m.sesList.Select(0)
		return
	}
	all := m.sessionsByProject[p.ID]
	q := strings.ToLower(strings.TrimSpace(m.sesFilter.Value()))
	if !resetSelection && q == m.lastSesQuery {
		return
	}
	m.lastSesQuery = q

	for _, s := range all {
		si := sessionItem{s}
		if q == "" {
			items = append(items, si)
			continue
		}
		if strings.Contains(strings.ToLower(si.Title()), q) || strings.Contains(strings.ToLower(si.Description()), q) {
			items = append(items, si)
		}
	}
	m.sesList.SetItems(items)
	if resetSelection {
		m.sesList.Select(0)
	}
}

func (m model) selectedProject() *opencodestorage.Project {
	it := m.projList.SelectedItem()
	if it == nil {
		return nil
	}
	p, ok := it.(projectItem)
	if !ok {
		return nil
	}
	return &p.Project
}

func (m model) selectedModel() config.Model {
	it := m.modelList.SelectedItem()
	if it == nil {
		return m.models[m.defaultModelIdx]
	}
	mi, ok := it.(modelItem)
	if !ok {
		return m.models[m.defaultModelIdx]
	}
	return mi.Model
}

func (m model) selectedSessionID() string {
	it := m.sesList.SelectedItem()
	if it == nil {
		return ""
	}
	sn, ok := it.(sessionNewItem)
	if ok {
		_ = sn
		return ""
	}
	si, ok := it.(sessionItem)
	if !ok {
		return ""
	}
	return si.Session.ID
}

func (m *model) loadSessionsForSelectedProjectCmd() tea.Cmd {
	p := m.selectedProject()
	if p == nil {
		return nil
	}
	if _, ok := m.sessionsByProject[p.ID]; ok {
		return nil
	}
	if m.loadingSessions[p.ID] {
		return nil
	}

	projectID := p.ID
	storageRoot := m.storageRoot
	m.loadingSessions[projectID] = true
	return func() tea.Msg {
		sessions, err := opencodestorage.LoadSessions(storageRoot, projectID)
		return sessionsLoadedMsg{projectID: projectID, sessions: sessions, err: err}
	}
}

func (m *model) setPlanFromSelection() bool {
	p := m.selectedProject()
	if p == nil {
		return false
	}
	m.plan = &LaunchPlan{
		ProjectDir: p.Worktree,
		Model:      m.selectedModel(),
		SessionID:  m.selectedSessionID(),
	}
	return true
}

type projectItem struct{ opencodestorage.Project }

func (p projectItem) Title() string       { return filepath.Base(p.Worktree) }
func (p projectItem) Description() string { return shortenPath(p.Worktree, maxProjectDescLen) }
func (p projectItem) FilterValue() string { return p.Title() + " " + p.Description() }

type modelItem struct{ config.Model }

func (mi modelItem) Title() string       { return mi.Name }
func (mi modelItem) Description() string { return mi.Model.Model }
func (mi modelItem) FilterValue() string { return mi.Name + " " + mi.Model.Model }

type sessionNewItem struct{}

func (s sessionNewItem) Title() string       { return "New session (choose model)" }
func (s sessionNewItem) Description() string { return "start fresh" }
func (s sessionNewItem) FilterValue() string { return "new" }

type sessionItem struct{ opencodestorage.Session }

func (s sessionItem) Title() string       { return s.Session.Title }
func (s sessionItem) Description() string { return formatUpdated(s.Session.Updated) }
func (s sessionItem) FilterValue() string { return s.Title() + " " + s.Description() }

func formatUpdated(ms int64) string {
	if ms <= 0 {
		return ""
	}
	t := time.UnixMilli(ms).Local()
	return t.Format("2006-01-02 15:04")
}

func shortenPath(p string, max int) string {
	if max <= 0 {
		return p
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		prefix := home + string(filepath.Separator)
		if strings.HasPrefix(p, prefix) {
			p = "~" + p[len(home):]
		}
	}
	if len(p) <= max {
		return p
	}
	if max < 10 {
		return p[:max]
	}
	// Keep both start and end; truncate in the middle.
	keepStart := (max - 3) / 2
	keepEnd := (max - 3) - keepStart
	return p[:keepStart] + "..." + p[len(p)-keepEnd:]
}
