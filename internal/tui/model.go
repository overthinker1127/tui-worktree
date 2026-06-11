package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type Config struct {
	Context    context.Context
	ThemeName  string
	Theme      theme.Styles
	ThemeNames []string
	Changes    []gitview.FileChange
	Diff       string
	Diffs      map[string]string
	Error      error
	LoadDiff   func(context.Context, gitview.FileChange) string
	Reload     func(context.Context) Snapshot
}

type Snapshot struct {
	Changes []gitview.FileChange
	Diffs   map[string]string
	Error   error
}

const (
	iconFile      = "󰈙"
	iconModified  = ""
	iconAdded     = ""
	iconDeleted   = ""
	iconRenamed   = "󰁕"
	iconUntracked = ""
	iconBinary    = ""
	iconRefresh   = ""
	iconTheme     = ""
	iconHelp      = "󰋖"
	iconQuit      = "󰩈"
)

type Model struct {
	styles       theme.Styles
	context      context.Context
	themeName    string
	themeNames   []string
	themeCursor  int
	changes      []gitview.FileChange
	diffs        map[string]string
	selected     int
	revision     int
	width        int
	height       int
	err          error
	status       string
	showHelp     bool
	pickingTheme bool
	loadDiff     func(context.Context, gitview.FileChange) string
	reload       func(context.Context) Snapshot
	viewport     viewport.Model
}

