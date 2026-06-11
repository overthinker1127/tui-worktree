package tui

import (
	"context"
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
	for _, want := range []string{"󰈙", ""} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing Nerd Font symbol %q in %q", want, view)
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

func TestQuestionMarkTogglesHelp(t *testing.T) {
	model := testModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	view := next.(Model).View().Content

	for _, want := range []string{"Help", "1/2/3: focus panels", "auto-refresh", " t themes", "a.go"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help view missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "mouse select") {
		t.Fatalf("help view should not explain mouse selection: %q", view)
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
	for _, want := range []string{"Themes", "a.go"} {
		if !strings.Contains(view, want) {
			t.Fatalf("theme overlay view missing %q in %q", want, view)
		}
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
	for _, want := range []string{"1/2/3 panels", "tab worktree", "hjkl move", "t themes", "? help", "q quit"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer missing %q in %q", want, footer)
		}
	}
	if strings.Contains(footer, "auto 5s") {
		t.Fatalf("footer should not show auto-refresh interval: %q", footer)
	}
	for _, want := range []string{iconWorktree, iconKey, iconFile, " │ "} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer missing status bar segment %q in %q", want, footer)
		}
	}
}

func TestFooterAlignsToRightEdge(t *testing.T) {
	model := testModel(t)
	model.width = 120

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

func TestViewKeepsFooterOnLastLine(t *testing.T) {
	model := testModel(t)
	model.width = 90
	model.height = 24

	lines := strings.Split(model.View().Content, "\n")
	last := ansi.Strip(lines[len(lines)-1])

	if !strings.Contains(last, "1/2/3 panels") || !strings.Contains(last, "? help") {
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
	if got.toast != "diff wrap off" {
		t.Fatalf("toast = %q, want diff wrap off", got.toast)
	}
	if !strings.Contains(got.View().Content, "diff wrap off") {
		t.Fatalf("toast should render in view: %q", got.View().Content)
	}
}

func TestToastExpires(t *testing.T) {
	model := testModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "w", Code: 'w'}))
	model = next.(Model)
	if model.toast == "" {
		t.Fatal("expected toast after wrap toggle")
	}

	next, _ = model.Update(toastExpiredMsg{id: model.toastID})
	model = next.(Model)
	if model.toast != "" {
		t.Fatalf("toast = %q, want empty after expiration", model.toast)
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
	start := strings.Index(rendered, "48;2;")
	if start < 0 {
		return ""
	}
	end := strings.IndexByte(rendered[start:], 'm')
	if end < 0 {
		return rendered[start:]
	}
	return rendered[start : start+end]
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
