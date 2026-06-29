package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
	"github.com/overthinker1127/tui-worktree/internal/tui/components"
)

func TestModelViewShowsFileListAndDiff(t *testing.T) {
	tm, err := theme.Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		Theme: theme.NewStyles(tm),
		Changes: []gitview.FileChange{
			{Path: "README.md", Status: gitview.Modified, Additions: 4, Deletions: 2},
		},
		Diff: "diff --git a/README.md b/README.md\n+hello\n-world",
	})

	view := model.View().Content

	if strings.Contains(view, "Files changed") {
		t.Fatalf("View() should not render Files changed title: %q", view)
	}
	for _, want := range []string{"[1]-", "[2]-", "[3]-", "README.md", "+4", "-2", "diff --git"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q in %q", want, view)
		}
	}
	if !strings.Contains(view, "● [2]-") {
		t.Fatalf("View() should focus files panel by default: %q", view)
	}
	for _, want := range []string{components.IconFile, components.IconModified} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing icon %q in %q", want, view)
		}
	}
}

func TestModelViewSupportsTransparentBackground(t *testing.T) {
	tm, err := theme.Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		ThemeName:   "tokyonight-night",
		Theme:       theme.NewStylesWithOptions(tm, theme.StyleOptions{Transparent: true}),
		Transparent: true,
		Width:       100,
		Height:      24,
		Changes: []gitview.FileChange{
			{Path: "a.go", Status: gitview.Modified, Additions: 1, Deletions: 1},
		},
		Diffs: map[string]string{"a.go": strings.Join([]string{
			"diff --git a/a.go b/a.go",
			"@@ -1 +1 @@",
			"-old",
			"+new",
		}, "\n")},
	})

	view := model.View().Content

	if containsEscape(view, "48;2;") {
		t.Fatalf("transparent view should not paint truecolor backgrounds: %q", view)
	}
	for _, token := range []string{styleForegroundToken(model.styles.Added), styleForegroundToken(model.styles.Deleted)} {
		if token != "" && !strings.Contains(view, token) {
			t.Fatalf("transparent view should keep diff foreground token %q in %q", token, view)
		}
	}
}

func TestNewModelUsesConfiguredInitialSize(t *testing.T) {
	tm, err := theme.Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		Theme:  theme.NewStyles(tm),
		Width:  132,
		Height: 41,
		Changes: []gitview.FileChange{
			{Path: "README.md", Status: gitview.Modified},
		},
		Diff: "diff --git a/README.md b/README.md\n+hello\n-world",
	})

	if model.width != 132 || model.height != 41 {
		t.Fatalf("initial size = %dx%d, want 132x41", model.width, model.height)
	}
	for i, line := range strings.Split(model.View().Content, "\n") {
		if got := lipgloss.Width(line); got > model.width {
			t.Fatalf("line %d width = %d, want <= %d: %q", i, got, model.width, ansi.Strip(line))
		}
	}
}

func TestModelMovesSelectionDown(t *testing.T) {
	tm, err := theme.Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		Theme: theme.NewStyles(tm),
		Changes: []gitview.FileChange{
			{Path: "a.go", Status: gitview.Modified},
			{Path: "b.go", Status: gitview.Added},
		},
	})

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	got := next.(Model).selectedFileValue()

	if got.Path != "b.go" {
		t.Fatalf("Selected() = %q, want b.go", got.Path)
	}
}

func TestQuestionMarkDoesNotOpenHelpOverlay(t *testing.T) {
	model := testModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	view := next.(Model).View().Content

	if strings.Contains(view, "Help") || strings.Contains(view, "auto-refresh") {
		t.Fatalf("question mark should not open help overlay: %q", view)
	}
}

func TestThemePickerAppliesTheme(t *testing.T) {
	model := testModel(t)
	model.themePicker.Names = []string{"tokyonight-night", "gruvbox-dark"}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if got.themePicker.Name != "gruvbox-dark" {
		t.Fatalf("themeName = %q, want gruvbox-dark", got.themePicker.Name)
	}
	if strings.Contains(got.View().Content, "Theme changed") {
		t.Fatalf("theme success message should be hidden: %q", got.View().Content)
	}
}

func TestThemePickerPreservesTransparentStyles(t *testing.T) {
	tm, err := theme.Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		ThemeName:   "tokyonight-night",
		Theme:       theme.NewStylesWithOptions(tm, theme.StyleOptions{Transparent: true}),
		Transparent: true,
		ThemeNames:  []string{"tokyonight-night", "gruvbox-dark"},
		Changes:     []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}},
		Diffs:       map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"},
	})

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if got.themePicker.Name != "gruvbox-dark" {
		t.Fatalf("themeName = %q, want gruvbox-dark", got.themePicker.Name)
	}
	if containsEscape(got.View().Content, "48;2;") {
		t.Fatalf("transparent theme switch should not paint backgrounds: %q", got.View().Content)
	}
}

func TestThemePickerSavesTheme(t *testing.T) {
	model := testModel(t)
	model.themePicker.Names = []string{"tokyonight-night", "kanagawa-wave"}
	var saved string
	model.saveTheme = func(name string) error {
		saved = name
		return nil
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	_, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))

	if saved != "kanagawa-wave" {
		t.Fatalf("saved theme = %q, want kanagawa-wave", saved)
	}
}

func TestThemePickerRendersAsOverlay(t *testing.T) {
	model := testModel(t)
	model.themePicker.Names = []string{"tokyonight-night", "gruvbox-dark"}
	model.openThemePicker()

	view := model.View().Content
	plain := ansi.Strip(view)
	for _, want := range []string{"Themes", "Transparent background  ", "a.go", components.IconSelected + " tokyonight-night"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("theme overlay view missing %q in %q", want, view)
		}
	}
	if strings.Contains(plain, "Transparent background  off") || strings.Contains(plain, "Transparent background  on") {
		t.Fatalf("theme overlay should render transparent toggle as icons: %q", view)
	}
}

func TestThemePickerTransparentRowSpaceTogglesBackground(t *testing.T) {
	model := testModel(t)
	model.themePicker.Names = []string{"tokyonight-night"}
	var saved *bool
	model.saveTransparent = func(value bool) error {
		saved = &value
		return nil
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: " ", Code: ' '}))
	got := next.(Model)

	if !got.themePicker.Transparent {
		t.Fatal("transparent row should enable transparent background")
	}
	if got.mode != modeThemePicker {
		t.Fatal("transparent toggle should keep theme picker open")
	}
	if saved == nil || !*saved {
		t.Fatalf("saved transparent = %v, want true", saved)
	}
	if containsEscape(got.View().Content, "48;2;") {
		t.Fatalf("transparent row should remove painted backgrounds: %q", got.View().Content)
	}
}

func TestThemePickerTransparentRowEnterDoesNotToggleBackground(t *testing.T) {
	model := testModel(t)
	model.themePicker.Names = []string{"tokyonight-night"}
	model.themePicker.Transparent = true
	var saved *bool
	model.saveTransparent = func(value bool) error {
		saved = &value
		return nil
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if !got.themePicker.Transparent {
		t.Fatal("enter on transparent row should not disable transparent background")
	}
	if got.mode != modeThemePicker {
		t.Fatal("enter on transparent row should keep theme picker open")
	}
	if saved != nil {
		t.Fatalf("transparent setting should not be saved on enter, got %v", *saved)
	}
}

func TestThemePickerRowsUsePanelColors(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		ThemeName:  "solarized-light",
		Theme:      theme.NewStyles(tm),
		ThemeNames: []string{"solarized-light", "ayu-mirage"},
	})
	model.openThemePicker()

	row := findRenderedLine(model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle()), "  ayu-mirage")
	if row == "" {
		t.Fatalf("theme picker missing unselected row: %q", model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle()))
	}
	for _, token := range []string{styleForegroundToken(model.styles.Diff), styleBackgroundToken(model.styles.Diff)} {
		if token == "" || !strings.Contains(row, token) {
			t.Fatalf("theme picker row should use panel colors token %q in %q", token, row)
		}
	}
}

func TestOverlayUsesPanelBorderColor(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		ThemeName:  "solarized-light",
		Theme:      theme.NewStyles(tm),
		ThemeNames: []string{"solarized-light", "ayu-mirage"},
	})
	model.openThemePicker()
	picker := model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())
	panelBorder := styleForegroundToken(model.styles.Panel)
	focusedBorder := styleForegroundToken(model.styles.PanelFocused)

	if panelBorder == "" || !strings.Contains(picker, panelBorder) {
		t.Fatalf("theme picker should use panel border token %q in %q", panelBorder, picker)
	}
	if focusedBorder != "" && strings.Contains(picker, focusedBorder) {
		t.Fatalf("theme picker should not use focused border token %q in %q", focusedBorder, picker)
	}
}

func TestOverlayBorderUsesPanelBackground(t *testing.T) {
	tm, err := theme.Preset("github-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		ThemeName:  "github-light",
		Theme:      theme.NewStyles(tm),
		ThemeNames: []string{"github-light", "github-dark"},
	})
	model.openThemePicker()
	picker := model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())
	firstLine, _, _ := strings.Cut(picker, "\n")
	panelBackground := styleBackgroundToken(model.styles.Panel)

	if panelBackground == "" || !strings.Contains(firstLine, panelBackground) {
		t.Fatalf("theme picker border should use panel background token %q in %q", panelBackground, firstLine)
	}
}

func TestThemePickerScrollsToCursor(t *testing.T) {
	model := testModel(t)
	model.height = 12
	model.themePicker.Names = []string{
		"theme-01", "theme-02", "theme-03", "theme-04", "theme-05",
		"theme-06", "theme-07", "theme-08", "theme-09", "theme-10",
		"theme-11", "theme-12", "theme-13", "theme-14", "theme-15",
	}
	model.themePicker.Cursor = 13
	model.mode = modeThemePicker

	view := ansi.Strip(model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle()))

	if !strings.Contains(view, components.IconSelected+" theme-13") {
		t.Fatalf("theme picker should keep cursor visible: %q", view)
	}
	if strings.Contains(view, "theme-01") {
		t.Fatalf("theme picker should not render every theme when constrained: %q", view)
	}
}

func TestThemePickerShowsCurrentThemePosition(t *testing.T) {
	model := testModel(t)
	model.height = 12
	model.themePicker.Names = []string{
		"theme-01", "theme-02", "theme-03", "theme-04", "theme-05",
		"theme-06", "theme-07", "theme-08", "theme-09", "theme-10",
		"theme-11", "theme-12", "theme-13", "theme-14", "theme-15",
	}
	model.themePicker.Cursor = 13
	model.mode = modeThemePicker

	view := ansi.Strip(model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle()))
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Fatalf("theme picker view too short: %q", view)
	}
	statusLine := lines[len(lines)-2]
	if !strings.Contains(statusLine, "13/15") {
		t.Fatalf("theme picker footer should show current theme position in bottom row: %q", view)
	}
}

func TestThemePickerShowsSpaceToggleHintOnTransparentRow(t *testing.T) {
	model := testModel(t)
	model.height = 12
	model.themePicker.Names = []string{"tokyonight-night", "kanagawa-wave"}
	model.themePicker.Cursor = 0
	model.mode = modeThemePicker

	view := ansi.Strip(model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle()))
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Fatalf("theme picker view too short: %q", view)
	}
	statusLine := lines[len(lines)-2]
	if !strings.Contains(statusLine, "space toggle") {
		t.Fatalf("theme picker footer should show space toggle hint on transparent row: %q", view)
	}

	model.themePicker.Cursor = 1
	view = ansi.Strip(model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle()))
	if strings.Contains(view, "space toggle") {
		t.Fatalf("theme picker footer should hide space toggle hint away from transparent row: %q", view)
	}
}

func TestThemePickerUsesTwoThirdsOverlayHeight(t *testing.T) {
	model := testModel(t)
	model.width = 120
	model.height = 30
	model.themePicker.Names = []string{
		"theme-01", "theme-02", "theme-03", "theme-04", "theme-05",
		"theme-06", "theme-07", "theme-08", "theme-09", "theme-10",
		"theme-11", "theme-12", "theme-13", "theme-14", "theme-15",
		"theme-16", "theme-17", "theme-18", "theme-19", "theme-20",
	}
	model.themePicker.Cursor = 1
	model.mode = modeThemePicker

	overlay := model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())
	if got, want := lipgloss.Height(overlay), model.themePicker.OverlayHeight(model.bodyHeight()); got != want {
		t.Fatalf("theme overlay height = %d, want %d", got, want)
	}
	if got, maxWidth := lipgloss.Width(overlay), model.width; got >= maxWidth {
		t.Fatalf("theme overlay width = %d, should be smaller than screen width %d", got, maxWidth)
	}
}

func TestThemePickerMouseWheelScrollsRows(t *testing.T) {
	model := testModel(t)
	model.height = 12
	model.themePicker.Names = []string{
		"theme-01", "theme-02", "theme-03", "theme-04", "theme-05",
		"theme-06", "theme-07", "theme-08", "theme-09", "theme-10",
	}
	model.themePicker.Cursor = 1
	model.mode = modeThemePicker
	overlay := model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())
	x, y := components.OverlayPosition(overlay, model.width, model.bodyHeight())

	next, _ := model.Update(tea.MouseWheelMsg{X: x + 2, Y: y + 2, Button: tea.MouseWheelDown})
	got := next.(Model)
	if got.themePicker.Cursor != 2 {
		t.Fatalf("theme cursor after wheel down = %d, want 2", got.themePicker.Cursor)
	}

	next, _ = got.Update(tea.MouseWheelMsg{X: x + 2, Y: y + 2, Button: tea.MouseWheelUp})
	got = next.(Model)
	if got.themePicker.Cursor != 1 {
		t.Fatalf("theme cursor after wheel up = %d, want 1", got.themePicker.Cursor)
	}
}

func TestMouseClickSelectsScrolledTheme(t *testing.T) {
	model := testModel(t)
	model.height = 12
	model.themePicker.Names = []string{
		"ayu-mirage", "catppuccin-mocha", "dracula", "everforest", "gruvbox-dark",
		"kanagawa-wave", "monokai", "nord", "one-dark", "rose-pine",
		"solarized-dark", "tokyonight-night", "tokyonight-storm", "vscode-dark",
	}
	model.themePicker.Cursor = 8
	model.mode = modeThemePicker
	model.saveTheme = func(string) error { return nil }
	overlay := model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())
	x, y := components.OverlayPosition(overlay, model.width, model.bodyHeight())
	targetRow := 8 - model.themePicker.Offset(model.bodyHeight())

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: x + 2, Y: y + 2 + targetRow}))
	got := next.(Model)

	if got.themePicker.Name != "nord" {
		t.Fatalf("themeName = %q, want nord", got.themePicker.Name)
	}
}

