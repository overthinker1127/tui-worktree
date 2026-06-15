package theme

import "charm.land/lipgloss/v2"

type StyleOptions struct {
	Transparent bool
}

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
	DiffKeyword    lipgloss.Style
	DiffAddition   lipgloss.Style
	DiffDeletion   lipgloss.Style
	DiffFileHeader lipgloss.Style
}

func NewStyles(t Theme) Styles {
	return NewStylesWithOptions(t, StyleOptions{})
}

func NewStylesWithOptions(t Theme, opts StyleOptions) Styles {
	addedBackground := firstNonEmpty(t.AddedBackground, "#123524")
	deletedBackground := firstNonEmpty(t.DeletedBackground, "#3a1f2b")
	keyword := firstNonEmpty(t.Keyword, t.Accent)
	panelBackground := lipgloss.Color(t.Panel)
	app := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground))
	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(t.Border))
	panelFocused := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(t.Accent))
	fileSelected := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground)).Bold(true)
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))
	diff := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground))
	diffHunk := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent))
	diffKeyword := lipgloss.NewStyle().Foreground(lipgloss.Color(keyword))
	diffAddition := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Added))
	diffDeletion := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Deleted))
	diffFileHeader := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)).Bold(true)
	if !opts.Transparent {
		app = app.Background(lipgloss.Color(t.Background))
		panel = panel.BorderBackground(panelBackground).Background(panelBackground)
		panelFocused = panelFocused.BorderBackground(panelBackground).Background(panelBackground)
		fileSelected = fileSelected.Background(lipgloss.Color(t.PanelSelected))
		footer = footer.Background(panelBackground)
		diff = diff.Background(panelBackground)
		diffHunk = diffHunk.Background(panelBackground)
		diffKeyword = diffKeyword.Background(panelBackground)
		diffAddition = diffAddition.Foreground(lipgloss.Color(t.Foreground)).Background(lipgloss.Color(addedBackground))
		diffDeletion = diffDeletion.Foreground(lipgloss.Color(t.Foreground)).Background(lipgloss.Color(deletedBackground))
		diffFileHeader = diffFileHeader.Background(panelBackground)
	}
	return Styles{
		App:            app,
		Title:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Foreground)),
		Header:         lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),
		Panel:          panel,
		PanelFocused:   panelFocused,
		FileItem:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.Foreground)),
		FileSelected:   fileSelected,
		Muted:          lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),
		Added:          lipgloss.NewStyle().Foreground(lipgloss.Color(t.Added)),
		Deleted:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.Deleted)),
		Error:          lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)).Bold(true),
		Footer:         footer,
		Diff:           diff,
		DiffHunk:       diffHunk,
		DiffKeyword:    diffKeyword,
		DiffAddition:   diffAddition,
		DiffDeletion:   diffDeletion,
		DiffFileHeader: diffFileHeader,
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
