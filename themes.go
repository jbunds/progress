package progress

// themeRegistry holds the isolated configuration lookup engine.
type themeRegistry struct { get func(name string) *theme }

// newThemeRegistry initializes the read-only templates within a single execution block and returns an isolated closure function.
func newThemeRegistry() *themeRegistry {

	blackToWhite := theme{
		name:        "blackToWhite",
		transitions: []endpoints{
			{initial: rgb{r: 0, g: 0, b: 0}, final: rgb{r: 255, g: 255, b: 255}}, // black -> white
		},
	}

	blueToRed := theme{
		name:        "blueToRed",
		transitions: []endpoints{
			{initial: rgb{r: 0, g: 0, b: 255}, final: rgb{r: 255, g: 0, b: 0}}, // blue -> purple -> red
		},
	}

	blueToGreen := theme{
		name:        "blueToGreen",
		transitions: []endpoints{
			{initial: rgb{r: 0, g: 0, b: 255}, final: rgb{r: 0, g: 255, b: 0}}, // blue -> green
		},
	}

	greenToBlue := theme{
		name:        "greenToBlue",
		transitions: []endpoints{
			{initial: rgb{r: 0, g: 255, b: 0}, final: rgb{r: 0, g: 0, b: 255}}, // green -> blue
		},
	}

	greenToYellow := theme{
		name:        "greenToYellow",
		transitions: []endpoints{
			{initial: rgb{r: 0, g: 255, b: 0}, final: rgb{r: 255, g: 255, b: 0}}, // green -> yellow
		},
	}

	fire := theme{
		name:        "fire",
		transitions: []endpoints{
			{initial: rgb{r: 255, g: 0, b: 0}, final: rgb{r: 255, g: 255, b: 0}}, // red -> orange -> yellow
		},
	}

	thermal := theme{
		name:        "thermal",
		transitions: []endpoints{
			{initial: rgb{r:   0, g:   0, b:  50}, final: rgb{r: 150, g:   0, b: 150}}, // dark blue   -> purple
			{initial: rgb{r: 150, g:   0, b: 150}, final: rgb{r: 255, g:  70, b:   0}}, // purple      -> orange
			{initial: rgb{r: 255, g:  70, b:   0}, final: rgb{r: 255, g: 220, b:   0}}, // orange      -> pale yellow
			{initial: rgb{r: 255, g: 220, b:   0}, final: rgb{r: 255, g: 255, b: 255}}, // pale yellow -> white
		},
	}

	sunset := theme{
		name:        "sunset",
		transitions: []endpoints{
			{initial: rgb{r:  48, g:  25, b:  52}, final: rgb{r: 199, g:   0, b:  57}}, // dark plum  -> crimson
			{initial: rgb{r: 199, g:   0, b:  57}, final: rgb{r: 255, g:  87, b:  51}}, // crimson    -> dark coral
			{initial: rgb{r: 255, g:  87, b:  51}, final: rgb{r: 255, g: 195, b:   0}}, // dark coral -> peach gold
		},
	}

	ocean := theme{
		name:        "ocean",
		transitions: []endpoints{
			{initial: rgb{r:   0, g:  10, b:  45}, final: rgb{r:   0, g: 128, b: 128}}, // navy blue -> teal
			{initial: rgb{r:   0, g: 128, b: 128}, final: rgb{r:   0, g: 200, b: 150}}, // teal      -> turquoise
			{initial: rgb{r:   0, g: 200, b: 150}, final: rgb{r: 100, g: 255, b: 150}}, // turquoise -> seafoam green
		},
	}

	rainbow := theme{
		name:        "rainbow",
		transitions: []endpoints{
			{initial: rgb{r:   0, g:   0, b: 255}, final: rgb{r:   0, g: 255, b: 0}}, // blue   -> green
			{initial: rgb{r:   0, g: 255, b:   0}, final: rgb{r: 255, g: 255, b: 0}}, // green  -> yellow
			{initial: rgb{r: 255, g: 255, b:   0}, final: rgb{r: 255, g: 127, b: 0}}, // yellow -> orange
			{initial: rgb{r: 255, g: 127, b:   0}, final: rgb{r: 255, g:   0, b: 0}}, // orange -> red
		},
	}

	rainbow2 := theme{ // this variant renders a higher proportion of green gradients in the middle of its span
		name:        "rainbow2",
		transitions: []endpoints{
			{initial: rgb{r:   0, g:   0, b: 255}, final: rgb{r:   0, g: 255, b: 0}}, // blue   -> green
			{initial: rgb{r:   0, g: 255, b:   0}, final: rgb{r: 255, g: 255, b: 0}}, // green  -> yellow
			{initial: rgb{r: 255, g: 255, b:   0}, final: rgb{r: 255, g:   0, b: 0}}, // yellow -> red
		},
	}

	retro := theme{
		name:        "retro",
		transitions: []endpoints{
			{initial: rgb{r:  55, g: 0, b: 255}, final: rgb{r: 255, g:   0, b: 128}}, // indigo  -> magenta
			{initial: rgb{r: 255, g: 0, b: 128}, final: rgb{r: 255, g:   0, b: 255}}, // magenta -> pink
			{initial: rgb{r: 255, g: 0, b: 255}, final: rgb{r:   0, g: 255, b: 255}}, // pink    -> cyan
		},
	}

	toxic := theme{
		name:        "toxic",
		transitions: []endpoints{
			{initial: rgb{r:  75, g:   0, b: 130}, final: rgb{r: 148, g:   0, b: 211}}, // violet     -> purple
			{initial: rgb{r: 148, g:   0, b: 211}, final: rgb{r:  50, g: 205, b:  50}}, // purple     -> lime green
			{initial: rgb{r:  50, g: 205, b:  50}, final: rgb{r: 173, g: 255, b:  47}}, // lime green -> green-yellow
		},
	}

	trans := theme{
		name:        "trans",
		transitions: []endpoints{
			{initial: rgb{r:  91, g: 206, b: 250}, final: rgb{r: 245, g: 169, b: 184}}, // light blue -> pink
			{initial: rgb{r: 245, g: 169, b: 184}, final: rgb{r: 255, g: 255, b: 255}}, // pink       -> white
			{initial: rgb{r: 255, g: 255, b: 255}, final: rgb{r: 245, g: 169, b: 184}}, // white      -> pink
			{initial: rgb{r: 245, g: 169, b: 184}, final: rgb{r:  91, g: 206, b: 250}}, // pink       -> light blue
		},
	}

	pride := theme{
		name:        "pride",
		transitions: []endpoints{
			{initial: rgb{r: 228, g:   3, b:   3}, final: rgb{r: 255, g: 140, b:   0}}, // red    -> orange
			{initial: rgb{r: 255, g: 140, b:   0}, final: rgb{r: 255, g: 237, b:   0}}, // orange -> yellow
			{initial: rgb{r: 255, g: 237, b:   0}, final: rgb{r:   0, g: 128, b:  38}}, // yellow -> green
			{initial: rgb{r:   0, g: 128, b:  38}, final: rgb{r:   0, g:  76, b: 255}}, // green  -> blue
			{initial: rgb{r:   0, g:  76, b: 255}, final: rgb{r: 117, g:   7, b: 135}}, // blue   -> violet
		},
	}

	bi := theme{
		name:        "bi",
		transitions: []endpoints{
			{initial: rgb{r: 214, g:   2, b: 112}, final: rgb{r: 155, g:  79, b: 150}}, // magenta -> purple
			{initial: rgb{r: 155, g:  79, b: 150}, final: rgb{r:   0, g:  56, b: 168}}, // purple  -> royal blue
		},
	}

	pan := theme{
		name:        "pan",
		transitions: []endpoints{
			{initial: rgb{r: 255, g:  27, b: 141}, final: rgb{r: 255, g: 216, b:   0}}, // hot pink -> canary yellow
			{initial: rgb{r: 255, g: 216, b:   0}, final: rgb{r:   1, g: 179, b: 247}}, // yellow   -> sky cyan
		},
	}

	matrix := theme{
		name:        "matrix",
		transitions: []endpoints{
			{initial: rgb{r: 0, g:  20, b:  0}, final: rgb{r:  0, g: 140, b: 40}},
			{initial: rgb{r: 0, g: 140, b: 40}, final: rgb{r: 50, g: 255, b: 50}},
		},
	}

	glacier := theme{
		name:        "glacier",
		transitions: []endpoints{
			{initial: rgb{r: 0, g:  30, b:  60}, final: rgb{r:   0, g: 210, b: 255}},
			{initial: rgb{r: 0, g: 210, b: 255}, final: rgb{r: 230, g: 250, b: 255}},
		},
	}

	autumn := theme{
		name:        "autumn",
		transitions: []endpoints{
			{initial: rgb{r:  34, g:  76, b: 34}, final: rgb{r: 218, g: 145, b: 0}},
			{initial: rgb{r: 218, g: 145, b:  0}, final: rgb{r: 210, g:  60, b: 0}},
		},
	}

	cyberpunk := theme{
		name:        "cyberpunk",
		transitions: []endpoints{
			{initial: rgb{r:  10, g: 15, b: 45}, final: rgb{r: 255, g:   0, b: 85}},
			{initial: rgb{r: 255, g:  0, b: 85}, final: rgb{r: 243, g: 231, b:  0}},
		},
	}

	magma := theme{
		name:        "magma",
		transitions: []endpoints{
			{initial: rgb{r:  20, g:  0, b: 25}, final: rgb{r: 210, g:  10, b: 0}},
			{initial: rgb{r: 210, g: 10, b:  0}, final: rgb{r: 255, g: 170, b: 0}},
		},
	}

	nebula := theme{
		name:        "nebula",
		transitions: []endpoints{
			{initial: rgb{r:  10, g: 0, b:  80}, final: rgb{r: 180, g:   0, b: 180}},
			{initial: rgb{r: 180, g: 0, b: 180}, final: rgb{r:   0, g: 230, b: 255}},
		},
	}

	hazard := theme{
		name:        "hazard",
		transitions: []endpoints{
			{initial: rgb{r: 255, g: 210, b: 0}, final: rgb{r: 255, g: 85, b:  0}},
			{initial: rgb{r: 255, g:  85, b: 0}, final: rgb{r:  25, g: 25, b: 25}},
		},
	}

	vaporwave := theme{
		name:        "vaporwave",
		transitions: []endpoints{
			{initial: rgb{r:  30, g: 220, b: 170}, final: rgb{r: 130, g: 180, b: 255}},
			{initial: rgb{r: 130, g: 180, b: 255}, final: rgb{r: 255, g: 130, b: 210}},
		},
	}

	coffee := theme{
		name:        "coffee",
		transitions: []endpoints{
			{initial: rgb{r:  45, g: 25, b: 15}, final: rgb{r: 150, g:  90, b:  40}},
			{initial: rgb{r: 150, g: 90, b: 40}, final: rgb{r: 240, g: 210, b: 170}},
		},
	}

	arcade := theme{
		name:        "arcade",
		transitions: []endpoints{
			{initial: rgb{r: 255, g:   0, b: 128}, final: rgb{r: 140, g:   0, b: 255}},
			{initial: rgb{r: 140, g:   0, b: 255}, final: rgb{r:   0, g:  70, b: 255}},
			{initial: rgb{r:   0, g:  70, b: 255}, final: rgb{r:   0, g: 255, b: 230}},
			{initial: rgb{r:   0, g: 255, b: 230}, final: rgb{r:  50, g: 255, b:   0}},
			{initial: rgb{r:  50, g: 255, b:   0}, final: rgb{r: 255, g: 230, b:   0}},
		},
	}

	prism := theme{
		name:        "prism",
		transitions: []endpoints{
			{initial: rgb{r: 255, g: 180, b: 255}, final: rgb{r: 180, g: 190, b: 255}},
			{initial: rgb{r: 180, g: 190, b: 255}, final: rgb{r: 170, g: 255, b: 220}},
			{initial: rgb{r: 170, g: 255, b: 220}, final: rgb{r: 255, g: 255, b: 160}},
			{initial: rgb{r: 255, g: 255, b: 160}, final: rgb{r: 255, g: 200, b: 160}},
			{initial: rgb{r: 255, g: 200, b: 160}, final: rgb{r: 255, g: 160, b: 190}},
		},
	}

	biohazard := theme{
		name:        "biohazard",
		transitions: []endpoints{
			{initial: rgb{r:   0, g: 255, b:  68}, final: rgb{r: 212, g: 255, b:   0}},
			{initial: rgb{r: 212, g: 255, b:   0}, final: rgb{r: 255, g: 110, b:   0}},
			{initial: rgb{r: 255, g: 110, b:   0}, final: rgb{r: 120, g:   0, b: 200}},
			{initial: rgb{r: 120, g:   0, b: 200}, final: rgb{r: 255, g:   0, b: 180}},
		},
	}

	supernova := theme{
		name:        "supernova",
		transitions: []endpoints{
			{initial: rgb{r:   5, g:   5, b:  40}, final: rgb{r:  80, g:   0, b: 120}},
			{initial: rgb{r:  80, g:   0, b: 120}, final: rgb{r: 230, g:   0, b: 130}},
			{initial: rgb{r: 230, g:   0, b: 130}, final: rgb{r: 255, g:  40, b:   0}},
			{initial: rgb{r: 255, g:  40, b:   0}, final: rgb{r: 255, g: 130, b:   0}},
			{initial: rgb{r: 255, g: 130, b:   0}, final: rgb{r: 255, g: 230, b:  60}},
			{initial: rgb{r: 255, g: 230, b:  60}, final: rgb{r: 255, g: 255, b: 255}},
		},
	}

	hyperdrive := theme{
		name:        "hyperdrive",
		transitions: []endpoints{
			{initial: rgb{r:  10, g:   0, b:  30}, final: rgb{r: 255, g:   0, b: 128}},
			{initial: rgb{r: 255, g:   0, b: 128}, final: rgb{r:   0, g: 255, b: 242}},
			{initial: rgb{r:   0, g: 255, b: 242}, final: rgb{r:  50, g: 255, b:   0}},
			{initial: rgb{r:  50, g: 255, b:   0}, final: rgb{r: 255, g: 215, b:   0}},
			{initial: rgb{r: 255, g: 215, b:   0}, final: rgb{r: 255, g:   0, b:  40}},
			{initial: rgb{r: 255, g:   0, b:  40}, final: rgb{r: 138, g:  43, b: 226}},
			{initial: rgb{r: 138, g:  43, b: 226}, final: rgb{r:   0, g:  71, b: 255}},
			{initial: rgb{r:   0, g:  71, b: 255}, final: rgb{r: 255, g: 102, b:   0}},
			{initial: rgb{r: 255, g: 102, b:   0}, final: rgb{r:   0, g: 255, b: 150}},
			{initial: rgb{r:   0, g: 255, b: 150}, final: rgb{r: 255, g:   0, b: 210}},
			{initial: rgb{r: 255, g:   0, b: 210}, final: rgb{r: 255, g:  60, b:   0}},
			{initial: rgb{r: 255, g:  60, b:   0}, final: rgb{r: 255, g: 255, b: 255}},
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
		"vaporwave":     &vaporwave,
		"coffee":        &coffee,
		"arcade":        &arcade,
		"prism":         &prism,
		"biohazard":     &biohazard,
		"supernova":     &supernova,
		"hyperdrive":    &hyperdrive,
	}

	return &themeRegistry{
		get: func(name string) *theme {
			if theme, exists := registryMap[name]; exists { return theme }
			return &sunset
		},
	}
}