func TestFooterOmitsCurrentThemeName(t *testing.T) {
	model := testModel(t)

	view := model.View().Content
	if strings.Contains(view, "theme:") {
		t.Fatalf("footer should not show current theme name: %q", view)
	}
}

func TestFooterShowsDescriptiveHints(t *testing.T) {
	model := testModel(t)

	footer := model.footerText()
	plain := ansi.Strip(footer)
	for _, want := range []string{"1/2/3 panels", "tab worktree", "hjkl move", "0/$ edge", "/ filter", "e edit", "t themes", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("footer missing %q in %q", want, footer)
		}
	}
	for _, unwanted := range []string{"d delete", "w wrap", "n nums", "? help"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("files footer should omit %q in %q", unwanted, footer)
		}
	}
	if strings.Contains(footer, "auto 5s") {
		t.Fatalf("footer should not show auto-refresh interval: %q", footer)
	}
	for _, want := range []string{components.IconWorktree, components.IconKey, components.IconFile, components.IconEdit, components.IconTheme, components.IconQuit, " │ "} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer missing status bar segment %q in %q", want, footer)
		}
	}
	for _, want := range []string{"1/2/3", "tab", "hjkl", "0/$", "/", "e", "t", "q"} {
		if !strings.Contains(footer, "\x1b[1;") || !strings.Contains(plain, want) {
			t.Fatalf("footer key %q should be bold in %q", want, footer)
		}
	}
}

func TestFooterShowsWorktreeActionsWhenWorktreeFocused(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees

	plain := ansi.Strip(model.footerText())
	for _, want := range []string{"1/2/3 panels", "tab worktree", "hjkl move", "d delete", "p PR", "m merge", "t themes", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("worktree footer missing %q in %q", want, plain)
		}
	}
	for _, unwanted := range []string{"e edit", "w wrap", "n nums", "? help"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("worktree footer should omit %q in %q", unwanted, plain)
		}
	}
}

func TestFooterShowsDiffActionsWhenDiffFocused(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneDiff

	plain := ansi.Strip(model.footerText())
	for _, want := range []string{"1/2/3 panels", "tab worktree", "hjkl scroll", "e edit", "w wrap", "n nums", "t themes", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("diff footer missing %q in %q", want, plain)
		}
	}
	for _, unwanted := range []string{"d delete", "p PR", "m merge", "? help"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("diff footer should omit %q in %q", unwanted, plain)
		}
	}
}

func TestFooterKeysUseFooterBackgroundOnly(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{Theme: theme.NewStyles(tm)})

	hint := model.footer.Hint(components.FooterHint{Key: "w", Label: "wrap"})
	selectedBackground := styleBackgroundToken(model.styles.FileSelected)
	footerBackground := styleBackgroundToken(model.styles.Footer)

	if footerBackground == "" || !strings.Contains(hint, footerBackground) {
		t.Fatalf("footer label should use footer background %q in %q", footerBackground, hint)
	}
	if selectedBackground != "" && strings.Contains(hint, selectedBackground) {
		t.Fatalf("footer key should not use selected badge background %q in %q", selectedBackground, hint)
	}
	if !strings.Contains(hint, "\x1b[1;") {
		t.Fatalf("footer key should stay bold: %q", hint)
	}
}

func TestFooterHintSpacesUseFooterBackground(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{Theme: theme.NewStyles(tm)})
	hint := model.footer.Hint(components.FooterHint{Icon: components.IconKey, Key: "1/2/3", Label: "panels"})
	background := styleBackgroundToken(model.styles.Footer)

	if background == "" {
		t.Fatal("footer background token should not be empty")
	}
	if got := strings.Count(hint, background); got < 5 {
		t.Fatalf("footer hint spaces should use footer background, got %d background spans in %q", got, hint)
	}
}

func TestPanelTitlePaddingUsesPanelBackground(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{Theme: theme.NewStyles(tm)})
	top := model.panel.RenderPanelTop(model.styles.PanelFocused, true, "[3]-"+components.IconFile+" Diff", 40)
	background := styleBackgroundToken(model.styles.PanelFocused)

	if background == "" {
		t.Fatal("panel background token should not be empty")
	}
	if got := strings.Count(top, background); got < 5 {
		t.Fatalf("panel title padding should use panel background, got %d background spans in %q", got, top)
	}
}

func TestSelectedFileRowPreservesLineStatColors(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	styles := theme.NewStyles(tm)
	content := renderFileLineWithBackground(styles, gitview.FileChange{Path: "internal/tui/model.go", Status: gitview.Modified, Additions: 2, Deletions: 1}, "", styles.FileSelected.GetBackground())
	row := renderScrollableListRow(styles.FileSelected, components.IconSelected+" ", content, 0, 60, false)

	for _, token := range []string{
		styleForegroundToken(styles.Added),
		styleForegroundToken(styles.Deleted),
	} {
		if token != "" && !strings.Contains(row, token) {
			t.Fatalf("selected row should preserve line stat token %q in %q", token, row)
		}
	}
	if token := styleBackgroundToken(styles.Panel); token != "" && strings.Contains(row, token) {
		t.Fatalf("selected row should not contain panel background token %q in %q", token, row)
	}
}

func TestSelectedFileListPreservesLineStatColors(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{{
		Path:      "internal/tui/model.go",
		Status:    gitview.Modified,
		Additions: 2,
		Deletions: 1,
	}}
	model.selected = 0
	model.refreshDiff()

	files := model.fileListComponent().Render(model.styles, model.panel, model.focusedPane == paneFiles, 48, 6, model.fileBadge(), model.visibleOverlapCount())
	for _, token := range []string{
		styleForegroundToken(model.styles.Added),
		styleForegroundToken(model.styles.Deleted),
	} {
		if token != "" && !strings.Contains(files, token) {
			t.Fatalf("selected file list should preserve line stat token %q in %q", token, files)
		}
	}
}

func TestFileFilterMatchRendersBold(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	styles := theme.NewStyles(tm)
	line := renderFileLine(styles, gitview.FileChange{Path: "internal/tui/model.go", Status: gitview.Modified}, "model")

	if !strings.Contains(line, "\x1b[1;") {
		t.Fatalf("filtered file line should bold matching text: %q", line)
	}
	if got := ansi.Strip(line); !strings.Contains(got, "internal/tui/model.go") {
		t.Fatalf("filtered file line changed visible path: %q", got)
	}
}

func TestListLineSpacesUsePanelBackground(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	styles := theme.NewStyles(tm)
	background := styleBackgroundToken(styles.Panel)
	fileLine := renderFileLine(styles, gitview.FileChange{Path: "internal/tui/model.go", Status: gitview.Modified, Additions: 6, Deletions: 2}, "")
	worktreeLine := renderWorktreeLine(styles, 0, WorktreeState{Worktree: gitview.Worktree{Branch: "main", Current: true}})

	if background == "" {
		t.Fatal("panel background token should not be empty")
	}
	if got := strings.Count(fileLine, background); got < 6 {
		t.Fatalf("file line spaces should use panel background, got %d in %q", got, fileLine)
	}
	if got := strings.Count(worktreeLine, background); got < 5 {
		t.Fatalf("worktree line spaces should use panel background, got %d in %q", got, worktreeLine)
	}
}

func TestEmptyFileListMessageUsesPanelBackground(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		Theme:   theme.NewStyles(tm),
		Changes: nil,
		Diffs:   map[string]string{},
	})
	files := model.fileListComponent().Render(model.styles, model.panel, model.focusedPane == paneFiles, 36, 8, model.fileBadge(), model.visibleOverlapCount())
	background := styleBackgroundToken(model.styles.Panel)

	if background == "" {
		t.Fatal("panel background token should not be empty")
	}
	if !strings.Contains(files, "No changed files") {
		t.Fatalf("file list missing empty message: %q", files)
	}
	for _, line := range strings.Split(files, "\n") {
		if !strings.Contains(line, "No changed files") {
			continue
		}
		if !strings.Contains(line, background) {
			t.Fatalf("empty file list message should use panel background %q in %q", background, line)
		}
		return
	}
	t.Fatalf("file list missing empty message line: %q", files)
}

func TestFooterAlignsToRightEdge(t *testing.T) {
	model := testModel(t)
	model.width = 160

	line := components.RightAlignText(model.footerText(), model.width)

	if got := lipgloss.Width(line); got != model.width {
		t.Fatalf("right aligned footer width = %d, want %d", got, model.width)
	}
	if !strings.HasSuffix(line, model.footerText()) {
		t.Fatalf("footer should be right aligned: %q", line)
	}
	if !strings.HasPrefix(line, " ") {
		t.Fatalf("footer should have leading fill before text: %q", line)
	}
	if got := components.RightAlignText("abcdef", 3); got != "def" {
		t.Fatalf("narrow footer = %q, want def", got)
	}
}

func TestRenderedFooterLeadingPaddingUsesFooterBackground(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{Theme: theme.NewStyles(tm), Width: 120})
	footer := model.footer.Render(model.width, model.footerText(), model.err)
	background := styleBackgroundToken(model.styles.Footer)

	if background == "" {
		t.Fatal("footer background token should not be empty")
	}
	if !strings.HasPrefix(footer, "\x1b[") || !strings.Contains(footer[:min(len(footer), 64)], background) {
		t.Fatalf("footer leading padding should use footer background %q in %q", background, footer)
	}
}

func TestViewKeepsFooterOnLastLine(t *testing.T) {
	model := testModel(t)
	model.width = 90
	model.height = 24

	lines := strings.Split(model.View().Content, "\n")
	last := ansi.Strip(lines[len(lines)-1])

	if !strings.Contains(last, "e edit") || !strings.Contains(last, "q quit") {
		t.Fatalf("last line should contain footer hints, got %q in view %q", last, model.View().Content)
	}
}

func TestViewLinesFitTerminalWidth(t *testing.T) {
	model := testModel(t)
	model.width = 94
	model.height = 38
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Branch: "main", Current: true}, Changes: model.changes},
		{Worktree: gitview.Worktree{Branch: "fix/v1-persistent-theme-config"}, Changes: model.changes},
	}
	model.changes = []gitview.FileChange{
		{Path: "internal/tui/model.go", Status: gitview.Modified, Additions: 1, Deletions: 1},
		{Path: "internal/tui/model_test.go", Status: gitview.Modified, Additions: 13},
	}
	model.diffs = map[string]string{
		model.diffKey(model.changes[0]): "diff --git a/internal/tui/model.go b/internal/tui/model.go\n@@ -830,7 +830,7 @@ func (m Model) layoutWidths() (int, int) {\n-\treturn max(4, m.height-1)\n+\treturn max(4, m.height-2)",
	}
	model.refreshDiff()

	fillsWidth := false
	for i, line := range strings.Split(model.View().Content, "\n") {
		got := lipgloss.Width(line)
		if got > model.width {
			t.Fatalf("line %d width = %d, want <= %d: %q", i, got, model.width, ansi.Strip(line))
		}
		if got == model.width {
			fillsWidth = true
		}
	}
	if !fillsWidth {
		t.Fatalf("view never filled terminal width %d: %q", model.width, model.View().Content)
	}
}

func TestSelectedRowsRenderVisiblePointer(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Branch: "main", Current: true}, Changes: model.changes},
		{Worktree: gitview.Worktree{Branch: "feature"}, Changes: model.changes},
	}
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Added},
	}
	model.selectedWorktree = 1
	model.selected = 1
	model.refreshDiff()

	view := ansi.Strip(model.View().Content)
	for _, want := range []string{components.IconSelected + "   " + components.IconBranch + " feature", components.IconSelected + " " + components.IconAdded + " b.go"} {
		if !strings.Contains(view, want) {
			t.Fatalf("selected row missing visible pointer %q in %q", want, view)
		}
	}
}

func TestHorizontalScrollMovesFocusedFileLine(t *testing.T) {
	model := testModel(t)
	model.width = 120
	model.changes = []gitview.FileChange{{
		Path:      "internal/some/really/long/path/that/needs/scrolling/model.go",
		Status:    gitview.Modified,
		Additions: 12,
		Deletions: 3,
	}}
	model.diffs = map[string]string{}
	model.focusedPane = paneFiles
	model.refreshDiff()

	initial := ansi.Strip(model.View().Content)
	if !strings.Contains(initial, "internal/") || !strings.Contains(initial, "scrolling/model.go") || !strings.Contains(initial, "…") {
		t.Fatalf("initial view should middle-ellipsize long path with tail: %q", initial)
	}
	if !strings.Contains(initial, "+12 -3") {
		t.Fatalf("initial view should keep line stats visible: %q", initial)
	}

	for range 20 {
		next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
		model = next.(Model)
	}
	scrolled := ansi.Strip(model.View().Content)
	if !strings.Contains(scrolled, components.IconSelected+" ly/long/path/that/needs/scrolling") {
		t.Fatalf("horizontal scroll did not move file row: %q", scrolled)
	}
	if strings.Contains(scrolled, "…") {
		t.Fatalf("scrolled file row should show a continuous slice, not middle ellipsis: %q", scrolled)
	}
	if model.fileScrollX != 20 {
		t.Fatalf("fileScrollX = %d, want 20", model.fileScrollX)
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "0", Code: '0'}))
	model = next.(Model)
	reset := ansi.Strip(model.View().Content)
	if model.fileScrollX != 0 {
		t.Fatalf("0 should reset fileScrollX, got %d", model.fileScrollX)
	}
	if !strings.Contains(reset, "…") || !strings.Contains(reset, "scrolling/model.go") {
		t.Fatalf("0 should restore middle-ellipsized path: %q", reset)
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "$", Code: '$'}))
	model = next.(Model)
	end := ansi.Strip(model.View().Content)
	if model.fileScrollX != model.maxFileScrollX() {
		t.Fatalf("$ fileScrollX = %d, want %d", model.fileScrollX, model.maxFileScrollX())
	}
	if !strings.Contains(end, "needs/scrolling/model.go +12 -3") {
		t.Fatalf("$ should scroll file row to path end: %q", end)
	}
}

