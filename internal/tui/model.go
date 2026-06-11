package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type Config struct {
	Theme   theme.Styles
	Changes []gitview.FileChange
	Diff    string
	Diffs   map[string]string
	Error   error
}

type Model struct {
	styles   theme.Styles
	changes  []gitview.FileChange
	diffs    map[string]string
	selected int
	width    int
	height   int
	err      error
	viewport viewport.Model
}

func NewModel(cfg Config) Model {
	vp := viewport.New()
	vp.SoftWrap = false
	m := Model{
		styles:   cfg.Theme,
		changes:  cfg.Changes,
		diffs:    cfg.Diffs,
		err:      cfg.Error,
		viewport: vp,
		width:    100,
		height:   30,
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "j", "down":
			m.moveSelection(1)
		case "k", "up":
			m.moveSelection(-1)
		case "g", "home":
			m.selected = 0
			m.refreshDiff()
		case "G", "end":
			if len(m.changes) > 0 {
				m.selected = len(m.changes) - 1
				m.refreshDiff()
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
	footer := m.styles.Footer.Render("j/k move  g/G top/bottom  q quit")
	if m.err != nil {
		footer = m.styles.Error.Render(m.err.Error())
	}

	view := tea.NewView(m.styles.App.Width(m.width).Height(m.height).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, body, footer),
	))
	view.AltScreen = true
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
	lines = append(lines, m.styles.Header.Render(fmt.Sprintf("%d files", len(m.changes))))
	for i, change := range m.changes {
		line := renderFileLine(m.styles, change)
		if i == m.selected {
			line = m.styles.FileSelected.Width(max(1, width-4)).Render(line)
		} else {
			line = m.styles.FileItem.Width(max(1, width-4)).Render(line)
		}
		lines = append(lines, line)
	}
	if len(m.changes) == 0 {
		lines = append(lines, m.styles.Muted.Render("No changed files"))
	}
	return m.styles.PanelFocused.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m Model) renderDiff(width, height int) string {
	selected := "Diff"
	if change := m.Selected(); change.Path != "" {
		selected = change.Path
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.styles.Header.Render(selected), m.viewport.View())
	return m.styles.Panel.Width(width).Height(height).Render(content)
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
	status := string(change.Status)
	if len(status) > 1 {
		status = strings.ToUpper(status[:1])
	}
	counts := ""
	if change.Binary {
		counts = styles.Muted.Render(" binary")
	} else if change.Additions != 0 || change.Deletions != 0 {
		counts = fmt.Sprintf(" %s %s", styles.Added.Render(fmt.Sprintf("+%d", change.Additions)), styles.Deleted.Render(fmt.Sprintf("-%d", change.Deletions)))
	}
	return fmt.Sprintf("%s %s%s", styles.Muted.Render(status), change.Path, counts)
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
