package theme

import (
	"fmt"
	"slices"
)

type Theme struct {
	Name              string
	Background        string
	Foreground        string
	Muted             string
	Accent            string
	Keyword           string
	Border            string
	Selection         string
	Added             string
	AddedBackground   string
	Deleted           string
	DeletedBackground string
	Error             string
	Panel             string
	PanelSelected     string
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
		Keyword:       "#bb9af7",
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
		Keyword:       "#957fb8",
		Border:        "#393836",
		Selection:     "#282727",
		Added:         "#87a987",
		Deleted:       "#c4746e",
		Error:         "#c4746e",
		Panel:         "#0d0c0c",
		PanelSelected: "#1d1c19",
	},
	"catppuccin":        catppuccinMocha(),
	"catppuccin-frappe": catppuccinFrappe(),
	"catppuccin-latte":  catppuccinLatte(),
	"catppuccin-mocha":  catppuccinMocha(),
	"catppuccin-macchiato": {
		Name:          "catppuccin-macchiato",
		Background:    "#24273a",
		Foreground:    "#cad3f5",
		Muted:         "#6e738d",
		Accent:        "#8aadf4",
		Keyword:       "#c6a0f6",
		Border:        "#494d64",
		Selection:     "#5b6078",
		Added:         "#a6da95",
		Deleted:       "#ed8796",
		Error:         "#ed8796",
		Panel:         "#1e2030",
		PanelSelected: "#363a4f",
	},
	"gruvbox":         gruvboxDark(),
	"gruvbox-dark":    gruvboxDark(),
	"gruvbox-light":   gruvboxLight(),
	"solarized":       solarizedDark(),
	"solarized-dark":  solarizedDark(),
	"solarized-light": solarizedLight(),
	"nord": {
		Name:          "nord",
		Background:    "#2e3440",
		Foreground:    "#d8dee9",
		Muted:         "#4c566a",
		Accent:        "#88c0d0",
		Keyword:       "#b48ead",
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
		Keyword:       "#ff79c6",
		Border:        "#44475a",
		Selection:     "#44475a",
		Added:         "#50fa7b",
		Deleted:       "#ff5555",
		Error:         "#ff5555",
		Panel:         "#21222c",
		PanelSelected: "#44475a",
	},
	"rose-pine":      rosePine(),
	"rose-pine-dawn": rosePineDawn(),
	"rose-pine-moon": {
		Name:          "rose-pine-moon",
		Background:    "#232136",
		Foreground:    "#e0def4",
		Muted:         "#6e6a86",
		Accent:        "#c4a7e7",
		Keyword:       "#eb6f92",
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
		Keyword:       "#c678dd",
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
		Keyword:       "#f92672",
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
		Keyword:       "#d699b6",
		Border:        "#475258",
		Selection:     "#3d484d",
		Added:         "#a7c080",
		Deleted:       "#e67e80",
		Error:         "#e67e80",
		Panel:         "#232a2e",
		PanelSelected: "#343f44",
	},
	"ayu":            ayuMirage(),
	"ayu-mirage":     ayuMirage(),
	"github-dark":    githubDark(),
	"github-light":   githubLight(),
	"modus-vivendi":  modusVivendi(),
	"modus-operandi": modusOperandi(),
	"nightfox":       nightfox(),
	"dayfox":         dayfox(),
	"carbonfox":      carbonfox(),
	"material-ocean": materialOcean(),
	"palenight":      palenight(),
	"oxocarbon":      oxocarbon(),
	"zenburn":        zenburn(),
}

func tokyoNight() Theme {
	return Theme{
		Name:          "tokyonight-night",
		Background:    "#1a1b26",
		Foreground:    "#c0caf5",
		Muted:         "#565f89",
		Accent:        "#7aa2f7",
		Keyword:       "#bb9af7",
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
		Keyword:       "#957fb8",
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
		Keyword:       "#cba6f7",
		Border:        "#45475a",
		Selection:     "#585b70",
		Added:         "#a6e3a1",
		Deleted:       "#f38ba8",
		Error:         "#f38ba8",
		Panel:         "#181825",
		PanelSelected: "#313244",
	}
}

func catppuccinFrappe() Theme {
	return Theme{
		Name:          "catppuccin-frappe",
		Background:    "#303446",
		Foreground:    "#c6d0f5",
		Muted:         "#737994",
		Accent:        "#8caaee",
		Keyword:       "#ca9ee6",
		Border:        "#51576d",
		Selection:     "#626880",
		Added:         "#a6d189",
		Deleted:       "#e78284",
		Error:         "#e78284",
		Panel:         "#292c3c",
		PanelSelected: "#414559",
	}
}

func catppuccinLatte() Theme {
	return Theme{
		Name:              "catppuccin-latte",
		Background:        "#eff1f5",
		Foreground:        "#4c4f69",
		Muted:             "#8c8fa1",
		Accent:            "#1e66f5",
		Keyword:           "#8839ef",
		Border:            "#ccd0da",
		Selection:         "#dce0e8",
		Added:             "#40a02b",
		AddedBackground:   "#d7f3d0",
		Deleted:           "#d20f39",
		DeletedBackground: "#f5d6dc",
		Error:             "#d20f39",
		Panel:             "#e6e9ef",
		PanelSelected:     "#dce0e8",
	}
}