func TestDiffWrapToggleDisablesSoftWrap(t *testing.T) {
	model := testModel(t)

	if !model.diff.SoftWrap() {
		t.Fatal("diff wrap should be enabled by default")
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "w", Code: 'w'}))
	got := next.(Model)

	if got.diff.SoftWrap() {
		t.Fatal("w should disable diff wrap on first press")
	}
	if got.toast.Message != "diff wrap off" || got.toast.Kind != components.ToastInfo {
		t.Fatalf("toast = %#v, want info diff wrap off", got.toast)
	}
	if !strings.Contains(got.View().Content, "diff wrap off") {
		t.Fatalf("toast should render in view: %q", got.View().Content)
	}
}

func TestToastHelpersSetTypedVariants(t *testing.T) {
	model := testModel(t)

	infoCmd := model.toast.Info("saved config")
	if infoCmd == nil {
		t.Fatal("info toast should return expiration command")
	}
	if model.toast.Message != "saved config" || model.toast.Kind != components.ToastInfo {
		t.Fatalf("info toast = %#v, want info saved config", model.toast)
	}

	errorCmd := model.toast.Error("editor failed")
	if errorCmd == nil {
		t.Fatal("error toast should return expiration command")
	}
	if model.toast.Message != "editor failed" || model.toast.Kind != components.ToastError {
		t.Fatalf("error toast = %#v, want error editor failed", model.toast)
	}

	successCmd := model.toast.Success("deleted feature")
	if successCmd == nil {
		t.Fatal("success toast should return expiration command")
	}
	if model.toast.Message != "deleted feature" || model.toast.Kind != components.ToastSuccess {
		t.Fatalf("success toast = %#v, want success deleted feature", model.toast)
	}
}

func TestToastRendersAsVariantOverlay(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.toast.Error("editor failed: boom")

	view := ansi.Strip(model.View().Content)

	for _, want := range []string{"Error", "editor failed: boom", "╭", "╯"} {
		if !strings.Contains(view, want) {
			t.Fatalf("toast overlay missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, components.IconStatus+" editor failed: boom") {
		t.Fatalf("toast should not render as compact status text: %q", view)
	}
}

func TestToastFrameUsesPanelColors(t *testing.T) {
	tm, err := theme.Preset("github-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		ThemeName: "github-light",
		Theme:     theme.NewStyles(tm),
		Width:     100,
		Height:    24,
	})
	model.toast.Info("line numbers off")
	toast := model.toast.RenderBox(model.styles, model.overlayPanelStyle(), model.width)
	firstLine, _, _ := strings.Cut(toast, "\n")
	panelBorder := styleForegroundToken(model.styles.Panel)
	panelBackground := styleBackgroundToken(model.styles.Panel)
	infoAccent := foregroundOnlyToken(model.styles.DiffHunk)

	for _, token := range []string{panelBorder, panelBackground} {
		if token == "" || !strings.Contains(firstLine, token) {
			t.Fatalf("toast frame should use panel token %q in %q", token, firstLine)
		}
	}
	if infoAccent != "" && strings.Contains(firstLine, infoAccent) {
		t.Fatalf("toast frame should not use info accent token %q in %q", infoAccent, firstLine)
	}
	if !strings.Contains(toast, infoAccent) {
		t.Fatalf("toast title should keep info accent token %q in %q", infoAccent, toast)
	}
}

func TestToastExpires(t *testing.T) {
	model := testModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "w", Code: 'w'}))
	model = next.(Model)
	if model.toast.Message == "" {
		t.Fatal("expected toast after wrap toggle")
	}

	next, _ = model.Update(components.ToastExpiredMsg{ID: model.toast.ID()})
	model = next.(Model)
	if model.toast.Message != "" {
		t.Fatalf("toast = %#v, want empty after expiration", model.toast)
	}
}

func TestDiffPaneSupportsHorizontalScrollWhenWrapIsOff(t *testing.T) {
	model := testModel(t)
	model.width = 72
	model.height = 20
	model.changes = []gitview.FileChange{{Path: "long.go", Status: gitview.Modified}}
	model.diffs = map[string]string{
		model.diffKey(model.changes[0]): "diff --git a/long.go b/long.go\n+const value = \"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz\"",
	}
	model.refreshDiff()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "3", Code: '3'}))
	model = next.(Model)
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "w", Code: 'w'}))
	model = next.(Model)
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
	model = next.(Model)

	if got := model.diff.XOffset(); got == 0 {
		t.Fatal("l should scroll diff viewport right when wrap is off")
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	model = next.(Model)
	if got := model.diff.XOffset(); got != 0 {
		t.Fatalf("h should scroll diff viewport left, got x offset %d", got)
	}
}

func TestLineNumbersRenderByDefault(t *testing.T) {
	model := testModel(t)
	model.setDiffContent("diff --git a/a.go b/a.go\n@@ -10,2 +20,2 @@ func main() {\n unchanged\n-old\n+new")

	if !model.diff.ShowLineNumbers() {
		t.Fatal("line numbers should be enabled by default")
	}
	view := ansi.Strip(model.diff.RenderContent())
	for _, want := range []string{"   20 │  unchanged", "  -11 │ -old", "   21 │ +new"} {
		if !strings.Contains(view, want) {
			t.Fatalf("line number gutter missing %q in %q", want, view)
		}
	}
}

func TestLineNumberToggleHidesGutter(t *testing.T) {
	model := testModel(t)
	model.setDiffContent("@@ -1,1 +1,1 @@\n unchanged")

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	model = next.(Model)

	if model.diff.ShowLineNumbers() {
		t.Fatal("n should disable line numbers on first press")
	}
	view := ansi.Strip(model.diff.RenderContent())
	if strings.Contains(view, "1 │") {
		t.Fatalf("line number gutter should be hidden: %q", view)
	}
	if model.toast.Message != "line numbers off" || model.toast.Kind != components.ToastInfo {
		t.Fatalf("toast = %#v, want info line numbers off", model.toast)
	}
}

func TestLineNumberGutterKeepsWrappedContinuationAligned(t *testing.T) {
	model := testModel(t)
	model.width = 72
	model.height = 20
	model.setDiffContent("@@ -1,1 +1,1 @@\n+" + strings.Repeat("x", model.diff.TextWidth(model.diff.Width())) + "tail")

	lines := strings.Split(ansi.Strip(model.diff.RenderContent()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected wrapped numbered diff: %q", model.diff.RenderContent())
	}
	if !strings.Contains(lines[1], "    1 │ +") {
		t.Fatalf("first wrapped line missing new line number gutter: %q", lines[1])
	}
	if strings.Contains(lines[2], "1 │") {
		t.Fatalf("continuation line should not repeat line number: %q", lines[2])
	}
	if !strings.HasPrefix(lines[2], strings.Repeat(" ", model.diff.GutterWidth())) {
		t.Fatalf("continuation line should keep blank gutter: %q", lines[2])
	}
}

func TestDiffAddsSpacerBeforeLaterHunksInSameFile(t *testing.T) {
	model := testModel(t)
	model.diff.ToggleLineNumbers()
	model.setDiffContent(strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"@@ -1,1 +1,1 @@ func a()",
		"-old",
		"+new",
		"@@ -20,1 +20,1 @@ func b()",
		"-old",
		"+new",
		"diff --git a/b.go b/b.go",
		"@@ -1,1 +1,1 @@ func c()",
		"-old",
		"+new",
	}, "\n"))

	viewLines := strings.Split(ansi.Strip(model.diff.RenderContent()), "\n")
	trimmed := make([]string, 0, len(viewLines))
	for _, line := range viewLines {
		trimmed = append(trimmed, strings.TrimRight(line, " "))
	}
	view := strings.Join(trimmed, "\n")
	if strings.Contains(view, "...") {
		t.Fatalf("separator should not render literal ellipsis: %q", view)
	}
	if !strings.Contains(view, "@@ -1,1 +1,1 @@ func a()\n-old\n+new\n\n@@ -20,1 +20,1 @@ func b()") {
		t.Fatalf("same-file hunk separator missing in %q", view)
	}
	if strings.Contains(view, "diff --git a/b.go b/b.go\n\n@@") {
		t.Fatalf("first hunk in new file should not get separator: %q", view)
	}
}

func TestDiffPanelFillsPaddingRowsWithDiffBackground(t *testing.T) {
	model := testModel(t)
	model.width = 90
	model.height = 24
	_, rightWidth := model.layoutWidths()
	diff := model.renderDiff(rightWidth, model.bodyHeight())
	lines := strings.Split(diff, "\n")
	if len(lines) < 10 {
		t.Fatalf("diff panel rendered too few lines: %q", diff)
	}

	emptyRow := lines[len(lines)-2]
	if !strings.Contains(ansi.Strip(emptyRow), "│") {
		t.Fatalf("expected panel body row, got %q", emptyRow)
	}
	if !containsEscape(emptyRow, "48;2;") {
		t.Fatalf("empty diff row should keep diff background: %q", emptyRow)
	}
}

func TestDiffViewportFillsEmptyRowsWithDiffBackground(t *testing.T) {
	model := testModel(t)
	model.width = 90
	model.height = 24
	model.refreshDiff()

	lines := strings.Split(model.diff.RenderContent(), "\n")
	if len(lines) < 10 {
		t.Fatalf("viewport rendered too few lines: %q", model.diff.RenderContent())
	}

	emptyLine := lines[len(lines)-1]
	if !containsEscape(emptyLine, "48;2;") {
		t.Fatalf("empty viewport line should keep diff background: %q", emptyLine)
	}
}

func TestDiffViewportFillsShortRowsBeforeReset(t *testing.T) {
	model := testModel(t)
	model.width = 90
	model.height = 24
	model.setDiffContent("short")

	line := strings.Split(model.diff.RenderContent(), "\n")[0]
	if strings.Contains(line, "short\x1b[m ") {
		t.Fatalf("short diff row resets before padding spaces: %q", line)
	}
	if !containsEscape(line, "48;2;") {
		t.Fatalf("short diff row should keep diff background: %q", line)
	}
}

func TestDiffWrappedTailFillsToViewportWidthWithLineBackground(t *testing.T) {
	model := testModel(t)
	model.width = 90
	model.height = 24
	model.refreshDiff()
	model.setDiffContent("-" + strings.Repeat("x", model.diff.Width()) + "}}")

	lines := strings.Split(model.diff.RenderContent(), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped diff tail line: %q", model.diff.RenderContent())
	}
	filled := lines[1]

	if got := lipgloss.Width(filled); got != model.diff.Width() {
		t.Fatalf("filled width = %d, want %d: %q", got, model.diff.Width(), filled)
	}
	if !strings.Contains(filled, styleBackgroundToken(model.styles.DiffDeletion)) {
		t.Fatalf("wrapped deletion tail should keep deletion background: %q", filled)
	}
	if strings.Contains(filled, "\x1b[m ") {
		t.Fatalf("wrapped deletion tail resets before fill spaces: %q", filled)
	}
}

func TestDiffHunkHeaderColorsLineRanges(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.refreshDiff()
	model.setDiffContent(strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"@@ -145,7 +145,8 @@ func main() {",
		" unchanged",
	}, "\n"))

	view := model.diff.RenderContent()
	addedToken := styleForegroundToken(model.styles.Added)
	deletedToken := styleForegroundToken(model.styles.Deleted)

	for _, token := range []string{addedToken, deletedToken} {
		if token != "" && !strings.Contains(view, token) {
			t.Fatalf("hunk header should preserve range color token %q in %q", token, view)
		}
	}
	if !strings.Contains(ansi.Strip(view), "-145,7 +145,8") {
		t.Fatalf("hunk header missing line ranges: %q", view)
	}
}

func TestDiffSyntaxHighlightsStaticKeywords(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.refreshDiff()
	model.setDiffContent(strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"@@ -1,13 +1,13 @@",
		"+func main() {",
		"+  if ready { const value = function() {} }",
		"+  def build(self): return class_name",
		"+  public class App implements Runnable {}",
		"+  async fn run() -> Result<()> { await task }",
		"+  switch kind { case value: return }",
		"+  typedef struct Node union Value",
		"+  record User(val name: String)",
		"+  fun render() = true",
		"+  var count = 1",
		"+  let next = count",
		"+  elseif fallback { return }",
		"}",
	}, "\n"))

	view := model.diff.RenderContent()
	keywordToken := foregroundOnlyToken(model.styles.DiffKeyword)

	if keywordToken == "" || !strings.Contains(view, keywordToken) {
		t.Fatalf("diff syntax keywords should use keyword token %q in %q", keywordToken, view)
	}
	for _, want := range []string{
		"func", "if", "const", "function", "def", "public", "class", "implements",
		"async", "fn", "await", "switch", "case", "typedef", "struct", "union",
		"record", "val", "fun", "var", "let", "elseif",
	} {
		if !strings.Contains(ansi.Strip(view), want) {
			t.Fatalf("diff view missing keyword %q in %q", want, view)
		}
	}
}

func TestDiffSyntaxTrimsHeaderPathWhitespace(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.refreshDiff()
	model.setDiffContent(strings.Join([]string{
		"diff --git a/dir space/file name.go b/dir space/file name.go",
		"--- a/dir space/file name.go\t",
		"+++ b/dir space/file name.go\t",
		"@@ -1,1 +1,1 @@",
		"+func main() {}",
	}, "\n"))

	view := model.diff.RenderContent()
	keywordToken := foregroundOnlyToken(model.styles.DiffKeyword)

	if keywordToken == "" || !strings.Contains(view, keywordToken) {
		t.Fatalf("diff syntax should trim header path whitespace and use keyword token %q in %q", keywordToken, view)
	}
}

func TestDiffSyntaxSkipsNonCodeFiles(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.refreshDiff()
	model.setDiffContent(strings.Join([]string{
		"diff --git a/README.md b/README.md",
		"@@ -1,1 +1,1 @@",
		"+true return class function should read as plain text",
	}, "\n"))

	view := model.diff.RenderContent()
	keywordToken := foregroundOnlyToken(model.styles.DiffKeyword)

	if keywordToken != "" && strings.Contains(view, keywordToken) {
		t.Fatalf("non-code file should not use keyword token %q in %q", keywordToken, view)
	}
	if !strings.Contains(ansi.Strip(view), "true return class function") {
		t.Fatalf("non-code diff should still render text: %q", view)
	}
}

