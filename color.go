package progress

import (
	"math"
	"slices"
)

// https://jakob-bagterp.github.io/colorist-for-python/ansi-escape-codes/rgb-colors/

const (
	threshold = 0.35 // foreground grey-scale inflection threshold: the lower the value, the earlier the transition will be triggered
	power     = 24.0 // foreground grey-scale inflection exponent: the higher the value, the sharper the transition point
)

// rgb represents a 24-bit color triplet.
type rgb struct { r, g, b int } // TODO(jeff): investigate s/int/uint8/ to decrease wasted memory

// theme defines a named color gradient composed of sequential color transitions.
type theme struct {
	name   string
	colors []rgb // sequential color stops
}

// bgColor calculates the color at a specific normalized column fraction (0.0 to 1.0).
func (t *theme) bgColor(fraction float64) rgb {
	numColors := len(t.colors)
	if numColors == 0   { return rgb{} }
	if numColors == 1   { return t.colors[0] }
	if fraction  <= 0.0 { return t.colors[0] }
	if fraction  >= 1.0 { return t.colors[numColors - 1] }


	// calculate which continuous stage boundary the position current occupies
	numSegments := numColors - 1
	globalStage := fraction * float64(numSegments)
	stageIdx    := int(math.Floor(globalStage))

	if stageIdx >= numSegments { stageIdx = numSegments - 1 } // prevent floating-point rounding edge cases from out-of-bounds index panic

	// extract the pure local interpolation fraction inside this specific stage slice
	localT := globalStage - float64(stageIdx)
	switch {
	case localT > 1.0: localT = 1.0
	case localT < 0.0: localT = 0.0
	}

	// read adjacent keyframes directly from the single array slice
	startColor := t.colors[stageIdx]
	endColor   := t.colors[stageIdx + 1]

	lerp := func(start, end int) int { // high-res 24-bit linear interpolation with accurate rounding
		s, e := float64(start), float64(end)
		return int(math.Floor(s + localT*(e - s) + 0.5))
	}

	return rgb{
		r: lerp(startColor.r, endColor.r),
		g: lerp(startColor.g, endColor.g),
		b: lerp(startColor.b, endColor.b),
	}
}

// fgColor calculates the perceptual luminance of the given background color and returns a high-contrast, grey-scale foreground color.
func (c rgb) fgColor() rgb {
	// W3C formula (https://www.w3.org/TR/WCAG20-TECHS/G17.html#G17-tests) using fast, integer-based scaling to avoid float bottlenecks:
	//   0.2126 * 10000 ~= 2126
	//   0.7152 * 10000 ~= 7152
	//   0.0722 * 10000 ~=  722
	luminance           := (c.r * 2126) + (c.g * 7152) + (c.b * 722)
	normalizedLuminance := float64(luminance) / 2550000.0 // normalize luminance to a strict 0.0 -> 1.0 spectrum; maximum possible luminance is 255 * 10000 == 2,550,000
	var biasedLuminance float64
	if normalizedLuminance < threshold {
		biasedLuminance = math.Pow(normalizedLuminance / threshold, power) * threshold // transition to white for dark backgrounds
	} else {
		biasedLuminance = 1.0 - (math.Pow((1.0 - normalizedLuminance) / (1.0 - threshold), power) * (1.0 - threshold)) // rapidly transition to black immediately past the threshold
	}
	// interpolate each channel linearly between white and black
	// color == 255 + biasedLuminance * (0 - 255) => 255 * (1.0 - biasedLuminance)
	color := int(math.Floor(255.0 * (1.0 - biasedLuminance) + 0.5))
	if color > 255 {
		color = 255
	} else if color < 0 {
		color = 0
	}
	return rgb{r: color, g: color, b: color}
}

func (t *theme) Equal(other *theme) bool { // work around cmp's draconian strictures
	if t == nil || other == nil { return t == other }
	return t.name == other.name && slices.Equal(t.colors, other.colors)
}
