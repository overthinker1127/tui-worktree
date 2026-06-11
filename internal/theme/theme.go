package theme

import (
	"fmt"
	"slices"
)

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
	if name == "" {
		name = "tokyonight"
	}
	if theme, ok := presets[name]; ok {
		return theme, nil
	}
	return Theme{}, fmt.Errorf("unknown theme %q", name)
}

func Names() []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

var presets = map[string]Theme{
	"tokyonight":       tokyoNight(),
	"tokyonight-night": tokyoNight(),
	"tokyonight-storm": {
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
	},
	"kanagawa":      kanagawaWave(),
	"kanagawa-wave": kanagawaWave(),
	"kanagawa-dragon": {
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
	},
	"catppuccin":       catppuccinMocha(),
	"catppuccin-mocha": catppuccinMocha(),
	"catppuccin-macchiato": {
		Name:          "catppuccin-macchiato",
		Background:    "#24273a",
		Foreground:    "#cad3f5",
		Muted:         "#6e738d",
		Accent:        "#8aadf4",
		Border:        "#494d64",
		Selection:     "#5b6078",
		Added:         "#a6da95",
		Deleted:       "#ed8796",
		Error:         "#ed8796",
		Panel:         "#1e2030",
		PanelSelected: "#363a4f",
	},
	"gruvbox":        gruvboxDark(),
	"gruvbox-dark":   gruvboxDark(),
	"solarized":      solarizedDark(),
	"solarized-dark": solarizedDark(),
	"nord": {
		Name:          "nord",
		Background:    "#2e3440",
		Foreground:    "#d8dee9",
		Muted:         "#4c566a",
		Accent:        "#88c0d0",
		Border:        "#434c5e",
		Selection:     "#3b4252",
		Added:         "#a3be8c",
		Deleted:       "#bf616a",
		Error:         "#bf616a",
		Panel:         "#3b4252",
		PanelSelected: "#434c5e",
	},
	"dracula": {
		Name:          "dracula",
		Background:    "#282a36",
		Foreground:    "#f8f8f2",
		Muted:         "#6272a4",
		Accent:        "#bd93f9",
		Border:        "#44475a",
		Selection:     "#44475a",
		Added:         "#50fa7b",
		Deleted:       "#ff5555",
		Error:         "#ff5555",
		Panel:         "#21222c",
		PanelSelected: "#44475a",
	},
	"rose-pine": rosePine(),
	"rose-pine-moon": {
		Name:          "rose-pine-moon",
		Background:    "#232136",
		Foreground:    "#e0def4",
		Muted:         "#6e6a86",
		Accent:        "#c4a7e7",
		Border:        "#393552",
		Selection:     "#44415a",
		Added:         "#3e8fb0",
		Deleted:       "#eb6f92",
		Error:         "#eb6f92",
		Panel:         "#2a273f",
		PanelSelected: "#393552",
	},
	"one-dark": {
		Name:          "one-dark",
		Background:    "#282c34",
		Foreground:    "#abb2bf",
		Muted:         "#5c6370",
		Accent:        "#61afef",
		Border:        "#3e4451",
		Selection:     "#3e4451",
		Added:         "#98c379",
		Deleted:       "#e06c75",
		Error:         "#e06c75",
		Panel:         "#21252b",
		PanelSelected: "#353b45",
	},
	"vscode":      vscodeDark(),
	"vscode-dark": vscodeDark(),
	"monokai": {
		Name:          "monokai",
		Background:    "#272822",
		Foreground:    "#f8f8f2",
		Muted:         "#75715e",
		Accent:        "#66d9ef",
		Border:        "#49483e",
		Selection:     "#49483e",
		Added:         "#a6e22e",
		Deleted:       "#f92672",
		Error:         "#f92672",
		Panel:         "#1e1f1c",
		PanelSelected: "#3e3d32",
	},
	"everforest": {
		Name:          "everforest",
		Background:    "#2d353b",
		Foreground:    "#d3c6aa",
		Muted:         "#859289",
		Accent:        "#7fbbb3",
		Border:        "#475258",
		Selection:     "#3d484d",
		Added:         "#a7c080",
		Deleted:       "#e67e80",
		Error:         "#e67e80",
		Panel:         "#232a2e",
		PanelSelected: "#343f44",
	},
	"ayu":        ayuMirage(),
	"ayu-mirage": ayuMirage(),
}

func tokyoNight() Theme {
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
	}
}

func kanagawaWave() Theme {
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
	}
}

func catppuccinMocha() Theme {
	return Theme{
		Name:          "catppuccin-mocha",
		Background:    "#1e1e2e",
		Foreground:    "#cdd6f4",
		Muted:         "#6c7086",
		Accent:        "#89b4fa",
		Border:        "#45475a",
		Selection:     "#585b70",
		Added:         "#a6e3a1",
		Deleted:       "#f38ba8",
		Error:         "#f38ba8",
		Panel:         "#181825",
		PanelSelected: "#313244",
	}
}

func gruvboxDark() Theme {
	return Theme{
		Name:          "gruvbox-dark",
		Background:    "#282828",
		Foreground:    "#ebdbb2",
		Muted:         "#928374",
		Accent:        "#83a598",
		Border:        "#504945",
		Selection:     "#3c3836",
		Added:         "#b8bb26",
		Deleted:       "#fb4934",
		Error:         "#fb4934",
		Panel:         "#1d2021",
		PanelSelected: "#32302f",
	}
}

func solarizedDark() Theme {
	return Theme{
		Name:          "solarized-dark",
		Background:    "#002b36",
		Foreground:    "#839496",
		Muted:         "#586e75",
		Accent:        "#268bd2",
		Border:        "#073642",
		Selection:     "#073642",
		Added:         "#859900",
		Deleted:       "#dc322f",
		Error:         "#dc322f",
		Panel:         "#073642",
		PanelSelected: "#094352",
	}
}

func rosePine() Theme {
	return Theme{
		Name:          "rose-pine",
		Background:    "#191724",
		Foreground:    "#e0def4",
		Muted:         "#6e6a86",
		Accent:        "#c4a7e7",
		Border:        "#26233a",
		Selection:     "#403d52",
		Added:         "#31748f",
		Deleted:       "#eb6f92",
		Error:         "#eb6f92",
		Panel:         "#1f1d2e",
		PanelSelected: "#26233a",
	}
}

func vscodeDark() Theme {
	return Theme{
		Name:          "vscode-dark",
		Background:    "#1e1e1e",
		Foreground:    "#d4d4d4",
		Muted:         "#858585",
		Accent:        "#569cd6",
		Border:        "#3c3c3c",
		Selection:     "#264f78",
		Added:         "#6a9955",
		Deleted:       "#f44747",
		Error:         "#f44747",
		Panel:         "#252526",
		PanelSelected: "#37373d",
	}
}

func ayuMirage() Theme {
	return Theme{
		Name:          "ayu-mirage",
		Background:    "#1f2430",
		Foreground:    "#cbccc6",
		Muted:         "#707a8c",
		Accent:        "#39bae6",
		Border:        "#343d4f",
		Selection:     "#33415e",
		Added:         "#bae67e",
		Deleted:       "#ff3333",
		Error:         "#ff3333",
		Panel:         "#1a1f29",
		PanelSelected: "#2f3b54",
	}
}