func TestDiffSyntaxHighlightsNonGoCodeFiles(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.refreshDiff()
	model.setDiffContent(strings.Join([]string{
		"diff --git a/app.ts b/app.ts",
		"@@ -1,1 +1,1 @@",
		"+export function render() { return true }",
	}, "\n"))

	view := model.diff.RenderContent()
	keywordToken := foregroundOnlyToken(model.styles.DiffKeyword)

	if keywordToken == "" || !strings.Contains(view, keywordToken) {
		t.Fatalf("non-Go code file should use keyword token %q in %q", keywordToken, view)
	}
	for _, want := range []string{"export", "function", "return", "true"} {
		if !strings.Contains(ansi.Strip(view), want) {
			t.Fatalf("diff view missing keyword %q in %q", want, view)
		}
	}
}

func TestDiffSyntaxDoesNotHighlightKeywordFragments(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.refreshDiff()
	model.setDiffContent(strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"@@ -1,1 +1,1 @@",
		"+constellation functionality letter variable",
	}, "\n"))

	view := model.diff.RenderContent()
	keywordToken := foregroundOnlyToken(model.styles.DiffKeyword)

	if keywordToken != "" && strings.Contains(view, keywordToken) {
		t.Fatalf("keyword fragments should not be highlighted with %q in %q", keywordToken, view)
	}
}

func TestRefreshDiffKeepsViewportWhenContentIsUnchanged(t *testing.T) {
	model := testModel(t)
	model.height = 8
	diff := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"@@ -1,12 +1,12 @@",
		" line-1",
		" line-2",
		" line-3",
		" line-4",
		" line-5",
		" line-6",
		" line-7",
		" line-8",
		" line-9",
	}, "\n")
	model.changes = []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}}
	model.diffs = map[string]string{"a.go": diff}
	model.refreshDiff()
	model.diff.SetYOffset(4)

	model.refreshDiff()

	if got := model.diff.YOffset(); got != 4 {
		t.Fatalf("refreshDiff with unchanged content should keep y offset = %d, want 4", got)
	}
}

func TestRefreshDiffResetsViewportWhenSelectionChangesToSameContent(t *testing.T) {
	model := testModel(t)
	model.height = 8
	diff := strings.Join([]string{
		"diff --git a/file.go b/file.go",
		"@@ -1,12 +1,12 @@",
		" line-1",
		" line-2",
		" line-3",
		" line-4",
		" line-5",
		" line-6",
		" line-7",
		" line-8",
		" line-9",
	}, "\n")
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Modified},
	}
	model.diffs = map[string]string{
		model.diffKey(model.changes[0]): diff,
		model.diffKey(model.changes[1]): diff,
	}
	model.selected = 0
	model.refreshDiff()
	model.diff.SetYOffset(4)

	model.selected = 1
	model.refreshDiff()

	if got := model.diff.YOffset(); got != 0 {
		t.Fatalf("selection change with same diff content y offset = %d, want 0", got)
	}
}

func TestViewShowsWorktreeSidebar(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{
			Worktree: gitview.Worktree{Path: "/repo", Branch: "main", Current: true},
			Changes:  []gitview.FileChange{{Path: "main.go", Status: gitview.Modified}},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
			Changes:  []gitview.FileChange{{Path: "feature.go", Status: gitview.Added}},
		},
	}
	model.selectedWorktree = 0
	model.changes = model.worktrees[0].Changes
	model.refreshDiff()

	view := model.View().Content
	for _, want := range []string{"[1]-", "[2]-", "[3]-", "worktrees", "main", "feature", "main.go"} {
		if !strings.Contains(view, want) {
			t.Fatalf("worktree sidebar missing %q in %q", want, view)
		}
	}
}

func TestOverlapTargetsDetectSamePathAndExcludeCurrentWorktree(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{
			Worktree: gitview.Worktree{Path: "/repo", Branch: "main", Current: true},
			Changes:  []gitview.FileChange{{Path: "internal/tui/model.go", Status: gitview.Modified}},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
			Changes:  []gitview.FileChange{{Path: "internal/tui/model.go", Status: gitview.Modified}},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/other", Branch: "other"},
			Changes:  []gitview.FileChange{{Path: "internal/tui/other.go", Status: gitview.Modified}},
		},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	targets := model.overlapTargetsFor(model.selectedFileValue())

	if len(targets) != 1 {
		t.Fatalf("overlapTargetsFor() len = %d, want 1: %#v", len(targets), targets)
	}
	if targets[0].Worktree.Path != "/repo/.worktrees/feature" || targets[0].Change.Path != "internal/tui/model.go" {
		t.Fatalf("overlap target = %#v, want feature model.go", targets[0])
	}
}

func TestOverlapTargetsIncludeRenameOldPath(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{
			Worktree: gitview.Worktree{Path: "/repo", Branch: "main", Current: true},
			Changes:  []gitview.FileChange{{Path: "internal/tui/model.go", OldPath: "internal/tui/old_model.go", Status: gitview.Renamed}},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
			Changes:  []gitview.FileChange{{Path: "internal/tui/old_model.go", Status: gitview.Modified}},
		},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	targets := model.overlapTargetsFor(model.selectedFileValue())

	if len(targets) != 1 {
		t.Fatalf("rename old path should overlap, got %#v", targets)
	}
	if got := targets[0].Change.Path; got != "internal/tui/old_model.go" {
		t.Fatalf("overlap change path = %q, want old path match", got)
	}
}

func TestFilesListRendersOverlapMarkerAndCount(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{
			Worktree: gitview.Worktree{Path: "/repo", Branch: "main", Current: true},
			Changes: []gitview.FileChange{
				{Path: "internal/tui/model.go", Status: gitview.Modified},
				{Path: "README.md", Status: gitview.Modified},
			},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
			Changes:  []gitview.FileChange{{Path: "internal/tui/model.go", Status: gitview.Modified}},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/experiment", Branch: "experiment"},
			Changes:  []gitview.FileChange{{Path: "internal/tui/model.go", Status: gitview.Modified}},
		},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	plain := ansi.Strip(model.fileListComponent().Render(model.styles, model.panel, model.focusedPane == paneFiles, 58, 8, model.fileBadge(), model.visibleOverlapCount()))

	for _, want := range []string{"[2]-", "2 files", "2 overlaps", components.IconWarning, "internal/tui/model.go", "overlap 2"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("files list missing %q in %q", want, plain)
		}
	}
	if strings.Contains(plain, "README.md overlap") {
		t.Fatalf("non-overlapped file should not show overlap marker: %q", plain)
	}
}

func TestOverlapKeyOpensPickerOnlyForOverlappedFile(t *testing.T) {
	model := overlapTestModel(t)

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("overlap picker should not load a diff yet")
	}
	if got.mode != modeOverlapPicker || len(got.overlap.targets) != 1 {
		t.Fatalf("o should open picker for overlapped file: %#v", got)
	}
	if view := ansi.Strip(got.View().Content); !strings.Contains(view, "Overlaps for a.go") || !strings.Contains(view, "feature") || !strings.Contains(view, "/repo/.worktrees/feature") {
		t.Fatalf("overlap picker view missing details: %q", view)
	}

	model = overlapTestModel(t)
	model.selected = 1
	model.refreshDiff()
	next, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	got = next.(Model)

	if cmd == nil {
		t.Fatal("non-overlapped file should show info toast")
	}
	if got.mode == modeOverlapPicker {
		t.Fatal("non-overlapped file should not open picker")
	}
	if got.toast.Message != "no overlaps for selected file" || got.toast.Kind != components.ToastInfo {
		t.Fatalf("toast = %#v, want no overlaps info", got.toast)
	}
}

func TestOverlapPickerEnterOpensCompareAndEscRestoresNormalDiff(t *testing.T) {
	model := overlapTestModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if got.mode != modeOverlapCompare {
		t.Fatalf("enter should open compare and close picker: comparing=%v picking=%v", got.mode == modeOverlapCompare, got.mode == modeOverlapPicker)
	}
	if got.overlap.compareTarget.Worktree.Branch != "feature" {
		t.Fatalf("compare target branch = %q, want feature", got.overlap.compareTarget.Worktree.Branch)
	}
	if cmd == nil {
		t.Fatal("opening compare should load overlap diff")
	}
	next, _ = got.Update(cmd())
	got = next.(Model)
	if !strings.Contains(got.overlap.compareDiff, "+feature") {
		t.Fatalf("compare diff was not loaded: %q", got.overlap.compareDiff)
	}
	view := ansi.Strip(got.View().Content)
	for _, want := range []string{"main ↔ feature", "a.go", "+current", "+feature"} {
		if !strings.Contains(view, want) {
			t.Fatalf("compare view missing %q in %q", want, view)
		}
	}

	got.diff.SetYOffset(3)
	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	got = next.(Model)

	if got.mode == modeOverlapCompare || got.mode == modeOverlapPicker {
		t.Fatal("esc should close overlap modal state")
	}
	if got.selectedWorktreeValue().Branch != "main" || got.selectedFileValue().Path != "a.go" {
		t.Fatalf("esc changed selection: worktree=%#v selected=%#v", got.selectedWorktreeValue(), got.selectedFileValue())
	}
}

func TestOverlapCompareLoadSurvivesAutoRefreshRevisionChange(t *testing.T) {
	model := overlapTestModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	model = next.(Model)
	if cmd == nil {
		t.Fatal("opening compare should load overlap diff")
	}
	next, _ = model.Update(autoRefreshMsg{})
	model = next.(Model)

	next, _ = model.Update(cmd())
	got := next.(Model)

	if got.overlap.compareLoading {
		t.Fatal("compare load should finish even if auto-refresh changed model revision")
	}
	if !strings.Contains(got.overlap.compareDiff, "+feature") {
		t.Fatalf("compare diff was not loaded after refresh: %q", got.overlap.compareDiff)
	}
}

func TestOverlapPickerEscClosesWithoutChangingSelection(t *testing.T) {
	model := overlapTestModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	got := next.(Model)

	if got.mode == modeOverlapPicker || got.mode == modeOverlapCompare {
		t.Fatal("esc should close picker")
	}
	if got.selectedWorktreeValue().Branch != "main" || got.selectedFileValue().Path != "a.go" {
		t.Fatalf("esc changed selection: worktree=%#v selected=%#v", got.selectedWorktreeValue(), got.selectedFileValue())
	}
}

func TestCompareScrollKeysMoveSharedOffsets(t *testing.T) {
	model := overlapTestModel(t)
	model.height = 8
	longLine := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 4)
	model.diffs[model.diffKey(model.selectedFileValue())] = "diff --git a/a.go b/a.go\n@@ -1,12 +1,12 @@\n line-1\n line-2\n line-3\n line-4\n line-5\n line-6\n line-7\n line-8\n+" + longLine
	model.loadDiff = func(_ context.Context, worktreePath string, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n@@ -1 +1 @@\n+" + longLine + worktreePath
	}
	model.refreshDiff()
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	model = next.(Model)
	next, _ = model.Update(cmd())
	model = next.(Model)

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	model = next.(Model)
	if model.overlap.compareYOffset == 0 {
		t.Fatal("j should scroll compare overlay down")
	}

	model.diff.ToggleWrap()
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
	model = next.(Model)
	if model.overlap.compareXOffset == 0 {
		t.Fatal("l should scroll compare overlay horizontally when wrap is off")
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	model = next.(Model)
	if model.overlap.compareYOffset != 0 {
		t.Fatalf("g should jump compare overlay to top, got %d", model.overlap.compareYOffset)
	}
}

func TestWorktreeLineOmitsShortcutAndChangeCount(t *testing.T) {
	model := testModel(t)
	line := renderWorktreeLine(model.styles, 3, WorktreeState{
		Worktree: gitview.Worktree{Branch: "feature"},
		Changes:  []gitview.FileChange{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}, {Path: "d.go"}, {Path: "e.go"}, {Path: "f.go"}, {Path: "g.go"}},
	})
	plain := ansi.Strip(line)

	if strings.Contains(plain, "3") || strings.Contains(plain, "4") || strings.Contains(plain, "7") {
		t.Fatalf("worktree line should not include shortcut or change count: %q", plain)
	}
	if !strings.Contains(plain, components.IconBranch+" feature") {
		t.Fatalf("worktree line missing branch label: %q", plain)
	}
}

func TestProtectedWorktreeLineShowsLock(t *testing.T) {
	model := testModel(t)
	line := renderWorktreeLine(model.styles, 0, WorktreeState{
		Worktree: gitview.Worktree{Branch: "main", Protected: true},
	})

	if !strings.Contains(ansi.Strip(line), components.IconProtected+" "+components.IconBranch+" main") {
		t.Fatalf("protected worktree line should show lock: %q", line)
	}
}

func TestDeleteKeyShowsConfirmForUnprotectedWorktree(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
			Changes: []gitview.FileChange{
				{Path: "README.md", Status: gitview.Modified},
				{Path: "new.txt", Status: gitview.Added},
			},
		},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("delete confirm should not run command yet")
	}
	if got.mode != modeDeleteConfirm {
		t.Fatal("delete key should open confirm dialog")
	}
	view := ansi.Strip(got.View().Content)
	for _, want := range []string{"DELETE", "feature", "/repo/.worktrees/feature", "2 changed files", "remove worktree and delete branch", "[Y]es", "[N]o"} {
		if !strings.Contains(view, want) {
			t.Fatalf("delete confirm view missing %q: %q", want, view)
		}
	}
	if strings.Contains(view, "y/enter yes") {
		t.Fatalf("delete confirm should render option buttons, got %q", view)
	}
}

func TestDeleteKeyBlocksProtectedWorktree(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", Protected: true}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	got := next.(Model)

	if cmd == nil {
		t.Fatal("protected delete should show toast")
	}
	if got.mode == modeDeleteConfirm {
		t.Fatal("protected delete should not open confirm dialog")
	}
	if !strings.Contains(got.toast.Message, "protected") || got.toast.Kind != components.ToastError {
		t.Fatalf("toast = %#v, want protected error", got.toast)
	}
}

func TestMergeKeyBlocksDefaultBranchSource(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	got := next.(Model)

	if cmd == nil {
		t.Fatal("default branch merge should show toast")
	}
	if got.mode == modeMergeTarget {
		t.Fatal("default branch should not open merge target picker")
	}
	if got.toast.Kind != components.ToastInfo || !strings.Contains(got.toast.Message, "default branch") {
		t.Fatalf("toast = %#v, want default branch info", got.toast)
	}
}

