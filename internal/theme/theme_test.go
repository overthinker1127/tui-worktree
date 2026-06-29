package theme

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestPresetReturnsBuiltInThemes(t *testing.T) {
	for _, name := range []string{
		"tokyonight-night",
		"tokyonight-storm",
		"kanagawa-wave",
		"kanagawa-dragon",
		"catppuccin-frappe",
		"catppuccin-latte",
		"catppuccin-mocha",
		"catppuccin-macchiato",
		"gruvbox-dark",
		"gruvbox-light",
		"solarized-dark",
		"solarized-light",
		"nord",
		"dracula",
		"rose-pine",
		"rose-pine-dawn",
		"rose-pine-moon",
		"one-dark",
		"vscode-dark",
		"monokai",
		"everforest",
		"ayu-mirage",
		"github-dark",
		"github-light",
		"modus-vivendi",
		"modus-operandi",
		"nightfox",
		"dayfox",
		"carbonfox",
		"material-ocean",
		"palenight",
		"oxocarbon",
		"zenburn",
	} {
		got, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) error = %v", name, err)
		}
		if got.Name == "" {
			t.Fatalf("Preset(%q).Name is empty", name)
		}
		if got.Background == "" || got.Foreground == "" || got.Accent == "" || got.Keyword == "" {
			t.Fatalf("Preset(%q) returned empty color tokens: %#v", name, got)
		}
	}
}

func TestPresetRejectsUnknownTheme(t *testing.T) {
	for _, name := range []string{"unknown", "dark", "light", "catppucine", "kanagawa", "tokyonight", "catppuccin", "vscode", "ayu"} {
		if _, err := Preset(name); err == nil {
			t.Fatalf("Preset(%q) error = nil, want non-nil", name)
		}
	}
}

func TestPresetKeywordColorsUseSyntaxThemeColors(t *testing.T) {
	tests := map[string]string{
		"tokyonight-night": "#bb9af7",
		"catppuccin-mocha": "#cba6f7",
		"dracula":          "#ff79c6",
		"github-dark":      "#ff7b72",
		"github-light":     "#cf222e",
		"monokai":          "#f92672",
		"vscode-dark":      "#c586c0",
		"zenburn":          "#f0dfaf",
	}

	for name, want := range tests {
		got, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) error = %v", name, err)
		}
		if got.Keyword != want {
			t.Fatalf("Preset(%q).Keyword = %q, want syntax keyword color %q", name, got.Keyword, want)
		}
		if got.Keyword == got.Accent {
			t.Fatalf("Preset(%q).Keyword should be syntax-specific, not accent fallback", name)
		}
	}
}

func TestNamesIncludesOnlyNamedThemes(t *testing.T) {
	names := Names()
	for _, forbidden := range []string{"dark", "light"} {
		for _, name := range names {
			if name == forbidden {
				t.Fatalf("Names() included generic theme %q: %#v", forbidden, names)
			}
		}
	}
	for _, want := range []string{"vscode-dark", "catppuccin-mocha", "gruvbox-dark", "solarized-dark", "tokyonight-night", "kanagawa-wave"} {
		if !contains(names, want) {
			t.Fatalf("Names() missing %q: %#v", want, names)
		}
	}
	for _, alias := range []string{"kanagawa", "tokyonight", "catppuccin", "vscode", "ayu"} {
		if contains(names, alias) {
			t.Fatalf("Names() included alias %q: %#v", alias, names)
		}
	}
}

func TestPresetPalettesAreUnique(t *testing.T) {
	seen := map[Theme]string{}
	for _, name := range Names() {
		tm, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) error = %v", name, err)
		}
		tm.Name = ""
		if previous, ok := seen[tm]; ok {
			t.Fatalf("Preset(%q) duplicates palette from %q", name, previous)
		}
		seen[tm] = name
	}
}

func TestNewStylesBuildsRenderableStyles(t *testing.T) {
	tm, err := Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset(\"tokyonight-night\") error = %v", err)
	}

	styles := NewStyles(tm)
	rendered := styles.Title.Render("Files changed")

	if rendered == "" || rendered == "Files changed" {
		t.Fatalf("Title.Render() = %q, want styled output", rendered)
	}
}

