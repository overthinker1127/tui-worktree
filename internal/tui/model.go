package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type Config struct {
	Context          context.Context
	ThemeName        string
	Theme            theme.Styles
	ThemeNames       []string
	Worktrees        []WorktreeState
	SelectedWorktree int
	Changes          []gitview.FileChange
	Diff             string
	Diffs            map[string]string
	Error            error
	LoadDiff         func(context.Context, string, gitview.FileChange) string
	Reload           func(context.Context, string) Snapshot
	SaveTheme        func(string) error
}

type Snapshot struct {
	Worktrees        []WorktreeState
	SelectedWorktree int
	Changes          []gitview.FileChange
	Diffs            map[string]string
	Error            error
}

type WorktreeState struct {
	Worktree gitview.Worktree
	Changes  []gitview.FileChange
	Error    error
}

const (
	iconFile      = "󰈙"
	iconWorktree  = "󰙅"
	iconBranch    = ""
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
	styles            theme.Styles
	context           context.Context
	themeName         string
	themeNames        []string
	themeCursor       int
	worktrees         []WorktreeState
	selectedWorktree  int
	changes           []gitview.FileChange
	diffs             map[string]string
	selected          int
	revision          int
	refreshGeneration int
	width             int
	height            int
	err               error
	status            string
	showHelp          bool
	pickingTheme      bool
	loadDiff          func(context.Context, string, gitview.FileChange) string
	reload            func(context.Context, string) Snapshot
	saveTheme         func(string) error
	viewport          viewport.Model
}