func TestMergeKeyOpensTargetListWithDefaultBranchSelected(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/dev", Branch: "dev"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("open merge target picker returned command, want nil")
	}
	if got.mode != modeMergeTarget {
		t.Fatal("m should open merge target picker")
	}
	if got.merge.selectedTargetPath() != "/repo" {
		t.Fatalf("selected merge target = %q, want /repo", got.merge.selectedTargetPath())
	}
	view := ansi.Strip(got.View().Content)
	for _, want := range []string{"Merge into", "main", "dev"} {
		if !strings.Contains(view, want) {
			t.Fatalf("merge target view missing %q: %q", want, view)
		}
	}
}

func TestMergeTargetPickerMarksSelectionAndDoesNotWrapRows(t *testing.T) {
	model := testModel(t)
	model.width = 58
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/source", Branch: "source"}},
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/demo-single-test-file", Branch: "demo/single-test-file", DefaultBranch: true}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	model = next.(Model)
	view := model.merge.picker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())
	lines := strings.Split(view, "\n")
	maxWidth := lipgloss.Width(lines[0])

	if !strings.Contains(ansi.Strip(view), components.IconSelected) {
		t.Fatalf("merge target picker should mark selected target: %q", ansi.Strip(view))
	}
	if !strings.Contains(ansi.Strip(view), "From: source  >  Target: demo/single-tes") {
		t.Fatalf("merge target picker should show source branch: %q", ansi.Strip(view))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > maxWidth {
			t.Fatalf("merge target picker line %d width = %d, want <= %d: %q", i, got, maxWidth, ansi.Strip(line))
		}
	}
}

func TestMergeTargetPickerHeaderFollowsSelectedTarget(t *testing.T) {
	model := testModel(t)
	model.width = 68
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/source", Branch: "source"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/dev", Branch: "dev"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	model = next.(Model)
	if got := ansi.Strip(model.merge.picker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())); !strings.Contains(got, "From: source  >  Target: main") {
		t.Fatalf("initial merge target header = %q, want main target", got)
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	model = next.(Model)
	if got := ansi.Strip(model.merge.picker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())); !strings.Contains(got, "From: source  >  Target: dev") {
		t.Fatalf("updated merge target header = %q, want dev target", got)
	}
}

func TestMergeTargetEnterShowsConfirmationBeforeRunningMerge(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	var gotReq MergeRequest
	model.mergeBranch = func(_ context.Context, req MergeRequest) error {
		gotReq = req
		return nil
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)
	if cmd != nil {
		t.Fatal("merge target enter should show confirmation before returning merge command")
	}
	if got.mode != modeMergeConfirm {
		t.Fatal("merge target enter should open merge confirmation")
	}
	if got.mode == modeMergeTarget {
		t.Fatal("merge confirmation should close target picker")
	}
	view := ansi.Strip(got.View().Content)
	for _, want := range []string{"Merge feature into main", "Target worktree will be updated first.", "Dirty files and conflicts will be checked before merging.", "[Y]es", "[N]o"} {
		if !strings.Contains(view, want) {
			t.Fatalf("merge confirm view missing %q: %q", want, view)
		}
	}

	next, cmd = got.Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	if cmd == nil {
		t.Fatal("merge confirm enter should return merge command")
	}
	next, _ = next.(Model).Update(commandMsgOfType[mergeBranchFinishedMsg](t, cmd))
	got = next.(Model)

	if gotReq.Source.Branch != "feature" || gotReq.Target.Branch != "main" {
		t.Fatalf("merge request = %#v, want feature into main", gotReq)
	}
	if got.mode == modeMergeConfirm || got.mode == modeMergeTarget {
		t.Fatal("successful merge should close merge overlays")
	}
	if got.toast.Kind != components.ToastSuccess || !strings.Contains(got.toast.Message, "merged feature into main") {
		t.Fatalf("toast = %#v, want merge success", got.toast)
	}
}

func TestMergeConfirmDoesNotWrapLongBranchNames(t *testing.T) {
	model := testModel(t)
	model.width = 58
	model.mode = modeMergeConfirm
	model.merge.openConfirm(MergeRequest{
		Source: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: strings.Repeat("feature-", 12)},
		Target: gitview.Worktree{Path: "/repo", Branch: strings.Repeat("main-", 12)},
	})
	title, message := mergeConfirmText(model.merge.request)
	model.confirm.Open(title, message)

	longHeight := lipgloss.Height(model.confirm.Render())
	model.merge.openConfirm(MergeRequest{
		Source: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
		Target: gitview.Worktree{Path: "/repo", Branch: "main"},
	})
	title, message = mergeConfirmText(model.merge.request)
	model.confirm.Open(title, message)
	shortHeight := lipgloss.Height(model.confirm.Render())

	if longHeight != shortHeight {
		t.Fatalf("long merge confirmation height = %d, want %d; overlay should stay one line instead of wrapping", longHeight, shortHeight)
	}
}

func TestMergeConfirmIgnoresHorizontalScrollKeys(t *testing.T) {
	model := testModel(t)
	model.width = 58
	model.mode = modeMergeConfirm
	model.merge.openConfirm(MergeRequest{
		Source: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: strings.Repeat("feature-", 12) + "unique-tail"},
		Target: gitview.Worktree{Path: "/repo", Branch: "main"},
	})
	title, message := mergeConfirmText(model.merge.request)
	model.confirm.Open(title, message)
	before := ansi.Strip(model.confirm.Render())
	if strings.Contains(before, "unique-tail") {
		t.Fatalf("merge confirmation unexpectedly shows truncated tail: %q", before)
	}

	for range 20 {
		next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
		if cmd != nil {
			t.Fatal("merge confirm horizontal key should not return command")
		}
		model = next.(Model)
	}

	after := ansi.Strip(model.confirm.Render())
	if after != before {
		t.Fatalf("horizontal key changed generic confirm view: before=%q after=%q", before, after)
	}
}

func TestMergeConfirmEscCancelsMerge(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	model.mergeBranch = func(context.Context, MergeRequest) error {
		t.Fatal("cancel should not run merge")
		return nil
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	got := next.(Model)

	if cmd != nil {
		t.Fatal("merge confirm cancel should not return command")
	}
	if got.mode == modeMergeConfirm || got.mode == modeMergeTarget || got.merge.request.Source.Path != "" {
		t.Fatalf("merge confirmation state = confirm:%v picker:%v request:%#v, want cleared", got.mode == modeMergeConfirm, got.mode == modeMergeTarget, got.merge.request)
	}
}

func TestMergeTargetEnterIgnoresDuplicateWhileInFlight(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	model.mergeBranch = func(context.Context, MergeRequest) error { return nil }

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	batch := requireBatchCommand(t, cmd, 2)
	requireBatchMsg[spinner.TickMsg](t, batch)
	if _, ok := batchMsgOfType[mergeBranchFinishedMsg](batch); !ok {
		t.Fatal("merge confirm enter should return command")
	}
	next, duplicate := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if duplicate != nil {
		t.Fatal("duplicate merge enter should be ignored while merge is in flight")
	}
	if !got.confirm.IsSubmitting() {
		t.Fatal("merge should remain in flight until command finishes")
	}
	view := ansi.Strip(got.View().Content)
	if !strings.Contains(view, "In progress") {
		t.Fatalf("merge in-flight view should show progress text: %q", view)
	}
	progressLine := ansi.Strip(findRenderedLine(view, "In progress"))
	for _, token := range []string{"[", "]", "="} {
		if strings.Contains(progressLine, token) {
			t.Fatalf("merge in-flight progress line should not show progress bar token %q: %q", token, progressLine)
		}
	}
	if strings.Contains(view, "[Y]es") || strings.Contains(view, "[N]o") {
		t.Fatalf("merge in-flight view should hide confirm buttons: %q", view)
	}
}

func TestPRKeyShowsErrorToastWhenForgeCLIIsMissing(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.findForgeCLI = func() (string, bool) { return "", false }

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	got := next.(Model)

	if cmd == nil {
		t.Fatal("missing Forge CLI should return toast command")
	}
	if got.mode == modePRForm {
		t.Fatal("missing Forge CLI should not open PR form")
	}
	if got.toast.Kind != components.ToastError || !strings.Contains(got.toast.Message, "gh or glab") {
		t.Fatalf("toast = %#v, want gh/glab error", got.toast)
	}
}

func TestPRKeyOpensFormWhenForgeCLIExists(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.findForgeCLI = func() (string, bool) { return "gh", true }

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("PR form open returned command, want nil")
	}
	if got.mode != modePRForm {
		t.Fatal("PR key should open PR form")
	}
	view := ansi.Strip(got.View().Content)
	for _, want := range []string{"Forge CLI: gh", "PR title", "PR description", "<tab> focus", "<c-o> create"} {
		if !strings.Contains(view, want) {
			t.Fatalf("PR form view missing %q: %q", want, view)
		}
	}
}

func TestPRFormInputsUseThemeColors(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		Theme: theme.NewStyles(tm),
		Worktrees: []WorktreeState{{
			Worktree: gitview.Worktree{Path: "/repo", Branch: "feature"},
		}},
	})
	model.focusedPane = paneWorktrees
	model.findForgeCLI = func() (string, bool) { return "gh", true }

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	got := next.(Model)
	background := styleBackgroundToken(got.styles.Panel)
	muted := styleForegroundToken(got.styles.Muted)

	if background == "" || muted == "" {
		t.Fatal("theme tokens should not be empty")
	}
	for name, view := range map[string]string{
		"title": got.pr.TitleView(),
		"body":  got.pr.BodyView(),
	} {
		if !strings.Contains(view, background) {
			t.Fatalf("PR %s input should use panel background %q in %q", name, background, view)
		}
		if !strings.Contains(view, muted) {
			t.Fatalf("PR %s placeholder should use muted foreground %q in %q", name, muted, view)
		}
	}
	for _, line := range strings.Split(got.pr.BodyView(), "\n") {
		if !strings.Contains(line, background) {
			t.Fatalf("PR body line should use panel background %q in %q", background, line)
		}
	}
}

func TestPRFormTabTogglesFocusAndEscCloses(t *testing.T) {
	model := testModel(t)
	model.findForgeCLI = func() (string, bool) { return "glab", true }
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = next.(Model)

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = next.(Model)
	if model.pr.Focus() != components.PRBody {
		t.Fatalf("pr focus = %v, want body", model.pr.Focus())
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	model = next.(Model)
	if model.mode == modePRForm {
		t.Fatal("esc should close PR form")
	}
}

func TestPRFormSubmitRequiresTitle(t *testing.T) {
	model := testModel(t)
	model.findForgeCLI = func() (string, bool) { return "gh", true }
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = next.(Model)

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
	got := next.(Model)

	if cmd == nil {
		t.Fatal("empty PR title should return toast command")
	}
	if got.mode != modePRForm {
		t.Fatal("empty PR title should keep PR form open")
	}
	if got.toast.Kind != components.ToastError || !strings.Contains(got.toast.Message, "PR title is required") {
		t.Fatalf("toast = %#v, want PR title required", got.toast)
	}
}

func TestPRFormSubmitCreatesPullRequest(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	model.findForgeCLI = func() (string, bool) { return "gh", true }
	var gotReq PullRequestRequest
	model.createPullRequest = func(_ context.Context, req PullRequestRequest) error {
		gotReq = req
		return nil
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = next.(Model)
	model.pr.SetTitle("Add PR creator")
	model.pr.SetBody("Creates a pull request from the selected worktree.")
	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("PR submit should return create command")
	}
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if gotReq.CLI != "gh" || gotReq.WorktreeDir != "/repo/.worktrees/feature" || gotReq.Branch != "feature" {
		t.Fatalf("request target = %#v, want gh feature worktree", gotReq)
	}
	if gotReq.Title != "Add PR creator" || gotReq.Body != "Creates a pull request from the selected worktree." {
		t.Fatalf("request message = %#v, want PR title/body", gotReq)
	}
	if got.mode == modePRForm {
		t.Fatal("successful PR create should close form")
	}
	if got.toast.Kind != components.ToastSuccess || got.toast.Message != "PR/MR created" {
		t.Fatalf("toast = %#v, want PR/MR created success", got.toast)
	}
}

func TestPRFormSubmitIgnoresDuplicateWhileInFlight(t *testing.T) {
	model := testModel(t)
	model.findForgeCLI = func() (string, bool) { return "gh", true }
	model.createPullRequest = func(context.Context, PullRequestRequest) error { return nil }

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = next.(Model)
	model.pr.SetTitle("Add PR creator")
	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("first PR submit should return command")
	}
	next, duplicate := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
	got := next.(Model)

	if duplicate != nil {
		t.Fatal("duplicate PR submit should be ignored while create is in flight")
	}
	if !got.pr.IsSubmitting() {
		t.Fatal("PR should remain submitting until command finishes")
	}
}

func TestPRFormSubmitFailureKeepsFormOpen(t *testing.T) {
	model := testModel(t)
	model.findForgeCLI = func() (string, bool) { return "glab", true }
	model.createPullRequest = func(context.Context, PullRequestRequest) error {
		return errors.New("not authenticated")
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = next.(Model)
	model.pr.SetTitle("Add MR creator")
	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("PR submit should return create command")
	}
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if got.mode != modePRForm {
		t.Fatal("failed PR create should keep form open")
	}
	if got.toast.Kind != components.ToastError || !strings.Contains(got.toast.Message, "not authenticated") {
		t.Fatalf("toast = %#v, want auth error", got.toast)
	}
	if got.pr.IsSubmitting() {
		t.Fatal("failed PR create should clear submitting state")
	}
}

