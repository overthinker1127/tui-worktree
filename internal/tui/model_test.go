package tui

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
)

func TestModelViewShowsFileListAndDiff(t *testing.T) {
	tm, err := theme.Preset("tokyonight")
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
	for _, want := range []string{iconFile, iconModified} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing icon %q in %q", want, view)
		}
	}
}

func TestNewModelUsesConfiguredInitialSize(t *testing.T) {
	tm, err := theme.Preset("tokyonight")
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
	tm, err := theme.Preset("tokyonight")
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
	got := next.(Model).Selected()

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
	model.themeNames = []string{"tokyonight", "gruvbox"}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if got.themeName != "gruvbox" {
		t.Fatalf("themeName = %q, want gruvbox", got.themeName)
	}
	if strings.Contains(got.View().Content, "Theme changed") {
		t.Fatalf("theme success message should be hidden: %q", got.View().Content)
	}
}

func TestThemePickerSavesTheme(t *testing.T) {
	model := testModel(t)
	model.themeNames = []string{"tokyonight", "kanagawa"}
	var saved string
	model.saveTheme = func(name string) error {
		saved = name
		return nil
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))

	if saved != "kanagawa" {
		t.Fatalf("saved theme = %q, want kanagawa", saved)
	}
}

func TestThemePickerRendersAsOverlay(t *testing.T) {
	model := testModel(t)
	model.openThemePicker()

	view := model.View().Content
	for _, want := range []string{"Themes", "a.go", iconSelected + " tokyonight"} {
		if !strings.Contains(view, want) {
			t.Fatalf("theme overlay view missing %q in %q", want, view)
		}
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
		ThemeNames: []string{"solarized-light", "ayu"},
	})
	model.openThemePicker()

	row := findRenderedLine(model.renderThemePicker(), "  ayu")
	if row == "" {
		t.Fatalf("theme picker missing unselected row: %q", model.renderThemePicker())
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
		ThemeNames: []string{"solarized-light", "ayu"},
	})
	model.openThemePicker()
	picker := model.renderThemePicker()
	panelBorder := styleForegroundToken(model.styles.Panel)
	focusedBorder := styleForegroundToken(model.styles.PanelFocused)

	if panelBorder == "" || !strings.Contains(picker, panelBorder) {
		t.Fatalf("theme picker should use panel border token %q in %q", panelBorder, picker)
	}
	if focusedBorder != "" && strings.Contains(picker, focusedBorder) {
		t.Fatalf("theme picker should not use focused border token %q in %q", focusedBorder, picker)
	}
}

func TestThemePickerScrollsToCursor(t *testing.T) {
	model := testModel(t)
	model.height = 12
	model.themeNames = []string{
		"theme-01", "theme-02", "theme-03", "theme-04", "theme-05",
		"theme-06", "theme-07", "theme-08", "theme-09", "theme-10",
		"theme-11", "theme-12", "theme-13", "theme-14", "theme-15",
	}
	model.themeCursor = 12
	model.pickingTheme = true

	view := ansi.Strip(model.renderThemePicker())

	if !strings.Contains(view, iconSelected+" theme-13") {
		t.Fatalf("theme picker should keep cursor visible: %q", view)
	}
	if strings.Contains(view, "theme-01") {
		t.Fatalf("theme picker should not render every theme when constrained: %q", view)
	}
}

func TestMouseClickSelectsScrolledTheme(t *testing.T) {
	model := testModel(t)
	model.height = 12
	model.themeNames = []string{
		"ayu", "catppuccin", "dracula", "everforest", "gruvbox",
		"kanagawa", "monokai", "nord", "one-dark", "rose-pine",
		"solarized", "tokyonight", "vscode", "vscode-dark", "tokyonight-storm",
	}
	model.themeCursor = 12
	model.pickingTheme = true
	model.saveTheme = func(string) error { return nil }
	overlay := model.renderThemePicker()
	x, y := model.overlayPosition(overlay)

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: x + 2, Y: y + 4}))
	got := next.(Model)

	if got.themeName != "nord" {
		t.Fatalf("themeName = %q, want nord", got.themeName)
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
	for _, want := range []string{"1/2/3 panels", "tab worktree", "hjkl move", "e edit", "t themes", "q quit"} {
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
	for _, want := range []string{iconWorktree, iconKey, iconFile, iconEdit, iconTheme, iconQuit, " │ "} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer missing status bar segment %q in %q", want, footer)
		}
	}
	for _, want := range []string{"1/2/3", "tab", "hjkl", "e", "t", "q"} {
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

	hint := model.footerHint("", "w", "wrap")
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
	hint := model.footerHint(iconKey, "1/2/3", "panels")
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
	top := model.renderPanelTop(model.styles.PanelFocused, true, "[3]-"+iconFile+" Diff", 40)
	background := styleBackgroundToken(model.styles.PanelFocused)

	if background == "" {
		t.Fatal("panel background token should not be empty")
	}
	if got := strings.Count(top, background); got < 5 {
		t.Fatalf("panel title padding should use panel background, got %d background spans in %q", got, top)
	}
}

