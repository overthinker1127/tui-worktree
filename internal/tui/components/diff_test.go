package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

func TestDiffSetContentReportsChanges(t *testing.T) {
	diff := testDiff(t, 60, 2)

	content := strings.Join([]string{
		"line-1",
		"line-2",
		"line-3",
		"line-4",
		"line-5",
		"line-6",
	}, "\n")
	if !diff.SetContent("a", content) {
		t.Fatal("first content set should report changed")
	}
	diff.SetYOffset(3)

	if diff.SetContent("a", content) {
		t.Fatal("same key and content should report unchanged")
	}
	if got := diff.YOffset(); got != 3 {
		t.Fatalf("unchanged content y offset = %d, want 3", got)
	}
	if !diff.SetContent("b", content) {
		t.Fatal("same content with a new key should report changed")
	}
}

func TestDiffLineNumbersRenderAndToggle(t *testing.T) {
	diff := testDiff(t, 80, 6)
	diff.SetContent("diff", strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"@@ -10,2 +20,2 @@ func main() {",
		" unchanged",
		"-old",
		"+new",
	}, "\n"))

	if !diff.ShowLineNumbers() {
		t.Fatal("line numbers should be enabled by default")
	}
	view := ansi.Strip(diff.RenderContent())
	for _, want := range []string{"   20 │  unchanged", "  -11 │ -old", "   21 │ +new"} {
		if !strings.Contains(view, want) {
			t.Fatalf("line number gutter missing %q in %q", want, view)
		}
	}

	if diff.ToggleLineNumbers() {
		t.Fatal("first line number toggle should disable line numbers")
	}
	view = ansi.Strip(diff.RenderContent())
	if strings.Contains(view, "20 │") || strings.Contains(view, "-11 │") {
		t.Fatalf("line number gutter should be hidden: %q", view)
	}
}

func TestDiffWrappedContinuationKeepsBlankGutter(t *testing.T) {
	diff := testDiff(t, 24, 6)
	diff.SetContent("wrap", "@@ -1,1 +1,1 @@\n+"+strings.Repeat("x", diff.TextWidth(diff.Width()))+"tail")

	lines := strings.Split(ansi.Strip(diff.RenderContent()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected wrapped numbered diff: %q", diff.RenderContent())
	}
	if !strings.Contains(lines[1], "    1 │ +") {
		t.Fatalf("first wrapped line missing new line number gutter: %q", lines[1])
	}
	if strings.Contains(lines[2], "1 │") {
		t.Fatalf("continuation line should not repeat line number: %q", lines[2])
	}
	if !strings.HasPrefix(lines[2], strings.Repeat(" ", diff.GutterWidth())) {
		t.Fatalf("continuation line should keep blank gutter: %q", lines[2])
	}
}

func TestDiffHorizontalScrollWhenWrapIsOff(t *testing.T) {
	diff := testDiff(t, 18, 4)
	diff.SetContent("long", "@@ -1,1 +1,1 @@\n+abcdefghijklmnopqrstuvwxyz")

	if diff.ToggleWrap() {
		t.Fatal("first wrap toggle should disable soft wrap")
	}
	diff.SetXOffset(8)

	view := ansi.Strip(diff.RenderContent())
	if strings.Contains(view, "+abcdefg") {
		t.Fatalf("horizontal scroll should cut the beginning of the line: %q", view)
	}
	if !strings.Contains(view, "hijklmnopq") {
		t.Fatalf("horizontal scroll should render the requested segment: %q", view)
	}
}

func TestDiffRenderTextAtUsesCurrentWrapAndLineNumbers(t *testing.T) {
	diff := testDiff(t, 80, 6)
	diff.ToggleWrap()

	rendered := ansi.Strip(diff.RenderTextAt("@@ -1,1 +1,1 @@\n+abcdefghijklmnopqrstuvwxyz", 18, 3, 1, 8))

	if strings.Contains(rendered, "+abcdefg") {
		t.Fatalf("RenderTextAt should honor x offset when wrap is off: %q", rendered)
	}
	if !strings.Contains(rendered, "hijklmnopq") {
		t.Fatalf("RenderTextAt should render scrolled segment: %q", rendered)
	}
	if !strings.Contains(rendered, "1 │") {
		t.Fatalf("RenderTextAt should use line number settings: %q", rendered)
	}
}

func TestDiffHighlightsHunkRangesAndCodeKeywords(t *testing.T) {
	styles := testStyles(t)
	diff := NewDiff(styles)
	diff.SetSize(80, 8)
	diff.SetContent("highlight", strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"@@ -145,7 +145,8 @@ func main() {",
		"+func main() { return }",
	}, "\n"))

	view := diff.RenderContent()
	for _, token := range []string{
		styleForegroundToken(styles.Added),
		styleForegroundToken(styles.Deleted),
		foregroundOnlyToken(styles.DiffKeyword),
	} {
		if token != "" && !strings.Contains(view, token) {
			t.Fatalf("diff highlight should contain token %q in %q", token, view)
		}
	}
	if !strings.Contains(ansi.Strip(view), "-145,7 +145,8") {
		t.Fatalf("hunk header missing line ranges: %q", view)
	}
}

