package components

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

const toastDuration = 1500 * time.Millisecond

type ToastKind int

const (
	ToastInfo ToastKind = iota
	ToastError
	ToastSuccess
)

type ToastExpiredMsg struct {
	ID int
}

type Toast struct {
	Message string
	Kind    ToastKind
	toastID int
}

func (t *Toast) Info(message string) tea.Cmd {
	return t.show(ToastInfo, message)
}

func (t *Toast) Error(message string) tea.Cmd {
	return t.show(ToastError, message)
}

func (t *Toast) Success(message string) tea.Cmd {
	return t.show(ToastSuccess, message)
}

func (t *Toast) show(kind ToastKind, message string) tea.Cmd {
	t.toastID++
	t.Message = message
	t.Kind = kind
	id := t.toastID
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return ToastExpiredMsg{ID: id}
	})
}

func (t Toast) ID() int {
	return t.toastID
}

func (t *Toast) ClearExpired(id int) {
	if id != t.toastID {
		return
	}
	t.Message = ""
	t.Kind = ToastInfo
}

func (t Toast) Render(background string, styles theme.Styles, panel lipgloss.Style, width int) string {
	if t.Message == "" {
		return background
	}
	toast := t.RenderBox(styles, panel, width)
	x := max(0, width-lipgloss.Width(toast)-1)
	y := 1

	bgLines := strings.Split(background, "\n")
	if len(bgLines) == 0 {
		bgLines = []string{""}
	}
	toastLines := strings.Split(toast, "\n")
	for len(bgLines) < y+len(toastLines) {
		bgLines = append(bgLines, "")
	}
	toastWidth := lipgloss.Width(toast)
	for i, toastLine := range toastLines {
		bgIndex := y + i
		line := bgLines[bgIndex]
		left := ansi.Cut(line, 0, x)
		right := ansi.Cut(line, x+toastWidth, lipgloss.Width(line))
		bgLines[bgIndex] = left + toastLine + right
	}
	return strings.Join(bgLines, "\n")
}

func (t Toast) RenderBox(styles theme.Styles, panel lipgloss.Style, screenWidth int) string {
	width := max(8, min(44, screenWidth-6))
	title, accent := t.titleAndStyle(styles)
	panelBackground := panel.GetBackground()
	titleLine := accent.
		Bold(true).
		Background(panelBackground).
		Width(width).
		Render(IconStatus + " " + title)
	messageLine := styles.Diff.
		Background(panelBackground).
		Width(width).
		Render(ansi.Truncate(t.Message, width, "…"))
	return panel.
		Padding(0, 1).
		Render(strings.Join([]string{titleLine, messageLine}, "\n"))
}

func (t Toast) titleAndStyle(styles theme.Styles) (string, lipgloss.Style) {
	switch t.Kind {
	case ToastError:
		return "Error", styles.Error
	case ToastSuccess:
		return "Success", styles.Added
	default:
		return "Info", styles.DiffHunk
	}
}