func TestSelectedListRowStripsNestedAnsiStyles(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	styles := theme.NewStyles(tm)
	content := renderFileLine(styles, gitview.FileChange{Path: "internal/tui/model.go", Status: gitview.Modified, Additions: 2, Deletions: 1})
	row := renderScrollableListRow(styles.FileSelected, iconSelected+" ", content, 0, 60, true)

	for _, token := range []string{
		styleForegroundToken(styles.Muted),
		styleForegroundToken(styles.Added),
		styleForegroundToken(styles.Deleted),
	} {
		if token != "" && strings.Contains(row, token) {
			t.Fatalf("selected row should not contain nested foreground token %q in %q", token, row)
		}
	}
}

func TestListLineSpacesUsePanelBackground(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	styles := theme.NewStyles(tm)
	background := styleBackgroundToken(styles.Panel)
	fileLine := renderFileLine(styles, gitview.FileChange{Path: "internal/tui/model.go", Status: gitview.Modified, Additions: 6, Deletions: 2})
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

func TestFooterAlignsToRightEdge(t *testing.T) {
	model := testModel(t)
	model.width = 160

	line := rightAlignText(model.footerText(), model.width)

	if got := lipgloss.Width(line); got != model.width {
		t.Fatalf("right aligned footer width = %d, want %d", got, model.width)
	}
	if !strings.HasSuffix(line, model.footerText()) {
		t.Fatalf("footer should be right aligned: %q", line)
	}
	if !strings.HasPrefix(line, " ") {
		t.Fatalf("footer should have leading fill before text: %q", line)
	}
	if got := rightAlignText("abcdef", 3); got != "def" {
		t.Fatalf("narrow footer = %q, want def", got)
	}
}

func TestRenderedFooterLeadingPaddingUsesFooterBackground(t *testing.T) {
	tm, err := theme.Preset("solarized-light")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{Theme: theme.NewStyles(tm), Width: 120})
	footer := model.renderFooter()
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
	for _, want := range []string{iconSelected + "   " + iconBranch + " feature", iconSelected + " " + iconAdded + " b.go"} {
		if !strings.Contains(view, want) {
			t.Fatalf("selected row missing visible pointer %q in %q", want, view)
		}
	}
}

func TestHorizontalScrollMovesFocusedFileLine(t *testing.T) {
	model := testModel(t)
	model.width = 54
	model.changes = []gitview.FileChange{{
		Path:   "internal/some/really/long/path/that/needs/scrolling/model.go",
		Status: gitview.Modified,
	}}
	model.diffs = map[string]string{}
	model.focusedPane = paneFiles
	model.refreshDiff()

	initial := ansi.Strip(model.View().Content)
	if !strings.Contains(initial, "internal/some") {
		t.Fatalf("initial view should show start of long path: %q", initial)
	}

	for range 20 {
		next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
		model = next.(Model)
	}
	scrolled := ansi.Strip(model.View().Content)
	if !strings.Contains(scrolled, "▸ ly/long/path/that/") {
		t.Fatalf("horizontal scroll did not move file row: %q", scrolled)
	}
	if model.fileScrollX != 20 {
		t.Fatalf("fileScrollX = %d, want 20", model.fileScrollX)
	}
}

func TestDiffWrapToggleDisablesSoftWrap(t *testing.T) {
	model := testModel(t)

	if !model.viewport.SoftWrap {
		t.Fatal("diff wrap should be enabled by default")
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "w", Code: 'w'}))
	got := next.(Model)

	if got.viewport.SoftWrap {
		t.Fatal("w should disable diff wrap on first press")
	}
	if got.toast.Message != "diff wrap off" || got.toast.Kind != toastInfo {
		t.Fatalf("toast = %#v, want info diff wrap off", got.toast)
	}
	if !strings.Contains(got.View().Content, "diff wrap off") {
		t.Fatalf("toast should render in view: %q", got.View().Content)
	}
}

