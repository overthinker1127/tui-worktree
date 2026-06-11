package theme

import "testing"

func TestPresetReturnsBuiltInThemes(t *testing.T) {
	for _, name := range []string{
		"dark",
		"light",
		"tokyonight",
		"tokyonight-night",
		"tokyonight-storm",
		"kanagawa",
		"kanagawa-wave",
		"kanagawa-dragon",
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
	if _, err := Preset("unknown"); err == nil {
		t.Fatal("Preset(\"unknown\") error = nil, want non-nil")
	}
}

func TestNewStylesBuildsRenderableStyles(t *testing.T) {
	tm, err := Preset("dark")
	if err != nil {
		t.Fatalf("Preset(\"dark\") error = %v", err)
	}

	styles := NewStyles(tm)
	rendered := styles.Title.Render("Files changed")

	if rendered == "" || rendered == "Files changed" {
		t.Fatalf("Title.Render() = %q, want styled output", rendered)
	}
}
