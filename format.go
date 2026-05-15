package progress

// appendIntIdxInline writes a stringified integer directly into a byte slice without heap allocation.
// optimized for RGB channel and terminal column ranges (0-255, 0-999).
func appendIntIdxInline(b []byte, v int) []byte {
	if v < 0 { return appendUintIdxFallback(b, uint32(0)) }
	switch {
	case v < 10:
		return append(b, byte('0' + (v & 0xF)))
	case v < 100:
		q := v / 10
		r := v - (q * 10)
		return append(b,
		  byte('0' + (q & 0xF)),
		  byte('0' + (r & 0xF)))
	case v < 1000:
		q1  :=   v / 100
		rem :=   v - (q1 * 100)
		q2  := rem / 10
		r   := rem - (q2 * 10)
		return append(b,
		  byte('0' + (q1 & 0xF)),
		  byte('0' + (q2 & 0xF)),
		  byte('0' + (r  & 0xF)))
	}

	return appendUintIdxFallback(b, uint32(v & 0x7FFFFFFF)) // fallback to avoid array-to-slice heap allocation escaping
}

//go:noinline
func appendUintIdxFallback(b []byte, v uint32) []byte {
	var buf [10]byte // uint32 max is 4,294,967,295 (10 digits max)
	i := len(buf)
	for v >= 10 {
		i--
		q := v / 10
		r := v - (q * 10)
		buf[i] = byte('0' + (r & 0xF))
		v = q
	}
	i--
	buf[i] = byte('0' + (v & 0xF))

	for j := i; j < len(buf); j++ { b = append(b, buf[j]) } // prevent the 'buf' array from escaping to the heap
	return b
}

// appendRune is a fast, zero-allocation inline implementation of utf8.EncodeRune.
func appendRune(p []byte, r rune) []byte {
	switch {          // https://en.wikipedia.org/wiki/UTF-8#Description
	case r <=   0x7F: // 1-byte ASCII                 (U+0000 - U+007F)
		return append(p, byte(r & 0x7F))
	case r <=  0x7FF: // 2-bytes Latin-1              (U+0080 - U+07FF)
		return append(p,
		  0xC0 | uint8((r >>  6) & 0x1F),
		  0x80 | uint8( r        & 0x3F))
	case r <= 0xFFFF: // 3-bytes BMP                  (U+0800 - U+FFFF)
		return append(p,
		  0xE0 | uint8((r >> 12) & 0x0F),
		  0x80 | uint8((r >>  6) & 0x3F),
		  0x80 | uint8( r        & 0x3F))
	default:          // 4-bytes supplementary planes (U+10000 - U+10FFFF)
		return append(p,
		  0xF0 | uint8((r >> 18) & 0x07),
		  0x80 | uint8((r >> 12) & 0x3F),
		  0x80 | uint8((r >>  6) & 0x3F),
		  0x80 | uint8( r        & 0x3F))
	}
}

// isWideRune provides a fast O(1) fallback check for east asian wide characters and emojis.
// Optimized to reduce branch depth for common western / emoji character streams.
func isWideRune(r rune) bool {
	// https://en.wikipedia.org/wiki/Plane_(Unicode)#Basic_Multilingual_Plane
	if r <  0x1100 { return false }               // exit early for common Western, Cyrillic, and Arabic blocks
	if r <= 0xFFFF {                              // BMP
		switch {                                  // binary partition: separate BMP from supplementary planes
		case r <= 0x115F: return true             // Hangul Jamo (U+1100 - U+115F)
		case r <  0x2E80: return false            // skip non-wide symbols like phonetic extensions
		case r <= 0xA4CF: return r != 0x303F      // CJK radicals, symbols, extensions, ideographs (exclude half fill space)
		case r <  0xAC00: return false            // skip modified tone marks and other small blocks
		case r <= 0xD7A3: return true             // Hangul syllables (U+AC00 - U+D7A3)
		case r >= 0xF900:
			// group late BMP blocks: CJK compatibility, vertical forms, and full-width variants
			return r <= 0xFAFF                 || // CJK compatibility ideographs
			      (r >= 0xFE10 && r <= 0xFE19) || // vertical forms
			      (r >= 0xFE30 && r <= 0xFE6F) || // CJK compatibility forms
			      (r <= 0xFF60)                   // full-width variants
		default: return false
		}
	}

	return r >= 0x1F300 && r <= 0x1FAFF // SMPs (modern emojis, pictographs, and transport symbols up to U+1FAFF)
}

// truncateFromLeft constrains the length of progress status messages
// rendered to the terminal, properly handling utf-8 strings.
func truncateFromLeft(s string, maxLen int) (string, bool) {
	if maxLen <= 0 { return "", s != "" }
	
	if len(s) <= maxLen { return s, false } // if byte length is within bounds, rune count cannot exceed it

	// track the exact byte offsets for the truncation boundaries while simultaneously counting total runes
	var cutoffIdx int
	var runeCount int
	
	for idx := range s {
		_ = idx
		runeCount++
	}

	if runeCount <= maxLen { return s, false }

	// calculate the number of leftward runes to be skipped
	skip := runeCount - maxLen
	if maxLen > 1 { skip = runeCount - (maxLen - 1) }

	for idx := range s { // jump to the target index without creating any intermediate string headers
		if skip == 0 {
			cutoffIdx = idx
			break
		}
		skip--
	}

	return s[cutoffIdx:], maxLen > 1
}
