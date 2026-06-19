package tui

import (
	"slices"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type themePicker struct {
	name        string
	transparent bool
	names       []string
	cursor      int
}

func (p *themePicker) normalize() {
	if p.name == "" {
		p.name = "tokyonight"
	}
	if len(p.names) == 0 {
		p.names = theme.Names()
	}
}

func (p *themePicker) open() {
	themeIndex := slices.Index(p.names, p.name)
	if themeIndex < 0 {
		p.cursor = 1
		return
	}
	p.cursor = themeIndex + 1
}

func (p *themePicker) moveCursor(delta int) {
	p.cursor = clamp(p.cursor+delta, 0, p.totalRows()-1)
}

func (p *themePicker) setCursor(cursor int) {
	p.cursor = clamp(cursor, 0, p.totalRows()-1)
}

func (p *themePicker) selectedName() (string, bool) {
	themeIndex := p.cursor - 1
	if themeIndex < 0 || themeIndex >= len(p.names) {
		return "", false
	}
	return p.names[themeIndex], true
}

func (p *themePicker) toggleTransparent() {
	p.transparent = !p.transparent
}

func (p themePicker) totalRows() int {
	return len(p.names) + 1
}

func (p themePicker) selectedThemeIndex() int {
	return max(0, p.cursor-1)
}
