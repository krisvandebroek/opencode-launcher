package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	p := tea.NewProgram(m, tea.WithAltScreen())
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
	focusModels
	focusSessions
)

type sessionsLoadedMsg struct {
	projectID string
	sessions  []opencodestorage.Session
	err       error
}

type model struct {
	storageRoot string

	projectsAll []opencodestorage.Project
	sessionsByProject map[string][]opencodestorage.Session
	loadingSessionsFor string

	models []config.Model
	defaultModelIdx int

	plan *LaunchPlan

	focus focus

	width  int
	height int

	projFilter textinput.Model
	sesFilter  textinput.Model
	lastProjQuery string
	lastSesQuery  string

	projList list.Model
	modelList list.Model
	sesList list.Model

	styles styles
}

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
	projFilter.Prompt = "> "
	projFilter.CharLimit = 100
	projFilter.Focus()

	sesFilter := textinput.New()
	sesFilter.Placeholder = "type to filter"
	sesFilter.Prompt = "> "
	sesFilter.CharLimit = 100

	projList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	projList.SetShowStatusBar(false)
	projList.SetFilteringEnabled(false)
	projList.SetShowHelp(false)
	projList.DisableQuitKeybindings()

	modelList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	modelList.SetShowStatusBar(false)
	modelList.SetFilteringEnabled(false)
	modelList.SetShowHelp(false)
	modelList.DisableQuitKeybindings()

	sesList := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	sesList.SetShowStatusBar(false)
	sesList.SetFilteringEnabled(false)
	sesList.SetShowHelp(false)
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
		storageRoot:        in.StorageRoot,
		projectsAll:        in.Projects,
		sessionsByProject:  map[string][]opencodestorage.Session{},
		models:             in.Models,
		defaultModelIdx:    defaultIdx,
		focus:              focusProjects,
		projFilter:         projFilter,
		sesFilter:          sesFilter,
		projList:           projList,
		modelList:          modelList,
		sesList:            sesList,
		styles:             st,
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
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.plan = nil
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % 3
			m.updateFocus()
			return m, nil
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
			m.updateFocus()
			return m, nil
		case "enter":
			if m.setPlanFromSelection() {
				return m, tea.Quit
			}
			return m, nil
		}
	case sessionsLoadedMsg:
		if msg.projectID == m.loadingSessionsFor {
			m.loadingSessionsFor = ""
		}
		if msg.err == nil {
			m.sessionsByProject[msg.projectID] = msg.sessions
			m.applySessionFilter(true)
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
	case focusModels:
		m.modelList, cmd = m.modelList.Update(msg)
		return m, cmd
	case focusSessions:
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
		return m, tea.Batch(cmd, cmd2)
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
		"tab: switch  enter: launch  q: quit",
	)

	projTitle := m.title("Projects", m.focus == focusProjects)
	modelTitle := m.title("Models", m.focus == focusModels)
	sesTitle := m.title("Sessions", m.focus == focusSessions)

	projPanel := m.panel(m.focus == focusProjects, projTitle+"\n"+m.projFilter.View()+"\n"+m.projList.View())
	modelPanel := m.panel(m.focus == focusModels, modelTitle+"\n"+m.modelList.View())
	loading := ""
	if m.loadingSessionsFor != "" {
		loading = m.styles.muted.Render("loading...") + "\n"
	}
	sesPanel := m.panel(m.focus == focusSessions, sesTitle+"\n"+m.sesFilter.View()+"\n"+loading+m.sesList.View())

	content := m.layout(projPanel, modelPanel, sesPanel)

	footer := m.selectionSummary()
	return strings.TrimRight(header+"\n\n"+content+"\n\n"+footer, "\n")
}

func (m model) selectionSummary() string {
	p := m.selectedProject()
	if p == nil {
		return m.styles.muted.Render("Pick a project. Default launch uses the selected model + New session.")
	}
	model := m.selectedModel()
	sesLabel := m.selectedSessionLabel()
	return m.styles.muted.Render(fmt.Sprintf("Project: %s  Model: %s  Session: %s", filepath.Base(p.Worktree), model.Name, sesLabel))
}

func (m model) selectedSessionLabel() string {
	it := m.sesList.SelectedItem()
	if it == nil {
		return "New session"
	}
	if _, ok := it.(sessionNewItem); ok {
		return "New session"
	}
	si, ok := it.(sessionItem)
	if !ok {
		return "New session"
	}
	return si.Session.Title
}

func (m *model) resize() {
	// Reserve a bit of space for header/footer.
	height := m.height - 6
	if height < 8 {
		height = 8
	}

	colW := (m.width - 6) / 3
	if colW < 20 {
		colW = 20
	}

	m.projList.SetSize(colW-2, height-4)
	m.modelList.SetSize(colW-2, height-2)
	m.sesList.SetSize(colW-2, height-5)
}

func (m *model) updateFocus() {
	if m.focus == focusProjects {
		m.projFilter.Focus()
		m.sesFilter.Blur()
	} else if m.focus == focusSessions {
		m.sesFilter.Focus()
		m.projFilter.Blur()
	} else {
		m.projFilter.Blur()
		m.sesFilter.Blur()
	}
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

func (m model) layout(a, b, c string) string {
	// If the terminal is too narrow, stack panels.
	if m.width < 110 {
		return lipgloss.JoinVertical(lipgloss.Top, a, b, c)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, a, b, c)
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
	if m.loadingSessionsFor == p.ID {
		return nil
	}

	projectID := p.ID
	storageRoot := m.storageRoot
	m.loadingSessionsFor = projectID
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
func (p projectItem) Description() string { return p.Worktree }
func (p projectItem) FilterValue() string { return p.Title() + " " + p.Description() }

type modelItem struct{ config.Model }

func (mi modelItem) Title() string       { return mi.Name }
func (mi modelItem) Description() string { return mi.Model.Model }
func (mi modelItem) FilterValue() string { return mi.Name + " " + mi.Model.Model }

type sessionNewItem struct{}

func (s sessionNewItem) Title() string       { return "New session" }
func (s sessionNewItem) Description() string { return "start fresh" }
func (s sessionNewItem) FilterValue() string { return "new" }

type sessionItem struct{ opencodestorage.Session }

func (s sessionItem) Title() string       { return s.Session.Title }
func (s sessionItem) Description() string { return s.Session.ID }
func (s sessionItem) FilterValue() string { return s.Title() + " " + s.Description() }
