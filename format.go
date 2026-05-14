package progress

// appendUintIdxInline writes a uint32 directly into a byte slice without heap allocation.
// optimized for RGB channel and terminal column ranges (0-255, 0-999).
func appendIntIdxInline(b []byte, v int) []byte {
	if v < 0 { return appendUintIdxFallback(b, uint32(0)) }
	switch {
	case v < 10:
		return append(b,                                                 byte('0' + (v & 0xF)))
	case v < 100:
		q := v / 10
		r := v - (q * 10) // fast modulo bypass
		return append(b,                         byte('0' + (q  & 0xF)), byte('0' + (r & 0xF)))
	case v < 1000:
		q1  := v / 100
		rem := v - (q1 * 100)
		q2  := rem / 10
		r   := rem - (q2 * 10)
		return append(b, byte('0' + (q1 & 0xF)), byte('0' + (q2 & 0xF)), byte('0' + (r & 0xF)))
	}

	return appendUintIdxFallback(b, uint32(v & 0x7FFFFFFF)) // fallback which avoids array-to-slice heap allocation escaping
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

	for j := i; j < len(buf); j++ { b = append(b, buf[j]) } // appending element by element in a small manual loop prevents the 'buf' array from escaping to the heap
	return b
}

// appendRune is a fast, zero-allocation inline implementation of utf8.EncodeRune.
func appendRune(p []byte, r rune) []byte {
	switch {
	case r <= 0x7F:
		return append(p, byte(r & 0x7F))
	case r <= 0x7FF:
		return append(p, 0xC0                                                                 | uint8((r >> 6) & 0x1F), 0x80 | uint8(r & 0x3F))
	case r <= 0xFFFF:
		return append(p, 0xE0                                 | uint8((r >> 12) & 0x0F), 0x80 | uint8((r >> 6) & 0x3F), 0x80 | uint8(r & 0x3F))
	default:
		return append(p, 0xF0 | uint8((r >> 18) & 0x07), 0x80 | uint8((r >> 12) & 0x3F), 0x80 | uint8((r >> 6) & 0x3F), 0x80 | uint8(r & 0x3F))
	}
}

// isWideRune provides a fast O(1) fallback check for east asian wide characters and emojis.
// Optimized to reduce branch depth for common western / emoji character streams.
func isWideRune(r rune) bool {
	if r < 0x1100 { return false } // fast path: early exit for common Western, Cyrillic, and Arabic blocks

	if r <= 0xFFFF { // binary partition: separate basic multilingual plane (BMP) from supplementary planes (emojis)
		switch {
		case r <= 0x115F: return true           // Hangul Jamo
		case r <  0x2E80: return false
		case r <= 0xA4CF: return r != 0x303F    // CJK radicals, symbols, extensions, ideographs
		case r <  0xAC00: return false
		case r <= 0xD7A3: return true           // Hangul syllables
		case r >= 0xF900:
			return r <= 0xFAFF                 || // group late BMP blocks together: CJK compatibility, vertical, and full-width forms
			      (r >= 0xFE10 && r <= 0xFE19) ||
			      (r >= 0xFE30 && r <= 0xFE6F) ||
			      (r <= 0xFF60)
		default: return false
		}
	}

	return r >= 0x1F300 && r <= 0x1FAFF // supplementary planes: modern emojis and pictographs (expanded up to U+1FAFF)
}

// truncateFromLeft constrains the length of progress status messages
// rendered to the terminal, properly handling utf-8 strings.
func truncateFromLeft(s string, maxLen int) (string, bool) {
	if maxLen <= 0 { return "", s != "" }
	
	if len(s) <= maxLen { return s, false } // fast path: if byte length is within bounds, rune count cannot exceed it

	// single-pass: track the exact byte offsets for the truncation boundaries while simultaneously counting total runes
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

	for idx := range s { // fast jump to the target index without creating any intermediate string headers
		if skip == 0 {
			cutoffIdx = idx
			break
		}
		skip--
	}

	return s[cutoffIdx:], maxLen > 1
}