func gruvboxDark() Theme {
	return Theme{
		Name:          "gruvbox-dark",
		Background:    "#282828",
		Foreground:    "#ebdbb2",
		Muted:         "#928374",
		Accent:        "#83a598",
		Keyword:       "#d3869b",
		Border:        "#504945",
		Selection:     "#3c3836",
		Added:         "#b8bb26",
		Deleted:       "#fb4934",
		Error:         "#fb4934",
		Panel:         "#1d2021",
		PanelSelected: "#32302f",
	}
}

func gruvboxLight() Theme {
	return Theme{
		Name:              "gruvbox-light",
		Background:        "#fbf1c7",
		Foreground:        "#3c3836",
		Muted:             "#928374",
		Accent:            "#076678",
		Keyword:           "#8f3f71",
		Border:            "#d5c4a1",
		Selection:         "#ebdbb2",
		Added:             "#79740e",
		AddedBackground:   "#e9edc7",
		Deleted:           "#9d0006",
		DeletedBackground: "#f1d2c2",
		Error:             "#9d0006",
		Panel:             "#f2e5bc",
		PanelSelected:     "#ebdbb2",
	}
}

func solarizedDark() Theme {
	return Theme{
		Name:          "solarized-dark",
		Background:    "#002b36",
		Foreground:    "#839496",
		Muted:         "#586e75",
		Accent:        "#268bd2",
		Keyword:       "#6c71c4",
		Border:        "#073642",
		Selection:     "#073642",
		Added:         "#859900",
		Deleted:       "#dc322f",
		Error:         "#dc322f",
		Panel:         "#073642",
		PanelSelected: "#094352",
	}
}

func solarizedLight() Theme {
	return Theme{
		Name:              "solarized-light",
		Background:        "#fdf6e3",
		Foreground:        "#657b83",
		Muted:             "#93a1a1",
		Accent:            "#268bd2",
		Keyword:           "#6c71c4",
		Border:            "#eee8d5",
		Selection:         "#eee8d5",
		Added:             "#859900",
		AddedBackground:   "#e5edc8",
		Deleted:           "#dc322f",
		DeletedBackground: "#f5d0ca",
		Error:             "#dc322f",
		Panel:             "#eee8d5",
		PanelSelected:     "#e4ddc9",
	}
}

func rosePine() Theme {
	return Theme{
		Name:          "rose-pine",
		Background:    "#191724",
		Foreground:    "#e0def4",
		Muted:         "#6e6a86",
		Accent:        "#c4a7e7",
		Keyword:       "#eb6f92",
		Border:        "#26233a",
		Selection:     "#403d52",
		Added:         "#31748f",
		Deleted:       "#eb6f92",
		Error:         "#eb6f92",
		Panel:         "#1f1d2e",
		PanelSelected: "#26233a",
	}
}

func rosePineDawn() Theme {
	return Theme{
		Name:              "rose-pine-dawn",
		Background:        "#faf4ed",
		Foreground:        "#575279",
		Muted:             "#9893a5",
		Accent:            "#907aa9",
		Keyword:           "#b4637a",
		Border:            "#dfdad9",
		Selection:         "#f2e9e1",
		Added:             "#286983",
		AddedBackground:   "#dcebec",
		Deleted:           "#b4637a",
		DeletedBackground: "#f1d9df",
		Error:             "#b4637a",
		Panel:             "#fffaf3",
		PanelSelected:     "#f2e9e1",
	}
}

func vscodeDark() Theme {
	return Theme{
		Name:          "vscode-dark",
		Background:    "#1e1e1e",
		Foreground:    "#d4d4d4",
		Muted:         "#858585",
		Accent:        "#569cd6",
		Keyword:       "#c586c0",
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
		Keyword:       "#ffae57",
		Border:        "#343d4f",
		Selection:     "#33415e",
		Added:         "#bae67e",
		Deleted:       "#ff3333",
		Error:         "#ff3333",
		Panel:         "#1a1f29",
		PanelSelected: "#2f3b54",
	}
}

func githubDark() Theme {
	return Theme{
		Name:              "github-dark",
		Background:        "#0d1117",
		Foreground:        "#c9d1d9",
		Muted:             "#8b949e",
		Accent:            "#58a6ff",
		Keyword:           "#ff7b72",
		Border:            "#30363d",
		Selection:         "#1f6feb",
		Added:             "#3fb950",
		AddedBackground:   "#033a16",
		Deleted:           "#f85149",
		DeletedBackground: "#490202",
		Error:             "#f85149",
		Panel:             "#161b22",
		PanelSelected:     "#21262d",
	}
}

func githubLight() Theme {
	return Theme{
		Name:              "github-light",
		Background:        "#ffffff",
		Foreground:        "#24292f",
		Muted:             "#57606a",
		Accent:            "#0969da",
		Keyword:           "#cf222e",
		Border:            "#d0d7de",
		Selection:         "#ddf4ff",
		Added:             "#1a7f37",
		AddedBackground:   "#dafbe1",
		Deleted:           "#cf222e",
		DeletedBackground: "#ffebe9",
		Error:             "#cf222e",
		Panel:             "#f6f8fa",
		PanelSelected:     "#eaeef2",
	}
}

