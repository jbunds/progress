package progress

import (
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

func TestAppendIntIdxInline(t *testing.T) {
	tests := []struct {
		name     string
		rgbInt   int
		allocCap int
		want     string
	}{
		{ "single digit",             5,  3,    "5" },
		{ "double digit",            42,  3,   "42" },
		{ "triple digit boundary",  999,  3,  "999" },
		{ "negative boundary",       -3, 10,    "0" },
		{ "fallback path",         1030, 10, "1030" },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(10, func() {
				appendIntIdxInline(make([]byte, 0, tt.allocCap), tt.rgbInt)
			})
			if allocs > 0 {
				t.Errorf("expected 0 allocations; got %v", allocs)
			}

			got := appendIntIdxInline(make([]byte, 0, tt.allocCap), tt.rgbInt)
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("appendIntIdxInline(%v) mismatch (-want +got):\n%s", tt.rgbInt, diff)
			}
		})
	}
}

func TestAppendRune(t *testing.T) {
	tests := []struct {
		name     string
		input    rune
		allocCap int
		want     string
	}{
		{ "1-byte ASCII",            'A',    1, "A"                  },
		{ "1-byte boundary control", 0x7F,   1, string(rune(0x7F))   },
		{ "2-byte Latin (Cyrillic)", 'я',    2, "я"                  },
		{ "2-byte boundary control", 0x7FF,  2, string(rune(0x7FF))  },
		{ "3-byte Euro symbol",      '€',    3, "€"                  },
		{ "3-byte boundary control", 0xFFFF, 3, string(rune(0xFFFF)) },
		{ "4-byte Emoji",            '🤘',   4, "🤘"                 },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(10, func() {
				appendRune(make([]byte, 0, tt.allocCap), tt.input)
			})
			if allocs > 0 {
				t.Errorf("expected 0 allocations; got %v", allocs)
			}

			got := appendRune(make([]byte, 0, tt.allocCap), tt.input)
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("appendRune(%q) mismatch (-want +got):\n%s", tt.input, diff)
			}

			// cross-validation against Go's standard library
			stdBuf  := make([]byte, utf8.UTFMax)
			n       := utf8.EncodeRune(stdBuf, tt.input)
			stdWant := string(stdBuf[:n])

			if diff := cmp.Diff(stdWant, string(got)); diff != "" {
				t.Errorf("appendRune(%q) mismatch (-want +got):\n%s", tt.input, diff)
			}
		})
	}
}
