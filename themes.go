package progress

// https://imagemagick.org/color/#color_names
// https://en.wikipedia.org/wiki/X11_color_names
//
// $ magick -list color
// $ magick xc:'rgb(178,34,34)' -depth 8 txt:-

// themeRegistry holds the isolated configuration lookup engine.
type themeRegistry struct { get func(name string) *theme }

// newThemeRegistry initializes the read-only templates within a single execution block and returns an isolated closure function.
func newThemeRegistry() *themeRegistry {

	blackToWhite := theme{
		name:   "blackToWhite",
		colors: []rgb{
			{  0,   0,   0}, // black
			{255, 255, 255}, // white
		},
	}

	blueToRed := theme{
		name:   "blueToRed",
		colors: []rgb{
			{  0, 0, 255}, // blue
			{255, 0,   0}, // red
		},
	}

	blueToGreen := theme{
		name:   "blueToGreen",
		colors: []rgb{
			{0,   0, 255}, // blue
			{0, 255,   0}, // green
		},
	}

	greenToBlue := theme{
		name:        "greenToBlue",
		colors: []rgb{
			{0, 255,   0}, // green
			{0,   0, 255}, // blue
		},
	}

	greenToYellow := theme{
		name:   "greenToYellow",
		colors: []rgb{
			{  0, 255, 0}, // green
			{255, 255, 0}, // yellow
		},
	}

	fire := theme{
		name:   "fire",
		colors: []rgb{
			{255,   0, 0}, // red
			{255, 255, 0}, // yellow
		},
	}

	thermal := theme{
		name:   "thermal",
		colors: []rgb{
			{  0,   0,  50}, // dark blue
			{150,   0, 150}, // purple
			{255,  70,   0}, // orange
			{255, 220,   0}, // pale yellow
			{255, 255, 255}, // white
		},
	}

	sunset := theme{
		name:   "sunset",
		colors: []rgb{
			{ 48,  25, 52}, // dark plum
			{199,   0, 57}, // crimson
			{255,  87, 51}, // dark coral
			{255, 195,  0}, // peach gold
		},
	}

	ocean := theme{
		name:   "ocean",
		colors: []rgb{
			{  0,  10,  45}, // navy blue
			{  0, 128, 128}, // teal
			{  0, 200, 150}, // turquoise
			{100, 255, 150}, // seafoam green
		},
	}

	rainbow := theme{
		name:   "rainbow",
		colors: []rgb{
			{  0,   0, 255}, // blue
			{  0, 255,   0}, // green
			{255, 255,   0}, // yellow
			{255, 127,   0}, // orange
			{255,   0,   0}, // red
		},
	}

	rainbow2 := theme{ // this variant renders a higher proportion of green gradients in the middle of its span
		name:   "rainbow2",
		colors: []rgb{
			{  0,   0, 255}, // blue
			{  0, 255,   0}, // green
			{255, 255,   0}, // yellow
			{255,   0,   0}, // red
		},
	}

	retro := theme{
		name:   "retro",
		colors: []rgb{
			{ 55,   0, 255}, // indigo
			{255,   0, 128}, // magenta
			{255,   0, 255}, // pink
			{  0, 255, 255}, // cyan
		},
	}

	vaporwave := theme{
		name:   "vaporwave",
		colors: []rgb{
			{ 30, 220, 170}, // greenish turquoise
			{130, 180, 255}, // pastel cornflower bluw
			{255, 130, 210}, // bubblegum pink
		},
	}

	toxic := theme{
		name:   "toxic",
		colors: []rgb{
			{ 75,   0, 130}, // violet
			{148,   0, 211}, // purple
			{ 50, 205,  50}, // lime green
			{173, 255,  47}, // green-yellow
		},
	}

	trans := theme{
		name:   "trans",
		colors: []rgb{
			{ 91, 206, 250}, // light blue
			{245, 169, 184}, // pink
			{255, 255, 255}, // white
			{245, 169, 184}, // pink
			{ 91, 206, 250}, // light blue
		},
	}

	pride := theme{
		name:   "pride",
		colors: []rgb{
			{228,   3,   3}, // red
			{255, 140,   0}, // orange
			{255, 237,   0}, // yellow
			{  0, 128,  38}, // green
			{  0,  76, 255}, // blue
			{117,   7, 135}, // violet
		},
	}

	bi := theme{
		name:   "bi",
		colors: []rgb{
			{214,  2, 112}, // magenta
			{155, 79, 150}, // purple
			{  0, 56, 168}, // royal blue
		},
	}

	pan := theme{
		name:   "pan",
		colors: []rgb{
			{255,  27, 141}, // hot pink
			{255, 216,   0}, // canary yellow
			{  1, 179, 247}, // sky cyan
		},
	}

	matrix := theme{
		name:   "matrix",
		colors: []rgb{
			{ 0,  40,  0}, // dark forest green
			{ 0, 140, 40}, // kelly green
			{50, 255, 50}, // neon green
		},
	}

	glacier := theme{
		name:   "glacier",
		colors: []rgb{
			{  0,  30,  60}, // midnight blue
			{  0, 210, 255}, // sky blue
			{230, 250, 255}, // pale ice blue
		},
	}

	autumn := theme{
		name:   "autumn",
		colors: []rgb{
			{ 34,  76, 34}, // forest green
			{218, 145,  0}, // ochre
			{210,  60,  0}, // burnt orange
		},
	}

	cyberpunk := theme{
		name:   "cyberpunk",
		colors: []rgb{
			{ 10,  15, 45}, // navy blue
			{255,   0, 85}, // neon red
			{243, 231,  0}, // canary yellow
		},
	}

	magma := theme{
		name:   "magma",
		colors: []rgb{
			{ 20,   0, 25}, // deep plum
			{210,  10,  0}, // scarlet
			{255, 170,  0}, // amber
		},
	}

	nebula := theme{
		name:   "nebula",
		colors: []rgb{
			{ 10,   0,  80}, // midnight blue
			{180,   0, 180}, // dark magenta
			{  0, 230, 255}, // bright cyan
		},
	}

	hazard := theme{
		name:   "hazard",
		colors: []rgb{
			{255, 210,  0}, // marigold yellow
			{255,  85,  0}, // neon orange
			{ 25,  25, 25}, // charcoal
		},
	}

	coffee := theme{
		name:   "coffee",
		colors: []rgb{
			{ 45,  25,  15}, // dark coffee
			{150,  90,  40}, // copper brown
			{240, 210, 170}, // navajo white
		},
	}

	arcade := theme{
		name:        "arcade",
		colors: []rgb{
			{255,   0, 128}, // vivid rose
			{140,   0, 255}, // electric indigo
			{  0,  70, 255}, // neon blue
			{  0, 255, 230}, // turquoise
			{ 50, 255,   0}, // lime green
			{255, 230,   0}, // canary yellow
		},
	}

	prism := theme{
		name:   "prism",
		colors: []rgb{
			{255, 180, 255}, // pastel pink
			{180, 190, 255}, // periwinkle
			{170, 255, 220}, // mint green
			{255, 255, 160}, // canary yellow
			{255, 200, 160}, // peach
			{255, 160, 190}, // bubblegum pink
		},
	}

	biohazard := theme{
		name:   "biohazard",
		colors: []rgb{
			{  0, 255,  68}, // spring green
			{212, 255,   0}, // chartreuse
			{255, 110,   0}, // orange
			{120,   0, 200}, // dark violet
			{255,   0, 180}, // neon pink
		},
	}

	supernova := theme{
		name:   "supernova",
		colors: []rgb{
			{  5,   5,  40}, // midnight blue
			{ 80,   0, 120}, // deep purple
			{230,   0, 130}, // raspberry
			{255,  40,   0}, // scarlet
			{255, 130,   0}, // tangerine
			{255, 230,  60}, // pastel yellow
			{255, 255, 255}, // white
		},
	}

	psychadelic := theme{
		name:   "psychadelic",
		colors: []rgb{
			{ 10,   0,  30}, // midnight blue
			{255,   0, 128}, // vivid rose
			{  0, 255, 242}, // bright turquoise
			{ 50, 255,   0}, // lime green
			{255, 215,   0}, // gold
			{255,   0,  40}, // bright crimson
			{138,  43, 226}, // blue violet
			{  0,  71, 255}, // neon blue
			{255, 102,   0}, // orange
			{  0, 255, 150}, // spring green
			{255,   0, 210}, // neon magenta
			{255,  60,   0}, // scarlet
			{255, 255, 255}, // white
		},
	}

	registryMap := map[string]*theme{
		"blackToWhite":  &blackToWhite,
		"blueToRed":     &blueToRed,
		"blueToGreen":   &blueToGreen,
		"greenToBlue":   &greenToBlue,
		"greenToYellow": &greenToYellow,
		"fire":          &fire,
		"thermal":       &thermal,
		"sunset":        &sunset,
		"ocean":         &ocean,
		"rainbow":       &rainbow,
		"rainbow2":      &rainbow2,
		"retro":         &retro,
		"vaporwave":     &vaporwave,
		"toxic":         &toxic,
		"trans":         &trans,
		"pride":         &pride,
		"bi":            &bi,
		"pan":           &pan,
		"matrix":        &matrix,
		"glacier":       &glacier,
		"autumn":        &autumn,
		"cyberpunk":     &cyberpunk,
		"magma":         &magma,
		"nebula":        &nebula,
		"hazard":        &hazard,
		"coffee":        &coffee,
		"arcade":        &arcade,
		"prism":         &prism,
		"biohazard":     &biohazard,
		"supernova":     &supernova,
		"psychadelic":   &psychadelic,
	}

	return &themeRegistry{
		get: func(name string) *theme {
			if theme, exists := registryMap[name]; exists { return theme }
			return &sunset
		},
	}
}
