package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type FooterHint struct {
	Icon  string
	Key   string
	Label string
}

type Footer struct {
	styles theme.Styles
}

func NewFooter(styles theme.Styles) Footer {
	return Footer{styles: styles}
}

func (f *Footer) SetStyles(styles theme.Styles) {
	f.styles = styles
}

func (f Footer) Text(hints []FooterHint) string {
	segments := make([]string, len(hints))
	for i, hint := range hints {
		segments[i] = f.Hint(hint)
	}
	return strings.Join(segments, f.styles.Footer.Render(" │ "))
}

func (f Footer) Hint(hint FooterHint) string {
	keyStyle := f.styles.Footer.Bold(true)
	keyText := keyStyle.Render(hint.Key)
	space := f.styles.Footer.Render(" ")
	labelText := f.styles.Footer.Render(hint.Label)
	if hint.Icon == "" {
		return keyText + space + labelText
	}
	return f.styles.Footer.Render(hint.Icon) + space + keyText + space + labelText
}

func (f Footer) Render(width int, text string, err error) string {
	style := f.styles.Footer
	if err != nil {
		style = f.styles.Error
		text = err.Error()
	}
	if width <= 0 {
		return style.Render(text)
	}
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return ansi.Cut(text, textWidth-width, textWidth)
	}
	return style.Render(strings.Repeat(" ", width-textWidth)) + text
}

func RightAlignText(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return ansi.Cut(text, textWidth-width, textWidth)
	}
	return strings.Repeat(" ", width-textWidth) + text
}
