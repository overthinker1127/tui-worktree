package theme

import "charm.land/lipgloss/v2"

type Styles struct {
	App            lipgloss.Style
	Title          lipgloss.Style
	Header         lipgloss.Style
	Panel          lipgloss.Style
	PanelFocused   lipgloss.Style
	FileItem       lipgloss.Style
	FileSelected   lipgloss.Style
	Muted          lipgloss.Style
	Added          lipgloss.Style
	Deleted        lipgloss.Style
	Error          lipgloss.Style
	Footer         lipgloss.Style
	Diff           lipgloss.Style
	DiffHunk       lipgloss.Style
	DiffAddition   lipgloss.Style
	DiffDeletion   lipgloss.Style
	DiffFileHeader lipgloss.Style
}

func NewStyles(t Theme) Styles {
	addedBackground := firstNonEmpty(t.AddedBackground, "#123524")
	deletedBackground := firstNonEmpty(t.DeletedBackground, "#3a1f2b")
	panelBackground := lipgloss.Color(t.Panel)
	return Styles{
		App:            lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground)).Background(lipgloss.Color(t.Background)),
		Title:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Foreground)),
		Header:         lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),
		Panel:          lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(t.Border)).Background(lipgloss.Color(t.Panel)),
		PanelFocused:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(t.Accent)).Background(lipgloss.Color(t.Panel)),
		FileItem:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground)),
		FileSelected:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground)).Background(lipgloss.Color(t.PanelSelected)).Bold(true),
		Muted:          lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),
		Added:          lipgloss.NewStyle().Foreground(lipgloss.Color(t.Added)),
		Deleted:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.Deleted)),
		Error:          lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)).Bold(true),
		Footer:         lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)).Background(panelBackground),
		Diff:           lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground)).Background(panelBackground),
		DiffHunk:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Background(panelBackground),
		DiffAddition:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground)).Background(lipgloss.Color(addedBackground)),
		DiffDeletion:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground)).Background(lipgloss.Color(deletedBackground)),
		DiffFileHeader: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)).Background(panelBackground).Bold(true),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