func TestToastHelpersSetTypedVariants(t *testing.T) {
	model := testModel(t)

	infoCmd := model.showToast("saved config")
	if infoCmd == nil {
		t.Fatal("info toast should return expiration command")
	}
	if model.toast.Message != "saved config" || model.toast.Kind != toastInfo {
		t.Fatalf("info toast = %#v, want info saved config", model.toast)
	}

	errorCmd := model.showErrorToast("editor failed")
	if errorCmd == nil {
		t.Fatal("error toast should return expiration command")
	}
	if model.toast.Message != "editor failed" || model.toast.Kind != toastError {
		t.Fatalf("error toast = %#v, want error editor failed", model.toast)
	}

	successCmd := model.showSuccessToast("deleted feature")
	if successCmd == nil {
		t.Fatal("success toast should return expiration command")
	}
	if model.toast.Message != "deleted feature" || model.toast.Kind != toastSuccess {
		t.Fatalf("success toast = %#v, want success deleted feature", model.toast)
	}
}

func TestToastRendersAsVariantOverlay(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.showErrorToast("editor failed: boom")

	view := ansi.Strip(model.View().Content)

	for _, want := range []string{"Error", "editor failed: boom", "╭", "╯"} {
		if !strings.Contains(view, want) {
			t.Fatalf("toast overlay missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, iconStatus+" editor failed: boom") {
		t.Fatalf("toast should not render as compact status text: %q", view)
	}
}

func TestToastExpires(t *testing.T) {
	model := testModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "w", Code: 'w'}))
	model = next.(Model)
	if model.toast.Message == "" {
		t.Fatal("expected toast after wrap toggle")
	}

	next, _ = model.Update(toastExpiredMsg{id: model.toastID})
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

	if got := model.viewport.XOffset(); got == 0 {
		t.Fatal("l should scroll diff viewport right when wrap is off")
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	model = next.(Model)
	if got := model.viewport.XOffset(); got != 0 {
		t.Fatalf("h should scroll diff viewport left, got x offset %d", got)
	}
}

func TestLineNumbersRenderByDefault(t *testing.T) {
	model := testModel(t)
	model.setDiffContent("diff --git a/a.go b/a.go\n@@ -10,2 +20,2 @@ func main() {\n unchanged\n-old\n+new")

	if !model.showLineNumbers {
		t.Fatal("line numbers should be enabled by default")
	}
	view := ansi.Strip(model.renderDiffViewportContent())
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

	if model.showLineNumbers {
		t.Fatal("n should disable line numbers on first press")
	}
	view := ansi.Strip(model.renderDiffViewportContent())
	if strings.Contains(view, "1 │") {
		t.Fatalf("line number gutter should be hidden: %q", view)
	}
	if model.toast.Message != "line numbers off" || model.toast.Kind != toastInfo {
		t.Fatalf("toast = %#v, want info line numbers off", model.toast)
	}
}