func TestConfirmDeleteRunsDeleteCallback(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	var deleted gitview.Worktree
	model.deleteWorktree = func(_ context.Context, worktree gitview.Worktree) error {
		deleted = worktree
		return nil
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	if cmd == nil {
		t.Fatal("confirm delete should return command")
	}
	msg := commandMsgOfType[deleteWorktreeFinishedMsg](t, cmd)
	next, _ = next.(Model).Update(msg)
	got := next.(Model)

	if deleted.Branch != "feature" {
		t.Fatalf("deleted branch = %q, want feature", deleted.Branch)
	}
	if got.mode == modeDeleteConfirm {
		t.Fatal("confirm dialog should close after delete")
	}
	if !strings.Contains(got.toast.Message, "deleted feature") || got.toast.Kind != components.ToastSuccess {
		t.Fatalf("toast = %#v, want deleted feature success", got.toast)
	}
}

func TestConfirmDeleteIgnoresDuplicateWhileInFlight(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	model.deleteWorktree = func(context.Context, gitview.Worktree) error { return nil }

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	batch := requireBatchCommand(t, cmd, 2)
	requireBatchMsg[spinner.TickMsg](t, batch)
	if _, ok := batchMsgOfType[deleteWorktreeFinishedMsg](batch); !ok {
		t.Fatal("first confirm delete should return command")
	}
	next, duplicate := next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	got := next.(Model)

	if duplicate != nil {
		t.Fatal("duplicate confirm delete should be ignored while delete is in flight")
	}
	if !got.confirm.IsSubmitting() {
		t.Fatal("delete should remain in flight until command finishes")
	}
	view := ansi.Strip(got.View().Content)
	if !strings.Contains(view, "In progress") {
		t.Fatalf("delete in-flight view should show progress text: %q", view)
	}
	progressLine := ansi.Strip(findRenderedLine(view, "In progress"))
	for _, token := range []string{"[", "]", "="} {
		if strings.Contains(progressLine, token) {
			t.Fatalf("delete in-flight progress line should not show progress bar token %q: %q", token, progressLine)
		}
	}
	if strings.Contains(view, "[Y]es") || strings.Contains(view, "[N]o") {
		t.Fatalf("delete in-flight view should hide confirm buttons: %q", view)
	}
}

func TestTabSwitchesWorktree(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{
			Worktree: gitview.Worktree{Path: "/repo", Branch: "main", Current: true},
			Changes:  []gitview.FileChange{{Path: "main.go", Status: gitview.Modified}},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
			Changes:  []gitview.FileChange{{Path: "feature.go", Status: gitview.Added}},
		},
	}
	model.selectedWorktree = 0
	model.changes = model.worktrees[0].Changes
	model.refreshDiff()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(Model)

	if got.selectedWorktreeValue().Branch != "feature" || got.selectedFileValue().Path != "feature.go" {
		t.Fatalf("selected worktree/file = %q/%q, want feature/feature.go", got.selectedWorktreeValue().Branch, got.selectedFileValue().Path)
	}
	if !strings.Contains(got.View().Content, "● [1]-") {
		t.Fatalf("worktree panel should be focused after tab: %q", got.View().Content)
	}
}

func TestNumberKeysFocusPanels(t *testing.T) {
	model := testModel(t)

	for _, tc := range []struct {
		key  string
		want string
	}{
		{key: "1", want: "● [1]-"},
		{key: "2", want: "● [2]-"},
		{key: "3", want: "● [3]-"},
	} {
		next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: tc.key, Code: rune(tc.key[0])}))
		model = next.(Model)
		if !strings.Contains(model.View().Content, tc.want) {
			t.Fatalf("key %s should focus panel %q: %q", tc.key, tc.want, model.View().Content)
		}
	}
}

func TestEnterOnFilePanelFocusesDiffPanel(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneFiles

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if got.focusedPane != paneDiff {
		t.Fatalf("focusedPane = %v, want paneDiff", got.focusedPane)
	}
	if !strings.Contains(got.View().Content, "● [3]-") {
		t.Fatalf("enter should focus diff panel: %q", got.View().Content)
	}
}

func TestEnterOnWorktreePanelFocusesFilePanelWhenFilesExist(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if got.focusedPane != paneFiles {
		t.Fatalf("focusedPane = %v, want paneFiles", got.focusedPane)
	}
	if !strings.Contains(got.View().Content, "● [2]-") {
		t.Fatalf("enter should focus files panel: %q", got.View().Content)
	}
}

func TestEnterOnWorktreePanelStaysWhenNoFilesExist(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.changes = nil
	model.worktrees[model.selectedWorktree].Changes = nil
	model.refreshDiff()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if got.focusedPane != paneWorktrees {
		t.Fatalf("focusedPane = %v, want paneWorktrees", got.focusedPane)
	}
}

func TestEditKeyOpensSelectedFileCommand(t *testing.T) {
	model := testModel(t)
	t.Setenv("EDITOR", "true")

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))

	if cmd == nil {
		t.Fatal("e should return editor command")
	}
}

func TestEditorTargetLineUsesFirstChangedNewLine(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"@@ -10,5 +10,6 @@",
		" unchanged",
		"-old",
		"+new",
		" context",
		"@@ -40,3 +41,4 @@",
		" context",
		"+latest",
	}, "\n")

	if got := editorTargetLine(diff); got != 11 {
		t.Fatalf("editorTargetLine() = %d, want 11", got)
	}
}

func TestEditorTargetLineUsesDeletionPositionWhenNoAddedLines(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"@@ -10,3 +10,2 @@",
		" unchanged",
		"-removed",
		" context",
	}, "\n")

	if got := editorTargetLine(diff); got != 11 {
		t.Fatalf("editorTargetLine() = %d, want 11", got)
	}
}

func TestEditKeyShowsToastWhenNoFileSelected(t *testing.T) {
	model := testModel(t)
	model.changes = nil
	model.worktrees[model.selectedWorktree].Changes = nil
	model.refreshDiff()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	got := next.(Model)

	if cmd == nil {
		t.Fatal("e without selected file should return toast command")
	}
	if got.toast.Message != "no file selected" || got.toast.Kind != components.ToastInfo {
		t.Fatalf("toast = %#v, want info no file selected", got.toast)
	}
}

func TestEditorFailureShowsToast(t *testing.T) {
	model := testModel(t)

	next, cmd := model.Update(editorFinishedMsg{err: errors.New("boom")})
	got := next.(Model)

	if cmd == nil {
		t.Fatal("editor failure should return toast command")
	}
	if !strings.Contains(got.toast.Message, "editor failed: boom") || got.toast.Kind != components.ToastError {
		t.Fatalf("toast = %#v, want editor failure error", got.toast)
	}
}

func TestEscMovesFocusBackWithoutQuitting(t *testing.T) {
	model := testModel(t)

	model.focusedPane = paneDiff
	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	model = next.(Model)
	if cmd != nil {
		t.Fatalf("esc from diff returned command, want nil")
	}
	if model.focusedPane != paneFiles {
		t.Fatalf("esc from diff focusedPane = %v, want paneFiles", model.focusedPane)
	}

	next, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	model = next.(Model)
	if cmd != nil {
		t.Fatalf("esc from files returned command, want nil")
	}
	if model.focusedPane != paneWorktrees {
		t.Fatalf("esc from files focusedPane = %v, want paneWorktrees", model.focusedPane)
	}

	next, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	model = next.(Model)
	if cmd != nil {
		t.Fatalf("esc from worktrees returned command, want nil")
	}
	if model.focusedPane != paneWorktrees {
		t.Fatalf("esc from worktrees focusedPane = %v, want paneWorktrees", model.focusedPane)
	}
}

func TestMouseClickFocusesDiffPanel(t *testing.T) {
	model := testModel(t)
	leftWidth, _ := model.layoutWidths()

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: leftWidth + 2, Y: 5}))
	got := next.(Model)

	if !strings.Contains(got.View().Content, "● [3]-") {
		t.Fatalf("diff panel should be focused after diff click: %q", got.View().Content)
	}
}

func TestMouseWheelScrollsDiffPanel(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 10
	model.refreshDiff()
	lines := make([]string, 0, 40)
	for i := range 40 {
		lines = append(lines, fmt.Sprintf(" line-%02d", i))
	}
	model.setDiffContent(strings.Join(lines, "\n"))
	leftWidth, _ := model.layoutWidths()

	next, _ := model.Update(tea.MouseWheelMsg{X: leftWidth + 2, Y: 5, Button: tea.MouseWheelDown})
	got := next.(Model)

	if got.focusedPane != paneDiff {
		t.Fatalf("wheel over diff focusedPane = %v, want paneDiff", got.focusedPane)
	}
	if got.diff.YOffset() == 0 {
		t.Fatal("wheel down over diff should scroll viewport")
	}

	next, _ = got.Update(tea.MouseWheelMsg{X: leftWidth + 2, Y: 5, Button: tea.MouseWheelUp})
	got = next.(Model)
	if got.diff.YOffset() != 0 {
		t.Fatalf("wheel up over diff y offset = %d, want 0", got.diff.YOffset())
	}
}

func TestMouseWheelDoesNotScrollDiffBehindOverlays(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Model)
	}{
		{name: "file filter", setup: func(m *Model) { m.mode = modeFileFilter }},
		{name: "overlap picker", setup: func(m *Model) { m.mode = modeOverlapPicker }},
		{name: "pull request form", setup: func(m *Model) { m.mode = modePRForm }},
		{name: "delete confirm", setup: func(m *Model) { m.mode = modeDeleteConfirm }},
		{name: "merge confirm", setup: func(m *Model) { m.mode = modeMergeConfirm }},
		{name: "merge target picker", setup: func(m *Model) { m.mode = modeMergeTarget }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := testModel(t)
			model.width = 100
			model.height = 10
			model.refreshDiff()
			lines := make([]string, 0, 40)
			for i := range 40 {
				lines = append(lines, fmt.Sprintf(" line-%02d", i))
			}
			model.setDiffContent(strings.Join(lines, "\n"))
			model.focusedPane = paneFiles
			tc.setup(&model)
			leftWidth, _ := model.layoutWidths()

			next, _ := model.Update(tea.MouseWheelMsg{X: leftWidth + 2, Y: 5, Button: tea.MouseWheelDown})
			got := next.(Model)

			if got.focusedPane != paneFiles {
				t.Fatalf("wheel behind %s focusedPane = %v, want paneFiles", tc.name, got.focusedPane)
			}
			if got.diff.YOffset() != 0 {
				t.Fatalf("wheel behind %s y offset = %d, want 0", tc.name, got.diff.YOffset())
			}
		})
	}
}

func TestMouseWheelScrollsOverlapCompareWithoutScrollingDiff(t *testing.T) {
	model := testModel(t)
	model.width = 120
	model.height = 10
	model.refreshDiff()
	lines := make([]string, 0, 40)
	for i := range 40 {
		lines = append(lines, fmt.Sprintf("+compare-%02d", i))
	}
	model.overlap.compareDiff = strings.Join(lines, "\n")
	model.mode = modeOverlapCompare

	next, _ := model.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown})
	got := next.(Model)

	if got.overlap.compareYOffset == 0 {
		t.Fatal("wheel down should scroll overlap compare")
	}
	if got.diff.YOffset() != 0 {
		t.Fatalf("wheel over compare scrolled diff y offset = %d, want 0", got.diff.YOffset())
	}

	next, _ = got.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelUp})
	got = next.(Model)
	if got.overlap.compareYOffset != 0 {
		t.Fatalf("wheel up compare y offset = %d, want 0", got.overlap.compareYOffset)
	}
}

func TestSidebarPanelsFillBodyHeight(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 30
	leftWidth, _ := model.layoutWidths()
	contentHeight := model.bodyHeight()
	worktrees := model.render(model.styles, model.panel, model.focusedPane == paneWorktrees, leftWidth, model.worktreePaneHeight(contentHeight))
	files := model.fileListComponent().Render(model.styles, model.panel, model.focusedPane == paneFiles, leftWidth, max(4, contentHeight-lipgloss.Height(worktrees)), model.fileBadge(), model.visibleOverlapCount())
	sidebar := lipgloss.JoinVertical(lipgloss.Left, worktrees, files)

	if got := lipgloss.Height(sidebar); got != contentHeight {
		t.Fatalf("sidebar height = %d, want %d", got, contentHeight)
	}
}

func TestInitSchedulesAutoRefresh(t *testing.T) {
	model := testModel(t)

	if cmd := model.Init(); cmd == nil {
		t.Fatal("Init() did not schedule auto-refresh")
	}
}

func TestAutoRefreshReloadsChanges(t *testing.T) {
	model := testModel(t)
	model.reload = func(context.Context, string) Snapshot {
		return Snapshot{
			Changes: []gitview.FileChange{{Path: "fresh.go", Status: gitview.Added}},
			Diffs:   map[string]string{"fresh.go": "diff --git a/fresh.go b/fresh.go\n+fresh"},
		}
	}

	next, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("auto-refresh command is nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("auto-refresh command = %#v, want batch", cmd())
	}
	msg := batch[0]()
	next, _ = next.(Model).Update(msg)
	got := next.(Model)

	if got.selectedFileValue().Path != "fresh.go" {
		t.Fatalf("Selected() = %q, want fresh.go", got.selectedFileValue().Path)
	}
	if !strings.Contains(got.View().Content, "fresh") {
		t.Fatalf("View() missing refreshed diff: %q", got.View().Content)
	}
}

func TestAutoRefreshSkipsReloadWhilePreviousReloadIsInFlight(t *testing.T) {
	model := testModel(t)
	model.reload = func(context.Context, string) Snapshot {
		return Snapshot{
			Changes: []gitview.FileChange{{Path: "fresh.go", Status: gitview.Added}},
			Diffs:   map[string]string{"fresh.go": "diff --git a/fresh.go b/fresh.go\n+fresh"},
		}
	}

	next, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("first auto-refresh command is nil")
	}
	model = next.(Model)
	if model.refreshGeneration != 1 {
		t.Fatalf("refreshGeneration after first refresh = %d, want 1", model.refreshGeneration)
	}

	next, cmd = model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("second auto-refresh should still schedule the next tick")
	}
	model = next.(Model)
	if model.refreshGeneration != 1 {
		t.Fatalf("refreshGeneration after skipped refresh = %d, want 1", model.refreshGeneration)
	}
}