func TestDiffSyntaxSkipsNonCodeFiles(t *testing.T) {
	styles := testStyles(t)
	diff := NewDiff(styles)
	diff.SetSize(80, 5)
	diff.SetContent("markdown", strings.Join([]string{
		"diff --git a/README.md b/README.md",
		"@@ -1,1 +1,1 @@",
		"+true return class function should read as plain text",
	}, "\n"))

	view := diff.RenderContent()
	keywordToken := foregroundOnlyToken(styles.DiffKeyword)
	if keywordToken != "" && strings.Contains(view, keywordToken) {
		t.Fatalf("non-code file should not use keyword token %q in %q", keywordToken, view)
	}
	if !strings.Contains(ansi.Strip(view), "true return class function") {
		t.Fatalf("non-code diff should still render text: %q", view)
	}
}

func TestDiffFillsEmptyRowsWithDiffBackground(t *testing.T) {
	styles := testStyles(t)
	diff := NewDiff(styles)
	diff.SetSize(30, 4)
	diff.SetContent("short", "short")

	lines := strings.Split(diff.RenderContent(), "\n")
	if len(lines) != 4 {
		t.Fatalf("rendered line count = %d, want 4: %q", len(lines), diff.RenderContent())
	}
	if !containsEscape(lines[len(lines)-1], "48;2;") {
		t.Fatalf("empty viewport line should keep diff background: %q", lines[len(lines)-1])
	}
	if strings.Contains(lines[0], "short\x1b[m ") {
		t.Fatalf("short diff row resets before padding spaces: %q", lines[0])
	}
}

func TestDiffAddsSpacerBeforeLaterHunksInSameFile(t *testing.T) {
	diff := testDiff(t, 80, 12)
	diff.ToggleLineNumbers()
	diff.SetContent("multi-hunk", strings.Join([]string{
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

	viewLines := strings.Split(ansi.Strip(diff.RenderContent()), "\n")
	trimmed := make([]string, 0, len(viewLines))
	for _, line := range viewLines {
		trimmed = append(trimmed, strings.TrimRight(line, " "))
	}
	view := strings.Join(trimmed, "\n")
	if !strings.Contains(view, "@@ -1,1 +1,1 @@ func a()\n-old\n+new\n\n@@ -20,1 +20,1 @@ func b()") {
		t.Fatalf("same-file hunk separator missing in %q", view)
	}
	if strings.Contains(view, "diff --git a/b.go b/b.go\n\n@@") {
		t.Fatalf("first hunk in new file should not get separator: %q", view)
	}
}

func testDiff(t *testing.T, width, height int) Diff {
	t.Helper()
	diff := NewDiff(testStyles(t))
	diff.SetSize(width, height)
	return diff
}

func testStyles(t *testing.T) theme.Styles {
	t.Helper()
	tm, err := theme.Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	return theme.NewStyles(tm)
}

func containsEscape(value string, want string) bool {
	return strings.Contains(value, want)
}

func styleForegroundToken(style lipgloss.Style) string {
	return styleANSIToken(style.Render(" "), "38;2;")
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