func NewModel(cfg Config) Model {
	vp := viewport.New()
	vp.SoftWrap = false
	m := Model{
		styles:           cfg.Theme,
		context:          cfg.Context,
		themeName:        cfg.ThemeName,
		themeNames:       cfg.ThemeNames,
		worktrees:        cfg.Worktrees,
		selectedWorktree: cfg.SelectedWorktree,
		changes:          cfg.Changes,
		diffs:            cfg.Diffs,
		err:              cfg.Error,
		loadDiff:         cfg.LoadDiff,
		reload:           cfg.Reload,
		saveTheme:        cfg.SaveTheme,
		viewport:         vp,
		width:            100,
		height:           30,
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
	m.normalizeWorktrees()
	if cfg.Diff != "" && len(cfg.Changes) > 0 {
		m.diffs[m.diffKey(cfg.Changes[0])] = cfg.Diff
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
		if msg.generation != m.refreshGeneration {
			return m, nil
		}
		m.applySnapshot(msg.snapshot)
		return m, m.ensureSelectedDiffCmd()
	case diffLoadedMsg:
		if msg.revision != m.revision {
			return m, nil
		}
		if msg.path != "" {
			key := msg.worktree + "\x00" + msg.path
			m.diffs[key] = msg.diff
			if selected := m.Selected(); selected.Path == msg.path && m.SelectedWorktree().Path == msg.worktree {
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
		if m.selectWorktreeShortcut(msg.String()) {
			return m, m.ensureSelectedDiffCmd()
		}
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "?":
			m.showHelp = true
		case "t":
			m.openThemePicker()
		case "tab":
			m.moveWorktree(1)
			return m, m.ensureSelectedDiffCmd()
		case "shift+tab":
			m.moveWorktree(-1)
			return m, m.ensureSelectedDiffCmd()
		case "r":
			m.revision++
			m.refreshGeneration++
			m.status = "Refreshing..."
			return m, m.reloadCmd(m.refreshGeneration, m.SelectedWorktree().Path)
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

	worktreeHeight := m.worktreePaneHeight(contentHeight)
	worktrees := m.renderWorktrees(leftWidth, worktreeHeight)
	files := m.renderFiles(leftWidth, max(4, contentHeight-lipgloss.Height(worktrees)))
	sidebar := lipgloss.JoinVertical(lipgloss.Left, worktrees, files)
	diff := m.renderDiff(rightWidth, contentHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, diff)

	header := m.styles.Title.Render("Files changed")
	footer := m.styles.Footer.Render(m.footerText())
	if m.err != nil {
		footer = m.styles.Error.Render(m.err.Error())
	}
	if m.pickingTheme {
		body = m.renderOverlay(body, m.renderThemePicker())
	} else if m.showHelp {
		body = m.renderOverlay(body, m.renderHelp())
	}

	view := tea.NewView(m.styles.App.Width(m.width).Height(m.height).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, body, footer),
	))
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (m Model) footerText() string {
	text := fmt.Sprintf("1-9  tab  j/k  %s r  %s t  %s ?  %s q", iconRefresh, iconTheme, iconHelp, iconQuit)
	if m.status != "" {
		return m.status + "  " + text
	}
	return text
}

func (m Model) Selected() gitview.FileChange {
	if len(m.changes) == 0 || m.selected < 0 || m.selected >= len(m.changes) {
		return gitview.FileChange{}
	}
	return m.changes[m.selected]
}

func (m Model) SelectedWorktree() gitview.Worktree {
	if len(m.worktrees) == 0 || m.selectedWorktree < 0 || m.selectedWorktree >= len(m.worktrees) {
		return gitview.Worktree{}
	}
	return m.worktrees[m.selectedWorktree].Worktree
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

func (m *Model) moveWorktree(delta int) {
	if len(m.worktrees) == 0 {
		return
	}
	m.selectedWorktree += delta
	if m.selectedWorktree < 0 {
		m.selectedWorktree = len(m.worktrees) - 1
	}
	if m.selectedWorktree >= len(m.worktrees) {
		m.selectedWorktree = 0
	}
	m.selectWorktree(m.selectedWorktree)
}

func (m *Model) selectWorktreeShortcut(key string) bool {
	if len(key) != 1 || key[0] < '1' || key[0] > '9' {
		return false
	}
	index := int(key[0] - '1')
	if index >= len(m.worktrees) {
		return false
	}
	m.selectWorktree(index)
	return true
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
	m.status = ""
	if m.saveTheme != nil {
		if err := m.saveTheme(name); err != nil {
			m.status = fmt.Sprintf("Could not save theme: %s", err)
		}
	}
	m.refreshDiff()
}

func (m *Model) handleMouse(mouse tea.Mouse) bool {
	if m.pickingTheme {
		overlay := m.renderThemePicker()
		x, y := m.overlayPosition(overlay)
		if mouse.X < x || mouse.X >= x+lipgloss.Width(overlay) {
			return false
		}
		index := mouse.Y - y - 3
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
	contentHeight := max(4, m.height-4)
	worktreeHeight := m.worktreePaneHeight(contentHeight)
	bodyY := mouse.Y - 1
	if bodyY >= 0 && bodyY < worktreeHeight {
		index := m.worktreeListOffset(worktreeHeight) + bodyY - 2
		if index >= 0 && index < len(m.worktrees) {
			m.selectWorktree(index)
			return true
		}
		return false
	}
	fileY := bodyY - worktreeHeight
	index := m.listOffset(max(4, contentHeight-worktreeHeight)) + fileY - 2
	if index >= 0 && index < len(m.changes) {
		m.selected = index
		m.refreshDiff()
		return true
	}
	return false
}

func (m Model) reloadCmd(generation int, selectedWorktreePath string) tea.Cmd {
	reload := m.reload
	if reload == nil {
		return func() tea.Msg {
			return reloadMsg{generation: generation, snapshot: Snapshot{Changes: m.changes, Diffs: m.diffs, Error: fmt.Errorf("no reload source configured")}}
		}
	}
	return func() tea.Msg {
		return reloadMsg{generation: generation, snapshot: reload(m.context, selectedWorktreePath)}
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
	if _, ok := m.diffs[m.diffKey(selected)]; ok {
		return nil
	}
	if _, ok := m.diffs[selected.Path]; ok {
		return nil
	}
	worktreePath := m.SelectedWorktree().Path
	return func() tea.Msg {
		return diffLoadedMsg{
			revision: m.revision,
			worktree: worktreePath,
			path:     selected.Path,
			diff:     m.loadDiff(m.context, worktreePath, selected),
		}
	}
}

func (m *Model) applySnapshot(snapshot Snapshot) {
	m.revision++
	m.worktrees = snapshot.Worktrees
	m.selectedWorktree = snapshot.SelectedWorktree
	m.changes = snapshot.Changes
	m.diffs = snapshot.Diffs
	m.err = snapshot.Error
	m.status = "Refreshed"
	m.normalizeWorktrees()
	m.selected = min(m.selected, max(0, len(m.changes)-1))
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	m.refreshDiff()
}

func (m *Model) normalizeWorktrees() {
	if len(m.worktrees) == 0 {
		m.worktrees = []WorktreeState{{
			Worktree: gitview.Worktree{Path: ".", Branch: "current", Current: true},
			Changes:  m.changes,
			Error:    m.err,
		}}
		m.selectedWorktree = 0
		return
	}
	if m.selectedWorktree < 0 || m.selectedWorktree >= len(m.worktrees) {
		m.selectedWorktree = 0
	}
	m.changes = m.worktrees[m.selectedWorktree].Changes
	if m.worktrees[m.selectedWorktree].Error != nil {
		m.err = m.worktrees[m.selectedWorktree].Error
	}
}

func (m *Model) selectWorktree(index int) {
	if index < 0 || index >= len(m.worktrees) {
		return
	}
	m.selectedWorktree = index
	m.changes = m.worktrees[index].Changes
	m.err = m.worktrees[index].Error
	m.selected = 0
	m.refreshDiff()
}

func (m *Model) resizeViewport() {
	_, rightWidth := m.layoutWidths()
	contentHeight := max(4, m.height-4)
	m.viewport.SetWidth(max(10, panelInnerWidth(rightWidth)))
	m.viewport.SetHeight(max(3, panelInnerHeight(contentHeight)-1))
}

func (m *Model) refreshDiff() {
	m.resizeViewport()
	if len(m.changes) == 0 {
		m.viewport.SetContent(m.styles.Muted.Render("No changes in this worktree."))
		return
	}
	diff := m.diffs[m.diffKey(m.changes[m.selected])]
	if diff == "" {
		diff = m.diffs[m.changes[m.selected].Path]
	}
	if diff == "" {
		diff = fmt.Sprintf("No diff loaded for %s", m.changes[m.selected].Path)
	}
	m.viewport.SetContent(m.renderDiffContent(diff, m.viewport.Width()))
	m.viewport.GotoTop()
}

func (m Model) renderWorktrees(width, height int) string {
	lines := make([]string, 0, len(m.worktrees)+1)
	lines = append(lines, m.styles.Header.Render(fmt.Sprintf("[1]-%s %d worktrees", iconWorktree, len(m.worktrees))))
	contentWidth := panelInnerWidth(width)
	visibleRows := max(1, panelInnerHeight(height)-2)
	offset := m.worktreeListOffset(height)
	end := min(len(m.worktrees), offset+visibleRows)
	for i, worktree := range m.worktrees[offset:end] {
		index := offset + i
		line := renderWorktreeLine(m.styles, index, worktree)
		if index == m.selectedWorktree {
			line = m.styles.FileSelected.Width(contentWidth).Render(line)
		} else {
			line = m.styles.FileItem.Width(contentWidth).Render(line)
		}
		lines = append(lines, line)
	}
	if end < len(m.worktrees) {
		lines = append(lines, m.styles.Muted.Render(fmt.Sprintf("… %d more", len(m.worktrees)-end)))
	}
	innerHeight := panelInnerHeight(height)
	return m.styles.PanelFocused.Width(contentWidth).Height(innerHeight).Render(strings.Join(fillLines(lines, innerHeight), "\n"))
}

func (m Model) worktreeListOffset(height int) int {
	if len(m.worktrees) == 0 {
		return 0
	}
	visibleRows := max(1, panelInnerHeight(height)-2)
	if m.selectedWorktree < visibleRows {
		return 0
	}
	offset := m.selectedWorktree - visibleRows + 1
	maxOffset := max(0, len(m.worktrees)-visibleRows)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (m Model) renderFiles(width, height int) string {
	lines := make([]string, 0, len(m.changes)+1)
	lines = append(lines, m.styles.Header.Render(fmt.Sprintf("[2]-%s %d files", iconFile, len(m.changes))))
	contentWidth := panelInnerWidth(width)
	visibleRows := max(1, panelInnerHeight(height)-2)
	offset := m.listOffset(height)
	end := min(len(m.changes), offset+visibleRows)
	for i, change := range m.changes[offset:end] {
		index := offset + i
		line := renderFileLine(m.styles, change)
		if index == m.selected {
			line = m.styles.FileSelected.Width(contentWidth).Render(line)
		} else {
			line = m.styles.FileItem.Width(contentWidth).Render(line)
		}
		lines = append(lines, line)
	}
	if len(m.changes) == 0 {
		lines = append(lines, m.styles.Muted.Render("No changed files"))
	} else if end < len(m.changes) {
		lines = append(lines, m.styles.Muted.Render(fmt.Sprintf("… %d more", len(m.changes)-end)))
	}
	innerHeight := panelInnerHeight(height)
	return m.styles.PanelFocused.Width(contentWidth).Height(innerHeight).Render(strings.Join(fillLines(lines, innerHeight), "\n"))
}

func (m Model) listOffset(height int) int {
	if len(m.changes) == 0 {
		return 0
	}
	visibleRows := max(1, panelInnerHeight(height)-2)
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
	selected := "[3]-" + iconFile + " Diff"
	if change := m.Selected(); change.Path != "" {
		selected = "[3]-" + change.Path
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.styles.Header.Render(selected), m.viewport.View())
	return m.styles.Panel.Width(panelInnerWidth(width)).Height(panelInnerHeight(height)).Render(content)
}

func (m Model) renderHelp() string {
	lines := []string{
		m.styles.Title.Render(iconHelp + " Help"),
		"1-9: jump to worktree",
		"tab / shift+tab: switch worktree",
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

func (m Model) renderOverlay(background, foreground string) string {
	fgWidth := lipgloss.Width(foreground)
	fgHeight := lipgloss.Height(foreground)
	x, y := m.overlayPosition(foreground)

	bgLines := strings.Split(background, "\n")
	fgLines := strings.Split(foreground, "\n")
	if len(bgLines) < y+fgHeight {
		bgLines = append(bgLines, make([]string, y+fgHeight-len(bgLines))...)
	}
	for i, line := range fgLines {
		bgIndex := y + i
		bgLine := bgLines[bgIndex]
		left := ansi.Cut(bgLine, 0, x)
		right := ansi.Cut(bgLine, x+fgWidth, lipgloss.Width(bgLine))
		bgLines[bgIndex] = left + line + right
	}
	return strings.Join(bgLines, "\n")
}

func (m Model) overlayPosition(foreground string) (int, int) {
	fgWidth := lipgloss.Width(foreground)
	fgHeight := lipgloss.Height(foreground)
	bodyWidth := m.width
	bodyHeight := max(4, m.height-4)
	x := max(0, (bodyWidth-fgWidth)/2)
	y := max(0, (bodyHeight-fgHeight)/3)
	return x, y
}

func (m Model) renderDiffContent(diff string, width int) string {
	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff --git"):
			lines[i] = m.styles.DiffFileHeader.Width(width).Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = m.styles.DiffHunk.Width(width).Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = m.styles.DiffAddition.Width(width).Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = m.styles.DiffDeletion.Width(width).Render(line)
		default:
			lines[i] = m.styles.Diff.Width(width).Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) layoutWidths() (int, int) {
	width := m.width
	if width <= 0 {
		width = 100
	}
	if width < 64 {
		left := max(22, width/3)
		return left, max(24, width-left)
	}
	left := max(28, min(44, width/3))
	right := max(30, width-left)
	return left, right
}

func (m Model) worktreePaneHeight(contentHeight int) int {
	if len(m.worktrees) <= 1 {
		return 5
	}
	return min(max(6, len(m.worktrees)+4), max(6, contentHeight/3))
}

func (m Model) diffKey(change gitview.FileChange) string {
	return m.SelectedWorktree().Path + "\x00" + change.Path
}

func renderWorktreeLine(styles theme.Styles, index int, state WorktreeState) string {
	worktree := state.Worktree
	name := worktree.Branch
	if name == "" {
		name = worktree.Path
	}
	marker := " "
	if worktree.Current {
		marker = "•"
	}
	summary := fmt.Sprintf("%d", len(state.Changes))
	if state.Error != nil {
		summary = "!"
	}
	shortcut := " "
	if index < 9 {
		shortcut = fmt.Sprintf("%d", index+1)
	}
	return fmt.Sprintf("%s %s %s %s %s", styles.Muted.Render(shortcut), styles.Muted.Render(marker), styles.Muted.Render(iconBranch), name, styles.Muted.Render(summary))
}

func panelInnerWidth(width int) int {
	return max(1, width-2)
}

func panelInnerHeight(height int) int {
	return max(1, height-2)
}

func fillLines(lines []string, height int) []string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
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
	generation int
	snapshot   Snapshot
}

type diffLoadedMsg struct {
	revision int
	worktree string
	path     string
	diff     string
}
