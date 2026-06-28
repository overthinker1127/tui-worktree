package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderProgressBarFitsWidth(t *testing.T) {
	view := RenderProgressBar(lipgloss.NewStyle(), lipgloss.NewStyle(), 24, "In progress")
	stripped := ansi.Strip(view)

	if lipgloss.Width(stripped) != 24 {
		t.Fatalf("progress bar width = %d, want 24: %q", lipgloss.Width(stripped), stripped)
	}
	for _, want := range []string{"In progress", "[", "]", "="} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("progress bar missing %q: %q", want, stripped)
		}
	}
}