func NewModel(cfg Config) Model {
	vp := viewport.New()
	vp.SoftWrap = false
	m := Model{
		styles:     cfg.Theme,
		context:    cfg.Context,
		themeName:  cfg.ThemeName,
		themeNames: cfg.ThemeNames,
		changes:    cfg.Changes,
		diffs:      cfg.Diffs,
		err:        cfg.Error,
		loadDiff:   cfg.LoadDiff,
		reload:     cfg.Reload,
		viewport:   vp,
		width:      100,
		height:     30,
	}
	if m.themeName == "" {
		m.themeName = "tokyonight"
	}
	if m.context == nil {
		m.context = context.Background()
	}
	if len(m.themeNames) == 0 {
		m.themeNames = theme.Names()
	}
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	if cfg.Diff != "" && len(cfg.Changes) > 0 {
		m.diffs[cfg.Changes[0].Path] = cfg.Diff
	}
	m.refreshDiff()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case reloadMsg:
		m.applySnapshot(msg.snapshot)
		return m, m.ensureSelectedDiffCmd()
	case diffLoadedMsg:
		if msg.revision != m.revision {
			return m, nil
		}
		if msg.path != "" {
			m.diffs[msg.path] = msg.diff
			if selected := m.Selected(); selected.Path == msg.path {
				m.refreshDiff()
			}
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
	case tea.MouseClickMsg:
		if m.handleMouse(msg.Mouse()) {
			return m, m.ensureSelectedDiffCmd()
		}
	case tea.KeyPressMsg:
		if m.showHelp {
			switch msg.String() {
			case "?", "esc":
				m.showHelp = false
				return m, nil
			case "ctrl+c", "q":
				return m, tea.Quit
			}
			return m, nil
		}
		if m.pickingTheme {
			return m.handleThemeKey(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "?":
			m.showHelp = true
		case "t":
			m.openThemePicker()
		case "r":
			m.status = "Refreshing..."
			return m, m.reloadCmd()
		case "j", "down":
			m.moveSelection(1)
			return m, m.ensureSelectedDiffCmd()
		case "k", "up":
			m.moveSelection(-1)
			return m, m.ensureSelectedDiffCmd()
		case "g", "home":
			m.selected = 0
			m.refreshDiff()
			return m, m.ensureSelectedDiffCmd()
		case "G", "end":
			if len(m.changes) > 0 {
				m.selected = len(m.changes) - 1
				m.refreshDiff()
				return m, m.ensureSelectedDiffCmd()
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	leftWidth, rightWidth := m.layoutWidths()
	contentHeight := max(4, m.height-4)

	files := m.renderFiles(leftWidth, contentHeight)
	diff := m.renderDiff(rightWidth, contentHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, files, diff)

	header := m.styles.Title.Render("Files changed")
	footerText := fmt.Sprintf("%s theme:%s  j/k move  %s r refresh  %s t themes  %s ? help  %s q quit", iconTheme, m.themeName, iconRefresh, iconTheme, iconHelp, iconQuit)
	if m.status != "" {
		footerText = m.status + "  " + footerText
	}
	footer := m.styles.Footer.Render(footerText)
	if m.err != nil {
		footer = m.styles.Error.Render(m.err.Error())
	}
	if m.pickingTheme {
		body = m.renderThemePicker()
	} else if m.showHelp {
		body = m.renderHelp()
	}

	view := tea.NewView(m.styles.App.Width(m.width).Height(m.height).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, body, footer),
	))
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (m Model) Selected() gitview.FileChange {
	if len(m.changes) == 0 || m.selected < 0 || m.selected >= len(m.changes) {
		return gitview.FileChange{}
	}
	return m.changes[m.selected]
}

func (m *Model) moveSelection(delta int) {
	if len(m.changes) == 0 {
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.changes) {
		m.selected = len(m.changes) - 1
	}
	m.refreshDiff()
}

func (m *Model) openThemePicker() {
	m.pickingTheme = true
	m.themeCursor = indexOf(m.themeNames, m.themeName)
	if m.themeCursor < 0 {
		m.themeCursor = 0
	}
}

func (m Model) handleThemeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "t":
		m.pickingTheme = false
	case "j", "down":
		if m.themeCursor < len(m.themeNames)-1 {
			m.themeCursor++
		}
	case "k", "up":
		if m.themeCursor > 0 {
			m.themeCursor--
		}
	case "enter":
		m.applyThemeCursor()
		m.pickingTheme = false
	}
	return m, nil
}

func (m *Model) applyThemeCursor() {
	if m.themeCursor < 0 || m.themeCursor >= len(m.themeNames) {
		return
	}
	name := m.themeNames[m.themeCursor]
	preset, err := theme.Preset(name)
	if err != nil {
		m.status = err.Error()
		return
	}
	m.themeName = name
	m.styles = theme.NewStyles(preset)
	m.status = fmt.Sprintf("Theme changed to %s", name)
	m.refreshDiff()
}

func (m *Model) handleMouse(mouse tea.Mouse) bool {
	if m.pickingTheme {
		index := mouse.Y - 3
		if index >= 0 && index < len(m.themeNames) {
			m.themeCursor = index
			m.applyThemeCursor()
			m.pickingTheme = false
		}
		return false
	}
	leftWidth, _ := m.layoutWidths()
	if mouse.X >= leftWidth || mouse.Y < 3 {
		return false
	}
	index := m.listOffset(m.height-4) + mouse.Y - 3
	if index >= 0 && index < len(m.changes) {
		m.selected = index
		m.refreshDiff()
		return true
	}
	return false
}

func (m Model) reloadCmd() tea.Cmd {
	reload := m.reload
	if reload == nil {
		return func() tea.Msg {
			return reloadMsg{snapshot: Snapshot{Changes: m.changes, Diffs: m.diffs, Error: fmt.Errorf("no reload source configured")}}
		}
	}
	return func() tea.Msg {
		return reloadMsg{snapshot: reload(m.context)}
	}
}

func (m Model) ensureSelectedDiffCmd() tea.Cmd {
	if m.loadDiff == nil {
		return nil
	}
	selected := m.Selected()
	if selected.Path == "" {
		return nil
	}
	if _, ok := m.diffs[selected.Path]; ok {
		return nil
	}
	return func() tea.Msg {
		return diffLoadedMsg{
			revision: m.revision,
			path:     selected.Path,
			diff:     m.loadDiff(m.context, selected),
		}
	}
}

func (m *Model) applySnapshot(snapshot Snapshot) {
	m.revision++
	m.changes = snapshot.Changes
	m.diffs = snapshot.Diffs
	m.err = snapshot.Error
	m.status = "Refreshed"
	m.selected = min(m.selected, max(0, len(m.changes)-1))
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	m.refreshDiff()
}

func (m *Model) resizeViewport() {
	_, rightWidth := m.layoutWidths()
	m.viewport.SetWidth(max(10, rightWidth-4))
	m.viewport.SetHeight(max(3, m.height-8))
}

func (m *Model) refreshDiff() {
	m.resizeViewport()
	if len(m.changes) == 0 {
		m.viewport.SetContent(m.styles.Muted.Render("No changes in this worktree."))
		return
	}
	diff := m.diffs[m.changes[m.selected].Path]
	if diff == "" {
		diff = fmt.Sprintf("No diff loaded for %s", m.changes[m.selected].Path)
	}
	m.viewport.SetContent(m.renderDiffContent(diff))
	m.viewport.GotoTop()
}

func (m Model) renderFiles(width, height int) string {
	lines := make([]string, 0, len(m.changes)+1)
	lines = append(lines, m.styles.Header.Render(fmt.Sprintf("%s %d files", iconFile, len(m.changes))))
	visibleRows := max(1, height-4)
	offset := m.listOffset(height)
	end := min(len(m.changes), offset+visibleRows)
	for i, change := range m.changes[offset:end] {
		index := offset + i
		line := renderFileLine(m.styles, change)
		if index == m.selected {
			line = m.styles.FileSelected.Width(max(1, width-4)).Render(line)
		} else {
			line = m.styles.FileItem.Width(max(1, width-4)).Render(line)
		}
		lines = append(lines, line)
	}
	if len(m.changes) == 0 {
		lines = append(lines, m.styles.Muted.Render("No changed files"))
	} else if end < len(m.changes) {
		lines = append(lines, m.styles.Muted.Render(fmt.Sprintf("… %d more", len(m.changes)-end)))
	}
	return m.styles.PanelFocused.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m Model) listOffset(height int) int {
	if len(m.changes) == 0 {
		return 0
	}
	visibleRows := max(1, height-4)
	if m.selected < visibleRows {
		return 0
	}
	offset := m.selected - visibleRows + 1
	maxOffset := max(0, len(m.changes)-visibleRows)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (m Model) renderDiff(width, height int) string {
	selected := iconFile + " Diff"
	if change := m.Selected(); change.Path != "" {
		selected = change.Path
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.styles.Header.Render(selected), m.viewport.View())
	return m.styles.Panel.Width(width).Height(height).Render(content)
}

func (m Model) renderHelp() string {
	lines := []string{
		m.styles.Title.Render(iconHelp + " Help"),
		"j/k or arrows: move file selection",
		iconRefresh + " r refresh: reload git worktree changes",
		iconTheme + " t themes: open theme picker",
		iconHelp + " ?: toggle this help",
		iconQuit + " q/esc: quit or close overlay",
	}
	return m.styles.PanelFocused.Width(min(m.width-2, 72)).Render(strings.Join(lines, "\n"))
}

func (m Model) renderThemePicker() string {
	lines := []string{m.styles.Title.Render(iconTheme + " Themes")}
	for i, name := range m.themeNames {
		line := name
		if i == m.themeCursor {
			line = m.styles.FileSelected.Width(28).Render(name)
		}
		lines = append(lines, line)
	}
	return m.styles.PanelFocused.Width(34).Render(strings.Join(lines, "\n"))
}

func (m Model) renderDiffContent(diff string) string {
	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff --git"):
			lines[i] = m.styles.DiffFileHeader.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = m.styles.DiffHunk.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = m.styles.DiffAddition.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = m.styles.DiffDeletion.Render(line)
		default:
			lines[i] = m.styles.Diff.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) layoutWidths() (int, int) {
	width := m.width
	if width <= 0 {
		width = 100
	}
	left := max(28, min(44, width/3))
	right := max(30, width-left-2)
	return left, right
}

func renderFileLine(styles theme.Styles, change gitview.FileChange) string {
	status := statusIcon(change.Status)
	counts := ""
	if change.Binary {
		counts = styles.Muted.Render(" " + iconBinary + " binary")
	} else if change.Additions != 0 || change.Deletions != 0 {
		counts = fmt.Sprintf(" %s %s", styles.Added.Render(fmt.Sprintf("+%d", change.Additions)), styles.Deleted.Render(fmt.Sprintf("-%d", change.Deletions)))
	}
	return fmt.Sprintf("%s %s%s", styles.Muted.Render(status), change.Path, counts)
}

func statusIcon(status gitview.ChangeStatus) string {
	switch status {
	case gitview.Added:
		return iconAdded
	case gitview.Modified:
		return iconModified
	case gitview.Deleted:
		return iconDeleted
	case gitview.Renamed:
		return iconRenamed
	case gitview.Untracked:
		return iconUntracked
	default:
		return iconFile
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func indexOf(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}

type reloadMsg struct {
	snapshot Snapshot
}

type diffLoadedMsg struct {
	revision int
	path     string
	diff     string
}
