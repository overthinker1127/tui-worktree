package components

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

const defaultConfirmWidth = 64

type ConfirmChoice int

const (
	ConfirmNone ConfirmChoice = iota
	ConfirmYes
	ConfirmNo
	ConfirmQuit
)

type Confirm struct {
	title      string
	message    string
	styles     confirmStyles
	width      int
	submitting bool
	spinner    spinner.Model
}

type confirmStyles struct {
	panel lipgloss.Style
	title lipgloss.Style
	line  lipgloss.Style
	key   lipgloss.Style
}

func (c *Confirm) Open(title, message string) {
	c.title = title
	c.message = message
	c.submitting = false
}

func (c *Confirm) Restore(title, message string, submitting bool) {
	c.title = title
	c.message = message
	c.submitting = submitting
}

func (c *Confirm) SetStyles(styles theme.Styles, panel lipgloss.Style, width int) {
	if width <= 0 {
		width = defaultConfirmWidth
	}
	c.styles = confirmStyles{
		panel: panel,
		title: styles.Error,
		line:  styles.Diff,
		key:   styles.DiffHunk,
	}
	c.width = width
	if c.spinner.ID() == 0 {
		c.spinner = newConfirmSpinner(c.styles.key)
	} else {
		c.spinner.Style = c.styles.key
	}
}

func (c *Confirm) Close() {
	styles := c.styles
	width := c.width
	*c = Confirm{styles: styles, width: width, spinner: newConfirmSpinner(styles.key)}
}

func (c *Confirm) Submit() bool {
	if c.submitting {
		return false
	}
	c.submitting = true
	return true
}

func (c Confirm) IsSubmitting() bool {
	return c.submitting
}

func (c Confirm) Tick() tea.Msg {
	return c.spinner.Tick()
}

func (c *Confirm) Update(msg spinner.TickMsg) tea.Cmd {
	if !c.submitting {
		return nil
	}
	next, cmd := c.spinner.Update(msg)
	c.spinner = next
	return cmd
}

func (c Confirm) Choice(key string) ConfirmChoice {
	switch key {
	case "y", "Y", "enter":
		return ConfirmYes
	case "n", "N", "esc", "d":
		return ConfirmNo
	case "ctrl+c", "q":
		return ConfirmQuit
	default:
		return ConfirmNone
	}
}

func (c Confirm) Render() string {
	width := c.renderWidth()
	lineStyle := c.styles.line
	panel := c.styles.panel
	titleStyle := c.styles.title.
		Background(panel.GetBackground()).
		Bold(true)
	title := renderLine(titleStyle, width, c.title, 0)
	lines := []string{
		title,
		lineStyle.Width(width).Render(""),
	}
	for _, line := range strings.Split(c.message, "\n") {
		if line == "" {
			lines = append(lines, lineStyle.Width(width).Render(""))
			continue
		}
		lines = append(lines, renderLine(lineStyle, width, line, 0))
	}
	lines = append(lines, lineStyle.Width(width).Render(""))
	if c.submitting {
		lines = append(lines, c.renderProgress(width))
	} else {
		yes := confirmButton(c.styles, "Y", "es")
		no := confirmButton(c.styles, "N", "o")
		options := lipgloss.NewStyle().
			Background(panel.GetBackground()).
			Width(width).
			Align(lipgloss.Center).
			Render(yes + lineStyle.Render("     ") + no)
		lines = append(lines, options)
	}
	panel = panel.Padding(1, 2)
	return panel.Width(width + panel.GetHorizontalFrameSize()).Render(strings.Join(lines, "\n"))
}

func (c Confirm) renderProgress(width int) string {
	panel := c.styles.panel
	lineStyle := c.styles.line
	progress := c.spinner.View() + lineStyle.Inline(true).Render(" In progress")
	return lipgloss.NewStyle().
		Background(panel.GetBackground()).
		Width(width).
		Align(lipgloss.Center).
		Render(progress)
}

func (c Confirm) renderWidth() int {
	if c.width <= 0 {
		return defaultConfirmWidth
	}
	return c.width
}

func newConfirmSpinner(style lipgloss.Style) spinner.Model {
	return spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(style),
	)
}

func confirmButton(styles confirmStyles, key, label string) string {
	keyText := styles.key.Bold(true).Render("[" + key + "]")
	return keyText + styles.line.Render(label)
}

func renderLine(style lipgloss.Style, width int, text string, offset int) string {
	if width <= 0 {
		return ""
	}
	offset = max(0, offset)
	rendered := style.Inline(true).MaxWidth(width).Render(ansi.Cut(text, offset, offset+width))
	if renderedWidth := lipgloss.Width(rendered); renderedWidth < width {
		rendered += style.Inline(true).Render(strings.Repeat(" ", width-renderedWidth))
	}
	return rendered
}
