package theme

import "fmt"

type Theme struct {
	Name          string
	Background    string
	Foreground    string
	Muted         string
	Accent        string
	Border        string
	Selection     string
	Added         string
	Deleted       string
	Error         string
	Panel         string
	PanelSelected string
}

func Preset(name string) (Theme, error) {
	switch name {
	case "", "dark":
		return Theme{
			Name:          "dark",
			Background:    "#0d1117",
			Foreground:    "#e6edf3",
			Muted:         "#8b949e",
			Accent:        "#58a6ff",
			Border:        "#30363d",
			Selection:     "#1f6feb",
			Added:         "#3fb950",
			Deleted:       "#f85149",
			Error:         "#ff7b72",
			Panel:         "#161b22",
			PanelSelected: "#1f2937",
		}, nil
	case "light":
		return Theme{
			Name:          "light",
			Background:    "#ffffff",
			Foreground:    "#24292f",
			Muted:         "#57606a",
			Accent:        "#0969da",
			Border:        "#d0d7de",
			Selection:     "#ddf4ff",
			Added:         "#1a7f37",
			Deleted:       "#cf222e",
			Error:         "#cf222e",
			Panel:         "#f6f8fa",
			PanelSelected: "#ddf4ff",
		}, nil
	case "tokyonight", "tokyonight-night":
		return Theme{
			Name:          "tokyonight-night",
			Background:    "#1a1b26",
			Foreground:    "#c0caf5",
			Muted:         "#565f89",
			Accent:        "#7aa2f7",
			Border:        "#414868",
			Selection:     "#33467c",
			Added:         "#9ece6a",
			Deleted:       "#f7768e",
			Error:         "#db4b4b",
			Panel:         "#16161e",
			PanelSelected: "#24283b",
		}, nil
	case "tokyonight-storm":
		return Theme{
			Name:          "tokyonight-storm",
			Background:    "#24283b",
			Foreground:    "#c0caf5",
			Muted:         "#565f89",
			Accent:        "#7aa2f7",
			Border:        "#414868",
			Selection:     "#364a82",
			Added:         "#9ece6a",
			Deleted:       "#f7768e",
			Error:         "#db4b4b",
			Panel:         "#1f2335",
			PanelSelected: "#292e42",
		}, nil
	case "kanagawa", "kanagawa-wave":
		return Theme{
			Name:          "kanagawa-wave",
			Background:    "#1f1f28",
			Foreground:    "#dcd7ba",
			Muted:         "#727169",
			Accent:        "#7e9cd8",
			Border:        "#54546d",
			Selection:     "#2d4f67",
			Added:         "#98bb6c",
			Deleted:       "#e46876",
			Error:         "#e82424",
			Panel:         "#16161d",
			PanelSelected: "#223249",
		}, nil
	case "kanagawa-dragon":
		return Theme{
			Name:          "kanagawa-dragon",
			Background:    "#181616",
			Foreground:    "#c5c9c5",
			Muted:         "#737c73",
			Accent:        "#8ba4b0",
			Border:        "#393836",
			Selection:     "#282727",
			Added:         "#87a987",
			Deleted:       "#c4746e",
			Error:         "#c4746e",
			Panel:         "#0d0c0c",
			PanelSelected: "#1d1c19",
		}, nil
	default:
		return Theme{}, fmt.Errorf("unknown theme %q", name)
	}
}
