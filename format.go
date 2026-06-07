package progress

import (
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// appendRGBInline writes a stringified integer directly into a byte slice without heap allocation.
// optimized for RGB channel and terminal column ranges (0-255, 0-999).
func appendRGBInline(b []byte, u uint8) []byte {
	switch {
	case u < 10:
		return append(b, byte('0' + u))
	case u < 100:
		q := u / 10
		r := u - (q * 10)
		return append(b, byte('0' + q), byte('0' + r))
	default: // 100 to 255
		q1  :=   u / 100
		rem :=   u - (q1 * 100)
		q2  := rem /  10
		r   := rem - (q2 * 10)
		return append(b, byte('0' + q1), byte('0' + q2), byte('0' + r))
	}
}

// appendRune is a fast, zero-allocation inline implementation of utf8.EncodeRune.
func appendRune(p []byte, r rune) []byte {
	u := uint32(r & 0x7FFFFFFF)
	// guard against invalid unicode surrogates and out-of-bounds code points (max U+10FFFF)
	// by mapping to the standard surrogate replacement character
	if (u >= 0xD800 && u <= 0xDFFF) || u > 0x10FFFF { return append(p, 0xEF, 0xBF, 0xBD) }
	switch {          // https://en.wikipedia.org/wiki/UTF-8#Description
	case u <=   0x7F: // 1-byte ASCII                 (U+0000 - U+007F)
		return append(p, byte(u & 0x7F))
	case u <=  0x7FF: // 2-bytes Latin-1              (U+0080 - U+07FF)
		return append(p,
		  0xC0 | uint8((u >>  6) & 0x1F),
		  0x80 | uint8( u        & 0x3F))
	case u <= 0xFFFF: // 3-bytes BMP                  (U+0800 - U+FFFF)
		return append(p,
		  0xE0 | uint8((u >> 12) & 0x0F),
		  0x80 | uint8((u >>  6) & 0x3F),
		  0x80 | uint8( u        & 0x3F))
	default:          // 4-bytes supplementary planes (U+10000 - U+10FFFF)
		return append(p,
		  0xF0 | uint8((u >> 18) & 0x07),
		  0x80 | uint8((u >> 12) & 0x3F),
		  0x80 | uint8((u >>  6) & 0x3F),
		  0x80 | uint8( u        & 0x3F))
	}
}

// isWideRune returns true if the rune consumes precisely two columns.
func isWideRune(r rune) bool {
	if r < 0x1100 { return false } // Hangul boundary
	return runewidth.RuneWidth(r) == 2
}

// widthInColumns calculates the visual column width of a string.
// correctly tracks 0-wide control codes, 1-wide western text, and 2-wide characters.
func widthInColumns(s string) int {
	width := 0
	for _, r := range s {
		if isWideRune(r) {
			width += 2
		} else {
			width += runewidth.RuneWidth(r) // safely resolve 0-wide control codes
		}
	}
	return width
}

// truncateFromLeft constrains the length of progress status messages
// rendered to the terminal, properly handling utf-8 strings.
func truncateFromLeft(s string, maxCols int) (string, bool) {
	if maxCols           <=       0 { return "", s != "" }
	if widthInColumns(s) <= maxCols { return s, false }

	width         := 0
	cutoffByteIdx := 0

	i := len(s)
	for i > 0 {
		_, size        := utf8.DecodeLastRuneInString(s[:i])
		currentByteIdx := i - size
		r              := []rune(s[currentByteIdx:])[0] // safely decode the single rune at this byte position
		
		rWidth := runewidth.RuneWidth(r) // handle 0-wide control codes
		if isWideRune(r) { rWidth = 2 }

		if width + rWidth > maxCols { // if adding this character exceeds maxCols, stop here
			cutoffByteIdx = i         // retain everything from index i onward
			break
		}
		width += rWidth
		i     -= size
	}

	return s[cutoffByteIdx:], true
}