func TestAutoRefreshPreservesMergeTargetPicker(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/dev", Branch: "dev"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	model = next.(Model)
	if model.mode != modeMergeTarget {
		t.Fatal("merge target picker should be open before refresh")
	}
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	model = next.(Model)

	model.applySnapshot(Snapshot{
		Worktrees: []WorktreeState{
			{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
			{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
			{Worktree: gitview.Worktree{Path: "/repo/.worktrees/dev", Branch: "dev"}},
		},
		SelectedWorktree: model.selectedWorktree,
		Changes:          model.changes,
		Diffs:            model.diffs,
	})

	if model.mode != modeMergeTarget {
		t.Fatal("refresh should preserve merge target picker")
	}
	if model.merge.source.Branch != "feature" {
		t.Fatalf("mergeSource = %#v, want refreshed feature source", model.merge.source)
	}
	if got := model.merge.selectedTargetPath(); got != "/repo/.worktrees/dev" {
		t.Fatalf("selected merge target = %q, want dev", got)
	}
}

func TestAutoRefreshClosesMergeTargetPickerWhenSourceDisappears(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	model = next.(Model)
	if model.mode != modeMergeTarget {
		t.Fatal("merge target picker should be open before refresh")
	}

	model.applySnapshot(Snapshot{
		Worktrees: []WorktreeState{
			{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
		},
		SelectedWorktree: 0,
		Changes:          model.changes,
		Diffs:            model.diffs,
	})

	if model.mode == modeMergeTarget {
		t.Fatal("refresh should close merge target picker when source disappears")
	}
	if model.merge.source.Path != "" {
		t.Fatalf("mergeSource = %#v, want cleared", model.merge.source)
	}
}

func TestAutoRefreshPreservesMergeConfirmation(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	model = next.(Model)
	if !model.confirm.Submit() {
		t.Fatal("merge confirmation should enter submitting state")
	}
	if model.mode != modeMergeConfirm {
		t.Fatal("merge confirmation should be open before refresh")
	}
	tick := model.confirm.Tick()

	model.applySnapshot(Snapshot{
		Worktrees: []WorktreeState{
			{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
			{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
		},
		SelectedWorktree: model.selectedWorktree,
		Changes:          model.changes,
		Diffs:            model.diffs,
	})

	if model.mode != modeMergeConfirm {
		t.Fatal("refresh should preserve merge confirmation")
	}
	if !model.confirm.IsSubmitting() {
		t.Fatal("refresh should preserve in-flight merge state")
	}
	if model.mode == modeMergeTarget {
		t.Fatal("merge confirmation should keep target picker closed")
	}
	if model.merge.request.Source.Branch != "feature" || model.merge.request.Target.Branch != "main" {
		t.Fatalf("merge request = %#v, want feature into main", model.merge.request)
	}
	if _, cmd := model.Update(tick); cmd == nil {
		t.Fatal("refresh should preserve pending merge spinner ticks")
	}
}

func TestWindowResizePreservesPendingConfirmSpinnerTick(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	model = next.(Model)
	if !model.confirm.Submit() {
		t.Fatal("delete confirmation should enter submitting state")
	}
	tick := model.confirm.Tick()

	next, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = next.(Model)

	if _, cmd := model.Update(tick); cmd == nil {
		t.Fatal("resize should preserve pending confirm spinner ticks")
	}
}

func TestAutoRefreshPreservesSelectedFileWhenNewFileIsAdded(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Modified},
	}
	model.worktrees[model.selectedWorktree].Changes = model.changes
	model.selected = 1
	model.refreshDiff()
	model.reload = func(context.Context, string) Snapshot {
		changes := []gitview.FileChange{
			{Path: "new.go", Status: gitview.Added},
			{Path: "a.go", Status: gitview.Modified},
			{Path: "b.go", Status: gitview.Modified},
		}
		return Snapshot{
			Worktrees: []WorktreeState{{
				Worktree: model.selectedWorktreeValue(),
				Changes:  changes,
			}},
			SelectedWorktree: 0,
			Changes:          changes,
			Diffs: map[string]string{
				model.selectedWorktreeValue().Path + "\x00" + "b.go": "diff --git a/b.go b/b.go\n+b",
			},
		}
	}

	next, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("auto-refresh command is nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("auto-refresh command = %#v, want batch", cmd())
	}
	next, _ = next.(Model).Update(batch[0]())
	got := next.(Model)

	if got.selectedFileValue().Path != "b.go" {
		t.Fatalf("Selected() = %q, want b.go", got.selectedFileValue().Path)
	}
	if !strings.Contains(got.View().Content, "+b") {
		t.Fatalf("View() should keep selected file diff: %q", got.View().Content)
	}
}

func TestAutoRefreshSkipsUnchangedSnapshot(t *testing.T) {
	model := testModel(t)
	model.height = 8
	model.changes = []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}}
	model.worktrees[model.selectedWorktree].Changes = model.changes
	model.diffs = map[string]string{"a.go": strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"@@ -1,8 +1,8 @@",
		" line-1",
		" line-2",
		" line-3",
		" line-4",
		" line-5",
	}, "\n")}
	model.refreshDiff()
	model.diff.SetYOffset(3)
	yOffset := model.diff.YOffset()
	model.refreshGeneration = 7
	model.refreshInFlight = true
	revision := model.revision

	next, cmd := model.Update(reloadMsg{
		generation: model.refreshGeneration,
		snapshot: Snapshot{
			Worktrees:        model.worktrees,
			SelectedWorktree: model.selectedWorktree,
			Changes:          model.changes,
			Diffs:            model.diffs,
			Error:            model.err,
		},
	})
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("unchanged snapshot returned command, want nil")
	}
	if got.revision != revision {
		t.Fatalf("unchanged snapshot revision = %d, want %d", got.revision, revision)
	}
	if got.refreshInFlight {
		t.Fatal("unchanged snapshot should clear refreshInFlight")
	}
	if got.diff.YOffset() != yOffset {
		t.Fatalf("unchanged snapshot y offset = %d, want %d", got.diff.YOffset(), yOffset)
	}
}

func TestAutoRefreshPreservesSelectedDiffWhenLineStatsAreUnchanged(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{{Path: "a.go", Status: gitview.Modified, Additions: 1}}
	model.worktrees[model.selectedWorktree].Changes = model.changes
	model.diffs = map[string]string{"a.go": "diff --git a/a.go b/a.go\n+old"}
	model.refreshDiff()
	loads := 0
	model.loadDiff = func(context.Context, string, gitview.FileChange) string {
		loads++
		return "diff --git a/a.go b/a.go\n+fresh"
	}
	model.reload = func(context.Context, string) Snapshot {
		return Snapshot{
			Worktrees: []WorktreeState{{
				Worktree: model.selectedWorktreeValue(),
				Changes:  model.changes,
			}},
			SelectedWorktree: 0,
			Changes:          model.changes,
		}
	}

	next, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("auto-refresh command is nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("auto-refresh command = %#v, want batch", cmd())
	}
	next, cmd = next.(Model).Update(batch[0]())
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("unchanged line stats returned diff reload command")
	}
	if loads != 0 {
		t.Fatalf("loadDiff calls = %d, want 0", loads)
	}
	if !strings.Contains(got.View().Content, "+old") {
		t.Fatalf("unchanged line stats should keep existing diff visible: %q", got.View().Content)
	}
}

func TestAutoRefreshReloadsSelectedDiffWhenFingerprintChanges(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{{Path: "a.go", Status: gitview.Modified, Additions: 1, Fingerprint: "old"}}
	model.worktrees[model.selectedWorktree].Changes = model.changes
	model.diffs = map[string]string{"a.go": "diff --git a/a.go b/a.go\n+old"}
	model.refreshDiff()
	loads := 0
	model.loadDiff = func(context.Context, string, gitview.FileChange) string {
		loads++
		return "diff --git a/a.go b/a.go\n+fresh"
	}
	freshChanges := []gitview.FileChange{{Path: "a.go", Status: gitview.Modified, Additions: 1, Fingerprint: "fresh"}}
	model.reload = func(context.Context, string) Snapshot {
		return Snapshot{
			Worktrees: []WorktreeState{{
				Worktree: model.selectedWorktreeValue(),
				Changes:  freshChanges,
			}},
			SelectedWorktree: 0,
			Changes:          freshChanges,
		}
	}

	next, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("auto-refresh command is nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("auto-refresh command = %#v, want batch", cmd())
	}
	next, cmd = next.(Model).Update(batch[0]())
	if cmd == nil {
		t.Fatal("fingerprint change should force selected diff reload")
	}
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if loads != 1 {
		t.Fatalf("loadDiff calls = %d, want 1", loads)
	}
	if !strings.Contains(got.View().Content, "+fresh") {
		t.Fatalf("fingerprint change should refresh selected diff: %q", got.View().Content)
	}
}

func TestAutoRefreshReloadsSelectedDiffWhenLineStatsChange(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{{Path: "a.go", Status: gitview.Modified, Additions: 1}}
	model.worktrees[model.selectedWorktree].Changes = model.changes
	model.diffs = map[string]string{"a.go": "diff --git a/a.go b/a.go\n+old"}
	model.refreshDiff()
	model.loadDiff = func(context.Context, string, gitview.FileChange) string {
		return "diff --git a/a.go b/a.go\n+fresh"
	}
	freshChanges := []gitview.FileChange{{Path: "a.go", Status: gitview.Modified, Additions: 2}}
	model.reload = func(context.Context, string) Snapshot {
		return Snapshot{
			Worktrees: []WorktreeState{{
				Worktree: model.selectedWorktreeValue(),
				Changes:  freshChanges,
			}},
			SelectedWorktree: 0,
			Changes:          freshChanges,
		}
	}

	next, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("auto-refresh command is nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("auto-refresh command = %#v, want batch", cmd())
	}
	next, cmd = next.(Model).Update(batch[0]())
	if cmd == nil {
		t.Fatal("changed line stats should force selected diff reload")
	}
	if view := next.(Model).View().Content; !strings.Contains(view, "+old") {
		t.Fatalf("changed line stats should keep old diff visible until reload finishes: %q", view)
	}
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if !strings.Contains(got.View().Content, "+fresh") {
		t.Fatalf("changed line stats should refresh selected diff: %q", got.View().Content)
	}
}

func TestAutoRefreshPreservesDiffScrollForSelectedFile(t *testing.T) {
	model := testModel(t)
	model.height = 8
	diff := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"@@ -1,12 +1,12 @@",
		" line-1",
		" line-2",
		" line-3",
		" line-4",
		" line-5",
		" line-6",
		" line-7",
		" line-8",
		" line-9",
		" line-10",
	}, "\n")
	model.diffs = map[string]string{"a.go": diff}
	model.refreshDiff()
	model.diff.SetYOffset(5)
	model.reload = func(context.Context, string) Snapshot {
		return Snapshot{
			Changes: []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}},
			Diffs:   map[string]string{"a.go": diff + "\n line-11"},
		}
	}

	next, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("auto-refresh command is nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("auto-refresh command = %#v, want batch", cmd())
	}
	next, _ = next.(Model).Update(batch[0]())
	got := next.(Model)

	if got.diff.YOffset() != 5 {
		t.Fatalf("diff y offset after refresh = %d, want 5", got.diff.YOffset())
	}
}

func TestAutoRefreshPreservesDiffScrollWhenSelectedDiffReloadsLazily(t *testing.T) {
	model := testModel(t)
	model.height = 8
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Modified},
	}
	selectedDiff := strings.Join([]string{
		"diff --git a/b.go b/b.go",
		"@@ -1,12 +1,12 @@",
		" line-1",
		" line-2",
		" line-3",
		" line-4",
		" line-5",
		" line-6",
		" line-7",
		" line-8",
		" line-9",
		" line-10",
	}, "\n")
	model.diffs = map[string]string{"b.go": selectedDiff}
	model.selected = 1
	model.refreshDiff()
	model.diff.SetYOffset(5)
	model.reload = func(context.Context, string) Snapshot {
		return Snapshot{
			Changes: model.changes,
			Diffs:   map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"},
		}
	}
	model.loadDiff = func(_ context.Context, _ string, change gitview.FileChange) string {
		return selectedDiff + "\n refreshed-" + change.Path
	}

	next, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("auto-refresh command is nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("auto-refresh command = %#v, want batch", cmd())
	}
	next, cmd = next.(Model).Update(batch[0]())
	if cmd == nil {
		t.Fatal("selected diff should be loaded lazily after refresh")
	}
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if got.diff.YOffset() != 5 {
		t.Fatalf("diff y offset after lazy model.reload = %d, want 5", got.diff.YOffset())
	}
	if !strings.Contains(got.diffs[got.diffKey(got.selectedFileValue())], "refreshed-b.go") {
		t.Fatalf("selected diff was not lazily reloaded: %#v", got.diffs)
	}
}

func TestAutoRefreshUsesConfiguredContext(t *testing.T) {
	model := testModel(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	model.context = ctx
	model.reload = func(ctx context.Context, _ string) Snapshot {
		if ctx.Err() == nil {
			t.Fatal("reload context was not canceled")
		}
		return Snapshot{}
	}

	_, cmd := model.Update(autoRefreshMsg{})
	if cmd == nil {
		t.Fatal("auto-refresh command is nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("auto-refresh command = %#v, want batch", cmd())
	}
	_ = batch[0]()
}

func TestMouseClickSelectsFile(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Added},
	}
	model.refreshDiff()

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: 2, Y: 7}))
	got := next.(Model).selectedFileValue()

	if got.Path != "b.go" {
		t.Fatalf("Selected() = %q, want b.go", got.Path)
	}
}

func TestMouseClickSelectsFirstWorktree(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{
			Worktree: gitview.Worktree{Path: "/repo", Branch: "main", Current: true},
			Changes:  []gitview.FileChange{{Path: "main.go", Status: gitview.Modified}},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
			Changes:  []gitview.FileChange{{Path: "feature.go", Status: gitview.Added}},
		},
	}
	model.selectedWorktree = 1
	model.normalizeWorktrees()

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: 2, Y: 1}))
	got := next.(Model)

	if got.selectedWorktree != 0 {
		t.Fatalf("selectedWorktree = %d, want 0", got.selectedWorktree)
	}
	if got.selectedWorktreeValue().Branch != "main" {
		t.Fatalf("selectedWorktreeValue() = %q, want main", got.selectedWorktreeValue().Branch)
	}
}

func TestMouseClickLoadsMissingDiffLazily(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Modified},
	}
	model.diffs = map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"}
	model.loadDiff = func(_ context.Context, _ string, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n+b"
	}
	model.refreshDiff()

	next, cmd := model.Update(tea.MouseClickMsg(tea.Mouse{X: 2, Y: 7}))
	if cmd == nil {
		t.Fatal("mouse selection did not request lazy diff load")
	}
	msg := cmd()
	next, _ = next.(Model).Update(msg)
	got := next.(Model)

	if !strings.Contains(got.View().Content, "+b") {
		t.Fatalf("View() missing lazily loaded mouse diff: %q", got.View().Content)
	}
}

func TestMouseClickSelectsTheme(t *testing.T) {
	model := testModel(t)
	model.themePicker.Names = []string{"tokyonight-night", "gruvbox-dark"}
	model.openThemePicker()
	overlay := model.themePicker.Render(model.width, model.bodyHeight(), model.styles, model.overlayPanelStyle())
	x, y := components.OverlayPosition(overlay, model.width, model.bodyHeight())

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: x + 2, Y: y + 4}))
	got := next.(Model)

	if got.themePicker.Name != "gruvbox-dark" {
		t.Fatalf("themeName = %q, want gruvbox-dark", got.themePicker.Name)
	}
	if got.mode == modeThemePicker {
		t.Fatal("theme picker still open after mouse selection")
	}
}