func modusVivendi() Theme {
	return Theme{
		Name:          "modus-vivendi",
		Background:    "#000000",
		Foreground:    "#ffffff",
		Muted:         "#989898",
		Accent:        "#00bcff",
		Keyword:       "#feacd0",
		Border:        "#646464",
		Selection:     "#203448",
		Added:         "#44bc44",
		Deleted:       "#ff5f59",
		Error:         "#ff5f59",
		Panel:         "#1e1e1e",
		PanelSelected: "#2a2a2a",
	}
}

func modusOperandi() Theme {
	return Theme{
		Name:              "modus-operandi",
		Background:        "#ffffff",
		Foreground:        "#000000",
		Muted:             "#595959",
		Accent:            "#0031a9",
		Keyword:           "#721045",
		Border:            "#c4c4c4",
		Selection:         "#d8eaff",
		Added:             "#006800",
		AddedBackground:   "#dff6dd",
		Deleted:           "#a60000",
		DeletedBackground: "#ffd8d8",
		Error:             "#a60000",
		Panel:             "#f2f2f2",
		PanelSelected:     "#e6e6e6",
	}
}

func nightfox() Theme {
	return Theme{
		Name:          "nightfox",
		Background:    "#192330",
		Foreground:    "#cdcecf",
		Muted:         "#71839b",
		Accent:        "#719cd6",
		Keyword:       "#c94f6d",
		Border:        "#39506d",
		Selection:     "#2b3b51",
		Added:         "#81b29a",
		Deleted:       "#c94f6d",
		Error:         "#c94f6d",
		Panel:         "#131a24",
		PanelSelected: "#212e3f",
	}
}

func dayfox() Theme {
	return Theme{
		Name:              "dayfox",
		Background:        "#f6f2ee",
		Foreground:        "#3d2b5a",
		Muted:             "#8a739a",
		Accent:            "#287980",
		Keyword:           "#955f61",
		Border:            "#d8d0c7",
		Selection:         "#e7d2be",
		Added:             "#396847",
		AddedBackground:   "#dce8d9",
		Deleted:           "#a5222f",
		DeletedBackground: "#ecd6d8",
		Error:             "#a5222f",
		Panel:             "#eee6de",
		PanelSelected:     "#e7d2be",
	}
}

func carbonfox() Theme {
	return Theme{
		Name:          "carbonfox",
		Background:    "#161616",
		Foreground:    "#f2f4f8",
		Muted:         "#6f6f6f",
		Accent:        "#78a9ff",
		Keyword:       "#ff7eb6",
		Border:        "#393939",
		Selection:     "#2a2a2a",
		Added:         "#42be65",
		Deleted:       "#ee5396",
		Error:         "#ee5396",
		Panel:         "#0f0f0f",
		PanelSelected: "#262626",
	}
}

func materialOcean() Theme {
	return Theme{
		Name:          "material-ocean",
		Background:    "#0f111a",
		Foreground:    "#a6accd",
		Muted:         "#676e95",
		Accent:        "#82aaff",
		Keyword:       "#c792ea",
		Border:        "#292d3e",
		Selection:     "#1f2233",
		Added:         "#c3e88d",
		Deleted:       "#ff5370",
		Error:         "#ff5370",
		Panel:         "#090b10",
		PanelSelected: "#1a1c25",
	}
}

func palenight() Theme {
	return Theme{
		Name:          "palenight",
		Background:    "#292d3e",
		Foreground:    "#a6accd",
		Muted:         "#676e95",
		Accent:        "#82aaff",
		Keyword:       "#c792ea",
		Border:        "#444267",
		Selection:     "#34324a",
		Added:         "#c3e88d",
		Deleted:       "#f07178",
		Error:         "#f07178",
		Panel:         "#202331",
		PanelSelected: "#34324a",
	}
}

func oxocarbon() Theme {
	return Theme{
		Name:          "oxocarbon",
		Background:    "#161616",
		Foreground:    "#f2f4f8",
		Muted:         "#7b7c7e",
		Accent:        "#33b1ff",
		Keyword:       "#ff7eb6",
		Border:        "#393939",
		Selection:     "#262626",
		Added:         "#25be6a",
		Deleted:       "#ee5396",
		Error:         "#ee5396",
		Panel:         "#0f0f0f",
		PanelSelected: "#262626",
	}
}

func zenburn() Theme {
	return Theme{
		Name:          "zenburn",
		Background:    "#3f3f3f",
		Foreground:    "#dcdccc",
		Muted:         "#7f9f7f",
		Accent:        "#8cd0d3",
		Keyword:       "#dc8cc3",
		Border:        "#5f5f5f",
		Selection:     "#4f4f4f",
		Added:         "#7f9f7f",
		Deleted:       "#cc9393",
		Error:         "#cc9393",
		Panel:         "#383838",
		PanelSelected: "#4f4f4f",
	}
}