func TestDiffKeywordUsesKeywordColorWhenConfigured(t *testing.T) {
	tm := Theme{
		Name:          "custom",
		Background:    "#000000",
		Foreground:    "#ffffff",
		Muted:         "#777777",
		Accent:        "#111111",
		Keyword:       "#ff00ff",
		Border:        "#222222",
		Added:         "#00ff00",
		Deleted:       "#ff0000",
		Error:         "#ff0000",
		Panel:         "#101010",
		PanelSelected: "#202020",
	}

	styles := NewStyles(tm)
	rendered := styles.DiffKeyword.Render("func")

	if !containsEscape(rendered, "38;2;255;0;255") {
		t.Fatalf("DiffKeyword = %q, want configured keyword foreground", rendered)
	}
	if containsEscape(rendered, "38;2;17;17;17") {
		t.Fatalf("DiffKeyword = %q, should not use accent when keyword is configured", rendered)
	}
	if containsEscape(rendered, "\x1b[1") || containsEscape(rendered, ";1;") {
		t.Fatalf("DiffKeyword = %q, should not render bold", rendered)
	}
}

func TestDiffKeywordFallsBackToAccentColor(t *testing.T) {
	tm := Theme{
		Name:          "custom",
		Background:    "#000000",
		Foreground:    "#ffffff",
		Muted:         "#777777",
		Accent:        "#111111",
		Border:        "#222222",
		Added:         "#00ff00",
		Deleted:       "#ff0000",
		Error:         "#ff0000",
		Panel:         "#101010",
		PanelSelected: "#202020",
	}

	styles := NewStyles(tm)
	rendered := styles.DiffKeyword.Render("func")

	if !containsEscape(rendered, "38;2;17;17;17") {
		t.Fatalf("DiffKeyword = %q, want accent fallback foreground", rendered)
	}
}

func TestDiffStylesUseLineBackgrounds(t *testing.T) {
	tm, err := Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset(\"tokyonight-night\") error = %v", err)
	}

	styles := NewStyles(tm)
	added := styles.DiffAddition.Width(12).Render("+hello")
	deleted := styles.DiffDeletion.Width(12).Render("-hello")
	neutral := styles.Diff.Width(12).Render("[Image #1]")

	for name, rendered := range map[string]string{"added": added, "deleted": deleted, "neutral": neutral} {
		if !containsEscape(rendered, "48;2;") {
			t.Fatalf("%s diff style = %q, want background color escape", name, rendered)
		}
	}
	for name, rendered := range map[string]string{"added": added, "deleted": deleted} {
		if !containsEscape(rendered, "38;2;192;202;245") {
			t.Fatalf("%s diff style = %q, want theme foreground escape", name, rendered)
		}
	}
}

func TestTransparentStylesDoNotPaintBackgrounds(t *testing.T) {
	tm, err := Preset("tokyonight-night")
	if err != nil {
		t.Fatalf("Preset(\"tokyonight-night\") error = %v", err)
	}

	styles := NewStylesWithOptions(tm, StyleOptions{Transparent: true})
	rendered := strings.Join([]string{
		styles.App.Render("app"),
		styles.Panel.Render("panel"),
		styles.FileSelected.Render("selected"),
		styles.Footer.Render("footer"),
		styles.Diff.Render("diff"),
		styles.DiffHunk.Render("@@ -1 +1 @@"),
		styles.DiffKeyword.Render("func"),
		styles.DiffAddition.Render("+added"),
		styles.DiffDeletion.Render("-deleted"),
		styles.DiffFileHeader.Render("diff --git"),
	}, "\n")

	if containsEscape(rendered, "48;2;") {
		t.Fatalf("transparent styles should not include truecolor background escapes: %q", rendered)
	}
	for _, token := range []string{
		styleForegroundEscape(tm.Added),
		styleForegroundEscape(tm.Deleted),
	} {
		if !containsEscape(rendered, token) {
			t.Fatalf("transparent diff styles should keep semantic foreground %q in %q", token, rendered)
		}
	}
}

func styleForegroundEscape(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}
	return fmt.Sprintf("38;2;%d;%d;%d", parseHexByte(hex[0:2]), parseHexByte(hex[2:4]), parseHexByte(hex[4:6]))
}

func parseHexByte(value string) int {
	parsed, _ := strconv.ParseUint(value, 16, 8)
	return int(parsed)
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsEscape(value string, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
