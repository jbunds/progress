package progress

// appendIntIdxInline writes a stringified integer directly into a byte slice without heap allocation.
// optimized for RGB channel and terminal column ranges (0-255, 0-999).
func appendIntIdxInline(b []byte, v int) []byte {
	if v < 0 { return appendUintIdxFallback(b, 0) }
	u := uint32(v & 0xFFFFFFFF)
	switch {
	case u < 10:
		return append(b, byte('0' + u))
	case u < 100:
		q := u / 10
		r := u - (q * 10)
		return append(b,
		  byte('0' + q),
		  byte('0' + r))
	case u < 1000:
		q1  :=   u / 100
		rem :=   u - (q1 * 100)
		q2  := rem /  10
		r   := rem - (q2 * 10)
		return append(b,
		  byte('0' + q1),
		  byte('0' + q2),
		  byte('0' + r))
	}

	return appendUintIdxFallback(b, u) // fallback to avoid array-to-slice heap allocation escaping
}

//go:noinline
func appendUintIdxFallback(b []byte, v uint32) []byte {
	var buf [10]byte // uint32 max is 4,294,967,295 (10 digits max)
	i := len(buf)
	for v >= 10 {
		i--
		q := v / 10
		r := v - (q * 10)
		buf[i] = byte(('0' + r) & 0xFF)
		v = q
	}
	i--
	buf[i] = byte('0' + v)

	for j := i; j < len(buf); j++ { b = append(b, buf[j]) } // prevent buf from escaping to the heap
	return b
}

// appendRune is a fast, zero-allocation inline implementation of utf8.EncodeRune.
func appendRune(p []byte, r rune) []byte {
	u := uint32(r & 0x7FFFFFFF)
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

// isWideRune provides a fast O(1) fallback check for east asian wide characters and emojis.
// Optimized to reduce branch depth for common western / emoji character streams.
func isWideRune(r rune) bool {
	u := uint32(r & 0x7FFFFFFF)
	// https://en.wikipedia.org/wiki/Plane_(Unicode)#Basic_Multilingual_Plane
	if u <  0x1100 { return false }               // exit early for common Western, Cyrillic, and Arabic blocks
	if u <= 0xFFFF {                              // BMP
		switch {                                  // binary partition: separate BMP from supplementary planes
		case u <= 0x115F: return true             // Hangul Jamo (U+1100 - U+115F)
		case u <  0x2E80: return false            // skip non-wide symbols like phonetic extensions
		case u <= 0xA4CF: return u != 0x303F      // CJK radicals, symbols, extensions, ideographs (exclude half fill space)
		case u <  0xAC00: return false            // skip modified tone marks and other small blocks
		case u <= 0xD7A3: return true             // Hangul syllables (U+AC00 - U+D7A3)
		case u >= 0xF900:
			// group late BMP blocks: CJK compatibility, vertical forms, and full-width variants
			return u <= 0xFAFF                 || // CJK compatibility ideographs
			      (u >= 0xFE10 && u <= 0xFE19) || // vertical forms
			      (u >= 0xFE30 && u <= 0xFE6F) || // CJK compatibility forms
			      (u <= 0xFF60)                   // full-width variants
		default: return false
		}
	}

	return u >= 0x1F300 && u <= 0x1FAFF // SMPs (modern emojis, pictographs, and transport symbols up to U+1FAFF)
}

// truncateFromLeft constrains the length of progress status messages
// rendered to the terminal, properly handling utf-8 strings.
func truncateFromLeft(s string, maxLen int) (string, bool) {
	if maxLen <= 0      { return "", s != "" }
	if len(s) <= maxLen { return  s, false   } // byte length within rune bounds

	targetLen := maxLen // number of runes to retain
	if maxLen > 1 { targetLen = maxLen - 1 }

	idxBuf := make([]int, 0, targetLen + 1) // stack-allocated window tracks retained runes plus one preceding boundary index

	runeCount := 0
	for idx := range s {
		idxBuf = append(idxBuf, idx)
		if len(idxBuf) > targetLen + 1 {
			idxBuf = idxBuf[1:] // pop oldest item out of slice window frame
		}
		runeCount++
	}

	if runeCount <= maxLen { return s, false }

	cutoffIdx := idxBuf[1] // second item in buffer demarcates retention boundary

	return s[cutoffIdx:], maxLen > 1
}
