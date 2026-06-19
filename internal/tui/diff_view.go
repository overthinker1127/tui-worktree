package tui

import "charm.land/bubbles/v2/viewport"

type diffView struct {
	viewport        viewport.Model
	lines           []string
	content         string
	contentKey      string
	showLineNumbers bool
}
