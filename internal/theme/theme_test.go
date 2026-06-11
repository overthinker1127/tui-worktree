package theme

import "testing"

func TestPresetReturnsBuiltInThemes(t *testing.T) {
	for _, name := range []string{
		"tokyonight",
		"tokyonight-night",
		"tokyonight-storm",
		"kanagawa",
		"kanagawa-wave",
		"kanagawa-dragon",
		"catppuccin",
		"catppuccin-mocha",
		"catppuccin-macchiato",
		"gruvbox",
		"gruvbox-dark",
		"solarized",
		"solarized-dark",
		"nord",
		"dracula",
		"rose-pine",
		"rose-pine-moon",
		"one-dark",
		"vscode",
		"vscode-dark",
		"monokai",
		"everforest",
		"ayu",
		"ayu-mirage",
	} {
		got, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) error = %v", name, err)
		}
		if got.Name == "" {
			t.Fatalf("Preset(%q).Name is empty", name)
		}
		if got.Background == "" || got.Foreground == "" || got.Accent == "" {
			t.Fatalf("Preset(%q) returned empty color tokens: %#v", name, got)
		}
	}
}

func TestPresetRejectsUnknownTheme(t *testing.T) {
	for _, name := range []string{"unknown", "dark", "light", "catppucine"} {
		if _, err := Preset(name); err == nil {
			t.Fatalf("Preset(%q) error = nil, want non-nil", name)
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
	for _, want := range []string{"vscode", "catppuccin", "gruvbox", "solarized", "tokyonight", "kanagawa"} {
		if !contains(names, want) {
			t.Fatalf("Names() missing %q: %#v", want, names)
		}
	}
}

func TestNewStylesBuildsRenderableStyles(t *testing.T) {
	tm, err := Preset("tokyonight")
	if err != nil {
		t.Fatalf("Preset(\"tokyonight\") error = %v", err)
	}

	styles := NewStyles(tm)
	rendered := styles.Title.Render("Files changed")

	if rendered == "" || rendered == "Files changed" {
		t.Fatalf("Title.Render() = %q, want styled output", rendered)
	}
}

func TestDiffStylesUseLineBackgrounds(t *testing.T) {
	tm, err := Preset("tokyonight")
	if err != nil {
		t.Fatalf("Preset(\"tokyonight\") error = %v", err)
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
