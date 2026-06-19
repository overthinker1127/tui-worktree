package components

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

const (
	prInputWidth = 48
	prBodyHeight = 5
)

type PRFocus int

const (
	PRTitle PRFocus = iota
	PRBody
)

type PR struct {
	title      textinput.Model
	body       textarea.Model
	focus      PRFocus
	submitting bool
	forgeCLI   string
}

func NewPR(styles theme.Styles) PR {
	var p PR
	p.Reset(styles)
	return p
}

func (p *PR) Reset(styles theme.Styles) {
	p.title = textinput.New()
	p.title.Prompt = ""
	p.title.Placeholder = "PR title"
	p.title.SetWidth(prInputWidth)
	p.title.Focus()

	p.body = textarea.New()
	p.body.Prompt = ""
	p.body.Placeholder = "PR description"
	p.body.ShowLineNumbers = false
	p.body.SetWidth(prInputWidth)
	p.body.SetHeight(prBodyHeight)
	p.body.Blur()

	p.focus = PRTitle
	p.submitting = false
	p.SetStyles(styles)
}

func (p *PR) SetStyles(styles theme.Styles) {
	p.title.SetStyles(prTextInputStyles(styles, p.title.Width()))
	p.body.SetStyles(prTextareaStyles(styles, p.body.Width(), p.body.Height()))
}

func (p *PR) SetForgeCLI(cli string) {
	p.forgeCLI = cli
}

func (p PR) ForgeCLI() string {
	return p.forgeCLI
}

func (p PR) Focus() PRFocus {
	return p.focus
}

func (p *PR) ToggleFocus() {
	if p.focus == PRTitle {
		p.title.Blur()
		p.body.Focus()
		p.focus = PRBody
		return
	}
	p.body.Blur()
	p.title.Focus()
	p.focus = PRTitle
}

func (p *PR) UpdateInput(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	if p.focus == PRTitle {
		p.title, cmd = p.title.Update(msg)
		return cmd
	}
	p.body, cmd = p.body.Update(msg)
	return cmd
}

func (p PR) Title() string {
	return strings.TrimSpace(p.title.Value())
}

func (p PR) Body() string {
	return strings.TrimSpace(p.body.Value())
}

func (p *PR) SetTitle(title string) {
	p.title.SetValue(title)
}

func (p *PR) SetBody(body string) {
	p.body.SetValue(body)
}

func (p PR) TitleView() string {
	return p.title.View()
}

func (p PR) BodyView() string {
	return p.body.View()
}

func (p *PR) Submit() bool {
	if p.submitting {
		return false
	}
	p.submitting = true
	return true
}

func (p *PR) FinishSubmit() {
	p.submitting = false
}

func (p PR) IsSubmitting() bool {
	return p.submitting
}

func prTextInputStyles(styles theme.Styles, width int) textinput.Styles {
	base := prInputStyle(styles, styles.Diff)
	placeholder := prInputStyle(styles, styles.Muted)
	prompt := prInputStyle(styles, styles.DiffHunk)
	if width > 0 {
		base = base.Width(width)
		placeholder = placeholder.Width(width)
	}
	return textinput.Styles{
		Focused: textinput.StyleState{
			Text:        base,
			Placeholder: placeholder,
			Suggestion:  placeholder,
			Prompt:      prompt,
		},
		Blurred: textinput.StyleState{
			Text:        base,
			Placeholder: placeholder,
			Suggestion:  placeholder,
			Prompt:      prompt,
		},
		Cursor: textinput.CursorStyle{
			Color: styles.DiffHunk.GetForeground(),
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}
}

func prTextareaStyles(styles theme.Styles, width, height int) textarea.Styles {
	base := prInputStyle(styles, styles.Diff)
	placeholder := prInputStyle(styles, styles.Muted)
	prompt := prInputStyle(styles, styles.DiffHunk)
	if width > 0 {
		base = base.Width(width)
		placeholder = placeholder.Width(width)
	}
	if height > 0 {
		base = base.Height(height)
	}
	return textarea.Styles{
		Focused: textarea.StyleState{
			Base:             base,
			Text:             base,
			LineNumber:       placeholder,
			CursorLineNumber: placeholder,
			CursorLine:       base,
			EndOfBuffer:      base,
			Placeholder:      placeholder,
			Prompt:           prompt,
		},
		Blurred: textarea.StyleState{
			Base:             base,
			Text:             base,
			LineNumber:       placeholder,
			CursorLineNumber: placeholder,
			CursorLine:       base,
			EndOfBuffer:      base,
			Placeholder:      placeholder,
			Prompt:           prompt,
		},
		Cursor: textarea.CursorStyle{
			Color: styles.DiffHunk.GetForeground(),
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}
}

func prInputStyle(styles theme.Styles, style lipgloss.Style) lipgloss.Style {
	return style.Background(styles.Panel.GetBackground())
}

func (p PR) Render(appWidth, bodyHeight int, styles theme.Styles, panel Panel, overlayPanel lipgloss.Style) string {
	width := min(max(48, appWidth-8), 78)
	titleInput := p.title
	bodyInput := p.body
	inputWidth := max(1, width-2)
	titleInput.SetWidth(inputWidth)
	titleInput.SetStyles(prTextInputStyles(styles, inputWidth))
	bodyInput.SetWidth(inputWidth)

	bodyPanelHeight := min(10, max(5, bodyHeight-8))
	bodyInputHeight := max(1, bodyPanelHeight-2)
	bodyInput.SetHeight(bodyInputHeight)
	bodyInput.SetStyles(prTextareaStyles(styles, inputWidth, bodyInputHeight))

	header := styles.Title.
		Background(overlayPanel.GetBackground()).
		Width(width).
		Render(IconPR + " Forge CLI: " + p.forgeCLI)
	title := panel.RenderPanel(width, 3, p.focus == PRTitle, "PR title", titleInput.View())
	bodyTitle := "PR description    <tab> focus    <c-o> create"
	body := panel.RenderPanel(width, bodyPanelHeight, p.focus == PRBody, bodyTitle, bodyInput.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, title, body)
}
