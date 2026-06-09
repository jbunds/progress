package progress

import (
	"math"
	"slices"
)

// https://jakob-bagterp.github.io/colorist-for-python/ansi-escape-codes/rgb-colors/

// rgb represents a 24-bit color triplet.
type rgb struct { r, g, b uint8 }

// theme represents a named sequential color gradient.
type theme struct {
	name   string
	colors []rgb // sequence of RGB colors to be interpolated
}

// bgColor calculates the color at a specific normalized column fraction (0.0 to 1.0).
func (t *theme) bgColor(fraction float64) rgb {
	numColors := len(t.colors)
	if numColors == 0   { return rgb{} }
	if numColors == 1   { return t.colors[0] }
	if fraction  <= 0.0 { return t.colors[0] }
	if fraction  >= 1.0 { return t.colors[numColors - 1] }

	// calculate which continuous stage boundary the position current occupies
	numSegments    := numColors - 1
	globalStage    := fraction * float64(numSegments)
	floorStage     := math.Floor(globalStage)
	stageIdx       := max(0, min(int(floorStage), numSegments - 1))
	scaledFraction := int((globalStage - floorStage) * float64(math.MaxInt16 + 1) + 0.5)
	startColor     := t.colors[stageIdx]
	endColor       := t.colors[stageIdx + 1]

	lerp := func(start, end uint8) uint8 { // high-res 15-bit fixed-point linear interpolation (lerp) with accurate rounding
		s, e := int(start), int(end)
		// 1. (s << 15) shifts the start color to a matching 15-bit fixed-point scale
		// 2. (e - s) * scaledFraction calculates the exact distance delta using a 0-2^15 integer scale
		// 3. adding 2^14 (half of 2^15) ensures accurate rounding
		// 4. >> 15 divides by 2^15 to shift back to standard 8-bit integer space (0-255)
		val := (s << 15 + (e - s) * scaledFraction + int(math.MaxInt16 + 1) / 2) >> 15
		return uint8(val & 0xFF) // satisfy gosec
	}

	return rgb{
		lerp(startColor.r, endColor.r),
		lerp(startColor.g, endColor.g),
		lerp(startColor.b, endColor.b),
	}
}

// fgColor returns an optimized, lookup-table-based function which calculates high-contrast foreground colors.
func fgColor() func(c rgb) rgb {
	var contrastLUT [256]uint8
	const threshold = 0.35 // foreground grayscale inflection threshold: the lower the value, the earlier the transition will occur
	const power     = 24.0 // foreground grayscale inflection exponent: the higher the value, the sharper the transition point
	for i := range 256 {
		normalizedLuminance := (float64(uint8(i)) * 10000.0) / 2550000.0 // normalize luminance to a 0.0 -> 1.0 spectrum; max luminance is 255 * 10000 == 2,550,000
		var biasedLuminance float64 // non-linear background brightness contrast scaling factor
		if normalizedLuminance < threshold {
			biasedLuminance = math.Pow(normalizedLuminance / threshold, power) * threshold // transition to white for dark backgrounds
		} else {
			biasedLuminance = 1 - (math.Pow((1 - normalizedLuminance) / (1 - threshold), power) * (1 - threshold)) // transition to black immediately past the threshold
		}
		val           := 255 * (1 - biasedLuminance) + 0.5
		contrastLUT[i] = uint8(int(val) & 0xFF)
	}
	return func(c rgb) rgb {
		// fast, integer-based approximation of the https://www.w3.org/TR/WCAG20-TECHS/G17.html#G17-tests W3C formula (skips gamma expansion):
		//   R: 0.2126 * 10000 ~= 2126
		//   G: 0.7152 * 10000 ~= 7152
		//   B: 0.0722 * 10000 ~=  722
		luminance := (int(c.r) * 2126) + (int(c.g) * 7152) + (int(c.b) * 722)
		lumIdx    := (luminance / 10000) & 0xFF
		color     := contrastLUT[lumIdx]
		return rgb{color, color, color}
	}
}

func (c rgb) Equal(other rgb) bool { // work around cmp's draconian strictures
	return c.r == other.r &&
	       c.g == other.g &&
	       c.b == other.b
}

func (t *theme) Equal(other *theme) bool { // work around cmp's draconian strictures
	if t == nil || other == nil { return t == other }
	return t.name == other.name && slices.Equal(t.colors, other.colors)
}
