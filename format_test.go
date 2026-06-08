package progress

import (
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

func TestAppendRGBInline(t *testing.T) {
	tests := []struct {
		name string
		rgb  uint8
		want string
	}{
		{ "single digit",   3,   "3" },
		{ "double digit",  42,  "42" },
		{ "upper bound",  255, "255" },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(10, func() {
				appendRGBInline(make([]byte, 0, 3), tt.rgb)
			})
			if allocs > 0 {
				t.Errorf("expected 0 allocations; got %v", allocs)
			}

			got := appendRGBInline(make([]byte, 0, 3), tt.rgb)
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("appendRGBInline(%v) mismatch (-want +got):\n%s", tt.rgb, diff)
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

func TestTruncateFromLeft(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		str       string
		maxLen    int
		wantStr   string
		wantTrunc bool
	}{
		{ "maxLen == 0",          "foo",    0, "",       true  },
		{ "wide runes truncated", "😐😕🙁", 3, "🙁",     true  },
		{ "wide runes retained",  "😐😕🙁", 6, "😐😕🙁", false },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, trunc := truncateFromLeft(tt.str, tt.maxLen)
			if diff := cmp.Diff(tt.wantStr, got); diff != "" {
				t.Errorf("truncateFromLeft(%q, %d) mismatch (-want +got):\n%s", tt.str, tt.maxLen, diff)
			}
			if diff := cmp.Diff(tt.wantTrunc, trunc); diff != "" {
				t.Errorf("truncateFromLeft(%q, %d) mismatch (-want +got):\n%s", tt.str, tt.maxLen, diff)
			}
		})
	}
}