func TestLineNumberGutterKeepsWrappedContinuationAligned(t *testing.T) {
	model := testModel(t)
	model.width = 72
	model.height = 20
	model.setDiffContent("@@ -1,1 +1,1 @@\n+" + strings.Repeat("x", model.diffTextWidth(model.viewport.Width())) + "tail")

	lines := strings.Split(ansi.Strip(model.renderDiffViewportContent()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected wrapped numbered diff: %q", model.renderDiffViewportContent())
	}
	if !strings.Contains(lines[1], "    1 │ +") {
		t.Fatalf("first wrapped line missing new line number gutter: %q", lines[1])
	}
	if strings.Contains(lines[2], "1 │") {
		t.Fatalf("continuation line should not repeat line number: %q", lines[2])
	}
	if !strings.HasPrefix(lines[2], strings.Repeat(" ", model.diffGutterWidth())) {
		t.Fatalf("continuation line should keep blank gutter: %q", lines[2])
	}
}

func TestDiffAddsSpacerBeforeLaterHunksInSameFile(t *testing.T) {
	model := testModel(t)
	model.showLineNumbers = false
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

	viewLines := strings.Split(ansi.Strip(model.renderDiffViewportContent()), "\n")
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

	lines := strings.Split(model.renderDiffViewportContent(), "\n")
	if len(lines) < 10 {
		t.Fatalf("viewport rendered too few lines: %q", model.renderDiffViewportContent())
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

	line := strings.Split(model.renderDiffViewportContent(), "\n")[0]
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
	model.setDiffContent("-" + strings.Repeat("x", model.viewport.Width()) + "}}")

	lines := strings.Split(model.renderDiffViewportContent(), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped diff tail line: %q", model.renderDiffViewportContent())
	}
	filled := lines[1]

	if got := lipgloss.Width(filled); got != model.viewport.Width() {
		t.Fatalf("filled width = %d, want %d: %q", got, model.viewport.Width(), filled)
	}
	if !strings.Contains(filled, styleBackgroundToken(model.styles.DiffDeletion)) {
		t.Fatalf("wrapped deletion tail should keep deletion background: %q", filled)
	}
	if strings.Contains(filled, "\x1b[m ") {
		t.Fatalf("wrapped deletion tail resets before fill spaces: %q", filled)
	}
}

func TestDiffSyntaxHighlightsStaticKeywords(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.refreshDiff()
	model.setDiffContent(strings.Join([]string{
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

	view := model.renderDiffViewportContent()
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

func TestDiffSyntaxDoesNotHighlightKeywordFragments(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 24
	model.refreshDiff()
	model.setDiffContent("+constellation functionality letter variable")

	view := model.renderDiffViewportContent()
	keywordToken := foregroundOnlyToken(model.styles.DiffKeyword)

	if keywordToken != "" && strings.Contains(view, keywordToken) {
		t.Fatalf("keyword fragments should not be highlighted with %q in %q", keywordToken, view)
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
	if !strings.Contains(plain, iconBranch+" feature") {
		t.Fatalf("worktree line missing branch label: %q", plain)
	}
}

func TestProtectedWorktreeLineShowsLock(t *testing.T) {
	model := testModel(t)
	line := renderWorktreeLine(model.styles, 0, WorktreeState{
		Worktree: gitview.Worktree{Branch: "main", Protected: true},
	})

	if !strings.Contains(ansi.Strip(line), iconProtected+" "+iconBranch+" main") {
		t.Fatalf("protected worktree line should show lock: %q", line)
	}
}

func TestDeleteKeyShowsConfirmForUnprotectedWorktree(t *testing.T) {
	model := testModel(t)
	model.focusedPane = paneWorktrees
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	got := next.(Model)

	if cmd != nil {
		t.Fatalf("delete confirm should not run command yet")
	}
	if !got.confirmDelete {
		t.Fatal("delete key should open confirm dialog")
	}
	view := ansi.Strip(got.View().Content)
	for _, want := range []string{"DELETE", "feature", "remove worktree and delete branch", "[Y]es", "[N]o"} {
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
	if got.confirmDelete {
		t.Fatal("protected delete should not open confirm dialog")
	}
	if !strings.Contains(got.toast.Message, "protected") || got.toast.Kind != toastError {
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
	if got.pickingMergeTarget {
		t.Fatal("default branch should not open merge target picker")
	}
	if got.toast.Kind != toastInfo || !strings.Contains(got.toast.Message, "default branch") {
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
	if !got.pickingMergeTarget {
		t.Fatal("m should open merge target picker")
	}
	selected, ok := got.mergeTargetList.SelectedItem().(mergeTargetItem)
	if !ok || selected.worktree.Branch != "main" {
		t.Fatalf("selected merge target = %#v, want main", got.mergeTargetList.SelectedItem())
	}
	view := ansi.Strip(got.View().Content)
	for _, want := range []string{"Merge into", "main", "dev"} {
		if !strings.Contains(view, want) {
			t.Fatalf("merge target view missing %q: %q", want, view)
		}
	}
}

func TestMergeTargetEnterRunsMergeCallback(t *testing.T) {
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
	if cmd == nil {
		t.Fatal("merge target enter should return merge command")
	}
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if gotReq.Source.Branch != "feature" || gotReq.Target.Branch != "main" {
		t.Fatalf("merge request = %#v, want feature into main", gotReq)
	}
	if got.pickingMergeTarget {
		t.Fatal("successful merge should close target picker")
	}
	if got.toast.Kind != toastSuccess || !strings.Contains(got.toast.Message, "merged feature into main") {
		t.Fatalf("toast = %#v, want merge success", got.toast)
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
	next, cmd := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	if cmd == nil {
		t.Fatal("first merge enter should return command")
	}
	next, duplicate := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if duplicate != nil {
		t.Fatal("duplicate merge enter should be ignored while merge is in flight")
	}
	if !got.mergingBranch {
		t.Fatal("merge should remain in flight until command finishes")
	}
}

func TestDefaultMergeBranchUsesNoEdit(t *testing.T) {
	dir := t.TempDir()
	bin := t.TempDir()
	logPath := bin + "/git-args"
	gitPath := bin + "/git"
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > " + logPath + "\n"
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := defaultMergeBranch(context.Background(), MergeRequest{
		Source: gitview.Worktree{Branch: "feature"},
		Target: gitview.Worktree{Path: dir, Branch: "main"},
	})
	if err != nil {
		t.Fatalf("defaultMergeBranch() error = %v", err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git args: %v", err)
	}

	if strings.TrimSpace(string(got)) != "merge --no-edit feature" {
		t.Fatalf("git args = %q, want merge --no-edit feature", strings.TrimSpace(string(got)))
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
	if got.creatingPR {
		t.Fatal("missing Forge CLI should not open PR form")
	}
	if got.toast.Kind != toastError || !strings.Contains(got.toast.Message, "gh or glab") {
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
	if !got.creatingPR {
		t.Fatal("PR key should open PR form")
	}
	view := ansi.Strip(got.View().Content)
	for _, want := range []string{"Forge CLI: gh", "PR title", "PR description", "<tab> focus", "<c-o> create"} {
		if !strings.Contains(view, want) {
			t.Fatalf("PR form view missing %q: %q", want, view)
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
	if model.prFormFocus != prFormBody {
		t.Fatalf("prFormFocus = %v, want body", model.prFormFocus)
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	model = next.(Model)
	if model.creatingPR {
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
	if !got.creatingPR {
		t.Fatal("empty PR title should keep PR form open")
	}
	if got.toast.Kind != toastError || !strings.Contains(got.toast.Message, "PR title is required") {
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
	model.prTitle.SetValue("Add PR creator")
	model.prBody.SetValue("Creates a pull request from the selected worktree.")
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
	if got.creatingPR {
		t.Fatal("successful PR create should close form")
	}
	if got.toast.Kind != toastSuccess || got.toast.Message != "PR/MR created" {
		t.Fatalf("toast = %#v, want PR/MR created success", got.toast)
	}
}

func TestPRFormSubmitIgnoresDuplicateWhileInFlight(t *testing.T) {
	model := testModel(t)
	model.findForgeCLI = func() (string, bool) { return "gh", true }
	model.createPullRequest = func(context.Context, PullRequestRequest) error { return nil }

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	model = next.(Model)
	model.prTitle.SetValue("Add PR creator")
	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("first PR submit should return command")
	}
	next, duplicate := next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
	got := next.(Model)

	if duplicate != nil {
		t.Fatal("duplicate PR submit should be ignored while create is in flight")
	}
	if !got.submittingPR {
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
	model.prTitle.SetValue("Add MR creator")
	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("PR submit should return create command")
	}
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if !got.creatingPR {
		t.Fatal("failed PR create should keep form open")
	}
	if got.toast.Kind != toastError || !strings.Contains(got.toast.Message, "not authenticated") {
		t.Fatalf("toast = %#v, want auth error", got.toast)
	}
	if got.submittingPR {
		t.Fatal("failed PR create should clear submitting state")
	}
}

func TestForgeCreateArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  PullRequestRequest
		want []string
	}{
		{
			name: "gh",
			req:  PullRequestRequest{CLI: "gh", Branch: "feature", Title: "Title", Body: "Body"},
			want: []string{"pr", "create", "--title", "Title", "--body", "Body", "--head", "feature"},
		},
		{
			name: "glab",
			req:  PullRequestRequest{CLI: "glab", Branch: "feature", Title: "Title", Body: "Body"},
			want: []string{"mr", "create", "--title", "Title", "--description", "Body", "--source-branch", "feature", "--yes"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := forgeCreateArgs(tc.req)
			if err != nil {
				t.Fatalf("forgeCreateArgs() error = %v", err)
			}
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Fatalf("forgeCreateArgs() = %#v, want %#v", got, tc.want)
			}
		})
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
	msg := cmd()
	next, _ = next.(Model).Update(msg)
	got := next.(Model)

	if deleted.Branch != "feature" {
		t.Fatalf("deleted branch = %q, want feature", deleted.Branch)
	}
	if got.confirmDelete {
		t.Fatal("confirm dialog should close after delete")
	}
	if !strings.Contains(got.toast.Message, "deleted feature") || got.toast.Kind != toastSuccess {
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
	if cmd == nil {
		t.Fatal("first confirm delete should return command")
	}
	next, duplicate := next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	got := next.(Model)

	if duplicate != nil {
		t.Fatal("duplicate confirm delete should be ignored while delete is in flight")
	}
	if !got.deletingWorktree {
		t.Fatal("delete should remain in flight until command finishes")
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

	if got.SelectedWorktree().Branch != "feature" || got.Selected().Path != "feature.go" {
		t.Fatalf("selected worktree/file = %q/%q, want feature/feature.go", got.SelectedWorktree().Branch, got.Selected().Path)
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
	if got.toast.Message != "no file selected" || got.toast.Kind != toastInfo {
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
	if !strings.Contains(got.toast.Message, "editor failed: boom") || got.toast.Kind != toastError {
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

func TestSidebarPanelsFillBodyHeight(t *testing.T) {
	model := testModel(t)
	model.width = 100
	model.height = 30
	leftWidth, _ := model.layoutWidths()
	contentHeight := model.bodyHeight()
	worktrees := model.renderWorktrees(leftWidth, model.worktreePaneHeight(contentHeight))
	files := model.renderFiles(leftWidth, max(4, contentHeight-lipgloss.Height(worktrees)))
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

	if got.Selected().Path != "fresh.go" {
		t.Fatalf("Selected() = %q, want fresh.go", got.Selected().Path)
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

func TestAutoRefreshClosesMergeTargetPickerToAvoidStaleTargets(t *testing.T) {
	model := testModel(t)
	model.worktrees = []WorktreeState{
		{Worktree: gitview.Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"}},
		{Worktree: gitview.Worktree{Path: "/repo", Branch: "main", DefaultBranch: true}},
	}
	model.selectedWorktree = 0
	model.normalizeWorktrees()
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	model = next.(Model)
	if !model.pickingMergeTarget {
		t.Fatal("merge target picker should be open before refresh")
	}

	model.applySnapshot(Snapshot{
		Worktrees:        model.worktrees,
		SelectedWorktree: model.selectedWorktree,
		Changes:          model.changes,
		Diffs:            model.diffs,
	})

	if model.pickingMergeTarget {
		t.Fatal("refresh should close merge target picker to avoid stale targets")
	}
	if model.mergeSource.Path != "" {
		t.Fatalf("mergeSource = %#v, want cleared", model.mergeSource)
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
				Worktree: model.SelectedWorktree(),
				Changes:  changes,
			}},
			SelectedWorktree: 0,
			Changes:          changes,
			Diffs: map[string]string{
				model.SelectedWorktree().Path + "\x00" + "b.go": "diff --git a/b.go b/b.go\n+b",
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

	if got.Selected().Path != "b.go" {
		t.Fatalf("Selected() = %q, want b.go", got.Selected().Path)
	}
	if !strings.Contains(got.View().Content, "+b") {
		t.Fatalf("View() should keep selected file diff: %q", got.View().Content)
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
	got := next.(Model).Selected()

	if got.Path != "b.go" {
		t.Fatalf("Selected() = %q, want b.go", got.Path)
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
	model.themeNames = []string{"tokyonight", "gruvbox"}
	model.openThemePicker()
	overlay := model.renderThemePicker()
	x, y := model.overlayPosition(overlay)

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: x + 2, Y: y + 4}))
	got := next.(Model)

	if got.themeName != "gruvbox" {
		t.Fatalf("themeName = %q, want gruvbox", got.themeName)
	}
	if got.pickingTheme {
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
	model.viewport.SetHeight(4)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "G", Code: 'G'}))
	model = next.(Model)
	if model.focusedPane != paneDiff {
		t.Fatalf("G from diff focusedPane = %v, want paneDiff", model.focusedPane)
	}
	if model.viewport.YOffset() == 0 {
		t.Fatal("G from diff should move viewport to bottom")
	}

	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	model = next.(Model)
	if model.focusedPane != paneDiff {
		t.Fatalf("g from diff focusedPane = %v, want paneDiff", model.focusedPane)
	}
	if model.viewport.YOffset() != 0 {
		t.Fatalf("g from diff y offset = %d, want 0", model.viewport.YOffset())
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
	tm, err := theme.Preset("tokyonight")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	return NewModel(Config{
		ThemeName: "tokyonight",
		Theme:     theme.NewStyles(tm),
		Changes:   []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}},
		Diffs:     map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"},
	})
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

func countEscape(value string, want string) int {
	count := 0
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			count++
		}
	}
	return count
}
