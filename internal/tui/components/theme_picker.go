package components

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type ThemePickerAction int

const (
	ThemePickerNone ThemePickerAction = iota
	ThemePickerCancel
	ThemePickerApply
	ThemePickerToggleTransparent
	ThemePickerQuit
)

type ThemePicker struct {
	Name        string
	Transparent bool
	Names       []string
	Cursor      int
}

func NewThemePicker(name string, transparent bool, names []string) ThemePicker {
	return ThemePicker{Name: name, Transparent: transparent, Names: names}
}

func (p *ThemePicker) Normalize() {
	if p.Name == "" {
		p.Name = "tokyonight-night"
	}
	if len(p.Names) == 0 {
		p.Names = theme.Names()
	}
}

func (p *ThemePicker) Open() {
	themeIndex := slices.Index(p.Names, p.Name)
	if themeIndex < 0 {
		p.Cursor = 1
		return
	}
	p.Cursor = themeIndex + 1
}

func (p *ThemePicker) MoveCursor(delta int) {
	p.Cursor = clamp(p.Cursor+delta, 0, p.totalRows()-1)
}

func (p *ThemePicker) SetCursor(cursor int) {
	p.Cursor = clamp(cursor, 0, p.totalRows()-1)
}

func (p *ThemePicker) SelectedName() (string, bool) {
	themeIndex := p.Cursor - 1
	if themeIndex < 0 || themeIndex >= len(p.Names) {
		return "", false
	}
	return p.Names[themeIndex], true
}

func (p *ThemePicker) ToggleTransparent() {
	p.Transparent = !p.Transparent
}

func (p ThemePicker) totalRows() int {
	return len(p.Names) + 1
}

func (p ThemePicker) selectedThemeIndex() int {
	return max(0, p.Cursor-1)
}

func (p *ThemePicker) HandleKey(key string) ThemePickerAction {
	switch key {
	case "ctrl+c", "q":
		return ThemePickerQuit
	case "esc", "t":
		return ThemePickerCancel
	case "j", "down":
		p.MoveCursor(1)
	case "k", "up":
		p.MoveCursor(-1)
	case " ", "space":
		if p.Cursor == 0 {
			return ThemePickerToggleTransparent
		}
	case "enter":
		if p.Cursor != 0 {
			return ThemePickerApply
		}
	}
	return ThemePickerNone
}

func (p *ThemePicker) HandleMouse(localY, bodyHeight int) ThemePickerAction {
	index := localY - 2
	if index == 0 {
		p.SetCursor(0)
		return ThemePickerToggleTransparent
	}
	offset := p.Offset(bodyHeight)
	if index > 0 && index <= p.visibleThemeRows(bodyHeight) && offset+index-1 < len(p.Names) {
		p.SetCursor(offset + index)
		return ThemePickerApply
	}
	return ThemePickerNone
}

func (p *ThemePicker) HandleWheel(button tea.MouseButton) {
	switch button {
	case tea.MouseWheelUp:
		p.MoveCursor(-1)
	case tea.MouseWheelDown:
		p.MoveCursor(1)
	}
}

func (p ThemePicker) Render(screenWidth, bodyHeight int, styles theme.Styles, panel lipgloss.Style) string {
	width := p.width(screenWidth)
	lines := []string{styles.Title.Background(panel.GetBackground()).Width(width).Render(IconTheme + " Themes")}
	lines = append(lines, p.renderRow(0, width, styles))
	offset := p.Offset(bodyHeight)
	end := min(len(p.Names), offset+p.visibleThemeRows(bodyHeight))
	for themeIndex := offset; themeIndex < end; themeIndex++ {
		lines = append(lines, p.renderRow(themeIndex+1, width, styles))
	}
	contentRows := max(3, p.OverlayHeight(bodyHeight)-2)
	for len(lines) < contentRows-1 {
		lines = append(lines, styles.Diff.Width(width).Render(""))
	}
	lines = append(lines, p.renderFooter(width, styles, panel))
	return panel.Width(width + 6).Render(strings.Join(lines, "\n"))
}

func (p ThemePicker) width(screenWidth int) int {
	available := max(28, screenWidth-8)
	target := max(34, (screenWidth*2)/3)
	return min(target, available)
}

func (p ThemePicker) renderRow(index, width int, styles theme.Styles) string {
	prefix := "  "
	if index == p.Cursor {
		prefix = IconSelected + " "
	}
	line := prefix + p.rowLabel(index)
	if index == p.Cursor {
		return styles.FileSelected.Width(width).Render(line)
	}
	return styles.Diff.Width(width).Render(line)
}

func (p ThemePicker) rowLabel(index int) string {
	if index == 0 {
		state := IconToggleOff
		if p.Transparent {
			state = IconToggleOn
		}
		return "Transparent background  " + state
	}
	themeIndex := index - 1
	if themeIndex < 0 || themeIndex >= len(p.Names) {
		return ""
	}
	return p.Names[themeIndex]
}

func (p ThemePicker) renderFooter(width int, styles theme.Styles, panel lipgloss.Style) string {
	label := p.positionLabel()
	if p.Cursor == 0 {
		label = themePickerFooterLabel("space toggle", label, width)
	}
	return styles.Muted.
		Background(panel.GetBackground()).
		Width(width).
		Align(lipgloss.Right).
		Render(label)
}

func themePickerFooterLabel(hint, position string, width int) string {
	if hint == "" {
		return position
	}
	if position == "" {
		return hint
	}
	spaceWidth := width - ansi.StringWidth(hint) - ansi.StringWidth(position)
	if spaceWidth < 2 {
		return hint
	}
	return hint + strings.Repeat(" ", spaceWidth) + position
}

func (p ThemePicker) positionLabel() string {
	total := len(p.Names)
	if total == 0 {
		return "0/0"
	}
	current := clamp(p.Cursor, 0, total)
	return fmt.Sprintf("%d/%d", current, total)
}

func (p ThemePicker) Offset(bodyHeight int) int {
	visibleRows := p.visibleThemeRows(bodyHeight)
	themeCursor := p.selectedThemeIndex()
	if visibleRows <= 0 || len(p.Names) <= visibleRows || themeCursor < visibleRows {
		return 0
	}
	return min(themeCursor-visibleRows+1, len(p.Names)-visibleRows)
}

func (p ThemePicker) visibleRows(bodyHeight int) int {
	if p.totalRows() == 0 {
		return 0
	}
	contentRows := max(2, p.OverlayHeight(bodyHeight)-2)
	available := max(1, contentRows-2)
	return min(p.totalRows(), available)
}

func (p ThemePicker) visibleThemeRows(bodyHeight int) int {
	return max(0, p.visibleRows(bodyHeight)-1)
}

func (p ThemePicker) OverlayHeight(bodyHeight int) int {
	return max(6, (max(6, bodyHeight)*2)/3)
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