func TestFileListWindowsLargeChangeSets(t *testing.T) {
	model := testModel(t)
	model.width = 90
	model.height = 9
	model.changes = make([]gitview.FileChange, 20)
	model.diffs = map[string]string{}
	for i := range model.changes {
		model.changes[i] = gitview.FileChange{Path: "file-" + string(rune('a'+i)) + ".go", Status: gitview.Modified}
	}
	model.refreshDiff()

	firstView := model.View().Content
	if strings.Contains(firstView, "file-t.go") {
		t.Fatalf("initial view included offscreen file: %q", firstView)
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "G", Code: 'G'}))
	lastView := next.(Model).View().Content
	if !strings.Contains(lastView, "file-t.go") || strings.Contains(lastView, "file-a.go") {
		t.Fatalf("last view did not window around selected file: %q", lastView)
	}
}

func TestFileFilterShowsOverlayAndFilteredFileTitle(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "internal/tui/model.go", Status: gitview.Modified, Additions: 4, Deletions: 1},
		{Path: "internal/tui/model_test.go", Status: gitview.Modified, Additions: 12},
		{Path: "internal/git/repository.go", Status: gitview.Modified, Additions: 2},
	}
	model.diffs = map[string]string{
		model.selectedWorktreeValue().Path + "\x00" + "internal/tui/model.go": "diff --git a/internal/tui/model.go b/internal/tui/model.go\n+model",
	}
	model.refreshDiff()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	model = next.(Model)
	for _, key := range []rune{'m', 'o', 'd', 'e', 'l'} {
		next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: string(key), Code: key}))
		model = next.(Model)
	}
	view := ansi.Strip(model.View().Content)

	if !strings.Contains(view, "Filters") || !strings.Contains(view, "model") {
		t.Fatalf("filtered view missing overlay query: %q", view)
	}
	if !strings.Contains(view, "2 filtered [Esc]") {
		t.Fatalf("filtered title missing filtered state: %q", view)
	}
	if strings.Contains(view, "2 files /model") {
		t.Fatalf("filtered title should not render raw query: %q", view)
	}
	if got := len(model.visibleChanges()); got != 2 {
		t.Fatalf("visible filtered changes = %d, want 2", got)
	}
	if got := model.selectedFileValue().Path; got != "internal/tui/model.go" {
		t.Fatalf("selected filtered file = %q, want internal/tui/model.go", got)
	}
	if strings.Contains(view, "internal/git/repository.go") {
		t.Fatalf("filtered view included non-match: %q", view)
	}
}

func TestFileFilterEnterKeepsFilterAndClosesOverlay(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "internal/tui/model.go", Status: gitview.Modified},
		{Path: "internal/git/repository.go", Status: gitview.Modified},
	}
	model.diffs = map[string]string{
		model.selectedWorktreeValue().Path + "\x00" + "internal/tui/model.go": "diff --git a/internal/tui/model.go b/internal/tui/model.go\n+model",
	}
	model.refreshDiff()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	model = next.(Model)
	for _, key := range []rune{'m', 'o', 'd', 'e', 'l'} {
		next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: string(key), Code: key}))
		model = next.(Model)
	}
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	model = next.(Model)
	view := ansi.Strip(model.View().Content)

	if strings.Contains(view, "Filters") || strings.Contains(view, "1 files /model") {
		t.Fatalf("enter should close filter overlay and hide query: %q", view)
	}
	if !strings.Contains(view, "1 filtered [Esc]") || !strings.Contains(view, "internal/tui/model.go") {
		t.Fatalf("enter should keep active filtered list: %q", view)
	}
	if strings.Contains(view, "internal/git/repository.go") {
		t.Fatalf("filter should remain active after enter: %q", view)
	}
}

func TestFileFilterEscClearsFilterAndPreservesSelection(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "README.md", Status: gitview.Modified},
		{Path: "internal/tui/model.go", Status: gitview.Modified},
		{Path: "internal/tui/model_test.go", Status: gitview.Modified},
	}
	model.selected = 1
	model.diffs = map[string]string{
		model.selectedWorktreeValue().Path + "\x00" + "internal/tui/model.go": "diff --git a/internal/tui/model.go b/internal/tui/model.go\n+model",
	}
	model.refreshDiff()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	model = next.(Model)
	for _, key := range []rune{'m', 'o', 'd', 'e', 'l'} {
		next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: string(key), Code: key}))
		model = next.(Model)
	}
	if got := model.selectedFileValue().Path; got != "internal/tui/model.go" {
		t.Fatalf("filtered selected path = %q, want internal/tui/model.go", got)
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	model = next.(Model)
	view := ansi.Strip(model.View().Content)

	if strings.Contains(view, "filtered") || strings.Contains(view, "Filters") {
		t.Fatalf("esc did not clear filter: %q", view)
	}
	if !strings.Contains(view, "3 files") || !strings.Contains(view, "README.md") {
		t.Fatalf("esc did not restore full file list: %q", view)
	}
	if got := model.selectedFileValue().Path; got != "internal/tui/model.go" {
		t.Fatalf("selection after clearing filter = %q, want internal/tui/model.go", got)
	}
}

func TestFileFilterSelectsFirstMatchWhenCurrentFileDoesNotMatch(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "README.md", Status: gitview.Modified},
		{Path: "internal/tui/model.go", Status: gitview.Modified},
		{Path: "internal/git/repository.go", Status: gitview.Modified},
	}
	model.selected = 0
	model.diffs = map[string]string{
		model.selectedWorktreeValue().Path + "\x00" + "internal/tui/model.go": "diff --git a/internal/tui/model.go b/internal/tui/model.go\n+model",
	}
	model.refreshDiff()

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	model = next.(Model)
	for _, key := range []rune{'m', 'o', 'd', 'e', 'l'} {
		next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: string(key), Code: key}))
		model = next.(Model)
	}

	if got := model.selectedFileValue().Path; got != "internal/tui/model.go" {
		t.Fatalf("selected path after filtering = %q, want first match", got)
	}
}

func TestDiffGoTopBottomKeepsDiffFocus(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneDiff
	model.setDiffContent(strings.Join([]string{
		"@@ -1,12 +1,12 @@",
		" line-1",
		" line-2",
		" line-3",
		" line-4",
		" line-5",
		" line-6",
		" line-7",
		" line-8",
		" line-9",
		" line-10",
		" line-11",
		" line-12",
	}, "\n"))
	model.diff.SetSize(model.diff.Width(), 4)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "G", Code: 'G'}))
	model = next.(Model)
	if model.focusedPane != paneDiff {
		t.Fatalf("G from diff focusedPane = %v, want paneDiff", model.focusedPane)
	}
	if model.diff.YOffset() == 0 {
		t.Fatal("G from diff should move viewport to bottom")
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	model = next.(Model)
	if model.focusedPane != paneDiff {
		t.Fatalf("g from diff focusedPane = %v, want paneDiff", model.focusedPane)
	}
	if model.diff.YOffset() != 0 {
		t.Fatalf("g from diff y offset = %d, want 0", model.diff.YOffset())
	}
}

func TestSelectionLoadsMissingDiffLazily(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Modified},
	}
	model.diffs = map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"}
	model.loadDiff = func(_ context.Context, _ string, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n+b"
	}
	model.refreshDiff()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	if cmd == nil {
		t.Fatal("selection change did not request lazy diff load")
	}
	msg := cmd()
	next, _ = next.(Model).Update(msg)
	got := next.(Model)

	if !strings.Contains(got.View().Content, "+b") {
		t.Fatalf("View() missing lazily loaded diff: %q", got.View().Content)
	}
}

func TestStaleLazyDiffDoesNotOverwriteRefreshedSnapshot(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}}
	model.diffs = map[string]string{}
	model.loadDiff = func(_ context.Context, _ string, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n+stale"
	}
	model.refreshDiff()

	cmd := model.ensureSelectedDiffCmd()
	if cmd == nil {
		t.Fatal("expected lazy diff command")
	}
	model.applySnapshot(Snapshot{
		Changes: []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}},
		Diffs:   map[string]string{"a.go": "diff --git a/a.go b/a.go\n+fresh"},
	})

	next, _ := model.Update(cmd())
	got := next.(Model)
	view := got.View().Content
	if strings.Contains(view, "stale") || !strings.Contains(view, "fresh") {
		t.Fatalf("stale lazy diff overwrote refreshed snapshot: %q", view)
	}
}

func TestSelectionChangeForcesSelectedDiffReload(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Modified},
	}
	model.diffs = map[string]string{
		model.diffKey(model.changes[0]): "diff --git a/a.go b/a.go\n+cached-a",
		model.diffKey(model.changes[1]): "diff --git a/b.go b/b.go\n+cached-b",
	}
	model.loadDiff = func(_ context.Context, _ string, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n+reloaded-" + change.Path
	}
	model.refreshDiff()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	if cmd == nil {
		t.Fatal("selection change should force selected diff reload")
	}
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if diff := got.diffs[got.diffKey(got.selectedFileValue())]; !strings.Contains(diff, "reloaded-b.go") {
		t.Fatalf("selected diff = %q, want reloaded b.go", diff)
	}
	if view := ansi.Strip(got.View().Content); !strings.Contains(view, "reloaded-b.go") {
		t.Fatalf("view missing reloaded selected diff: %q", view)
	}
}

func TestStaleRefreshDoesNotOverwriteNewerSnapshot(t *testing.T) {
	model := testModel(t)
	model.refreshGeneration = 2
	model.applySnapshot(Snapshot{
		Changes: []gitview.FileChange{{Path: "fresh.go", Status: gitview.Modified}},
		Diffs:   map[string]string{"fresh.go": "diff --git a/fresh.go b/fresh.go\n+fresh"},
	})
	model.refreshGeneration = 2

	next, _ := model.Update(reloadMsg{
		generation: 1,
		snapshot: Snapshot{
			Changes: []gitview.FileChange{{Path: "stale.go", Status: gitview.Modified}},
			Diffs:   map[string]string{"stale.go": "diff --git a/stale.go b/stale.go\n+stale"},
		},
	})
	got := next.(Model)
	view := got.View().Content
	if strings.Contains(view, "stale.go") || !strings.Contains(view, "fresh.go") {
		t.Fatalf("stale refresh overwrote newer snapshot: %q", view)
	}
}

func TestRefreshStartInvalidatesPendingLazyDiff(t *testing.T) {
	model := testModel(t)
	model.diffs = map[string]string{}
	model.loadDiff = func(context.Context, string, gitview.FileChange) string {
		return "diff --git a/a.go b/a.go\n+stale"
	}
	model.refreshDiff()
	cmd := model.ensureSelectedDiffCmd()
	if cmd == nil {
		t.Fatal("expected lazy diff command")
	}

	next, _ := model.Update(autoRefreshMsg{})
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if strings.Contains(got.View().Content, "stale") {
		t.Fatalf("pending lazy diff survived refresh start: %q", got.View().Content)
	}
}

func testModel(t *testing.T) Model {
	t.Helper()
	tm, err := theme.Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	return NewModel(Config{
		ThemeName: "tokyonight-night",
		Theme:     theme.NewStyles(tm),
		Changes:   []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}},
		Diffs:     map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"},
	})
}

func overlapTestModel(t *testing.T) Model {
	t.Helper()
	model := testModel(t)
	model.width = 120
	model.height = 24
	model.worktrees = []WorktreeState{
		{
			Worktree: gitview.Worktree{Path: "/repo", Branch: "main", Current: true},
			Changes: []gitview.FileChange{
				{Path: "a.go", Status: gitview.Modified},
				{Path: "b.go", Status: gitview.Modified},
			},
		},
		{
			Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"},
			Changes:  []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}},
		},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	model.diffs = map[string]string{
		model.diffKey(model.selectedFileValue()): "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-current\n+current",
	}
	model.loadDiff = func(_ context.Context, worktreePath string, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n@@ -1 +1 @@\n-overlap\n+feature " + worktreePath
	}
	model.refreshDiff()
	return model
}

func containsEscape(value string, want string) bool {
	return countEscape(value, want) > 0
}

func styleBackgroundToken(style lipgloss.Style) string {
	rendered := style.Render(" ")
	return styleANSIToken(rendered, "48;2;")
}

func styleForegroundToken(style lipgloss.Style) string {
	rendered := style.Render(" ")
	return styleANSIToken(rendered, "38;2;")
}

func foregroundOnlyToken(style lipgloss.Style) string {
	token := styleForegroundToken(style)
	if before, _, ok := strings.Cut(token, ";48;2;"); ok {
		return before
	}
	return token
}

func styleANSIToken(rendered, prefix string) string {
	start := strings.Index(rendered, prefix)
	if start < 0 {
		return ""
	}
	end := strings.IndexByte(rendered[start:], 'm')
	if end < 0 {
		return rendered[start:]
	}
	return rendered[start : start+end]
}

func findRenderedLine(rendered, plainSubstring string) string {
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(ansi.Strip(line), plainSubstring) {
			return line
		}
	}
	return ""
}

func requireBatchCommand(t *testing.T, cmd tea.Cmd, wantLen int) tea.BatchMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("command is nil")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("command message = %T, want tea.BatchMsg", msg)
	}
	if len(batch) != wantLen {
		t.Fatalf("batch command length = %d, want %d", len(batch), wantLen)
	}
	return batch
}

func commandMsgOfType[T any](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	var zero T
	if cmd == nil {
		t.Fatal("command is nil")
	}
	msg := cmd()
	if got, ok := msg.(T); ok {
		return got
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("command message = %T, want %T or tea.BatchMsg", msg, zero)
	}
	got, ok := batchMsgOfType[T](batch)
	if !ok {
		t.Fatalf("batch command missing message type %T", zero)
	}
	return got
}

func requireBatchMsg[T any](t *testing.T, batch tea.BatchMsg) T {
	t.Helper()
	var zero T
	got, ok := batchMsgOfType[T](batch)
	if !ok {
		t.Fatalf("batch command missing message type %T", zero)
	}
	return got
}

func batchMsgOfType[T any](batch tea.BatchMsg) (T, bool) {
	var zero T
	for _, cmd := range batch {
		if cmd == nil {
			continue
		}
		msg := cmd()
		if got, ok := msg.(T); ok {
			return got, true
		}
	}
	return zero, false
}

func countEscape(value string, want string) int {
	count := 0
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			count++
		}
	}
	return count
}
