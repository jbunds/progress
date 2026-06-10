package progress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBgColor(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name     string
		fraction float64
		want     rgb
	}{
		{ "sunset",  0.5, rgb{227,  44,  54} },
		{ "ocean",   0.7, rgb{ 10, 206, 150} },
		{ "thermal", 0.3, rgb{171,  14, 120} },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newThemeRegistry().get(tt.name).bgColor(tt.fraction)
			if diff := cmp.Diff(tt.want, got, getCmpOpts()); diff != "" {
				t.Errorf("bgColor(%f) mismatch (-want +got):\n%s", tt.fraction, diff)
			}
		})
	}
}

func TestBgColorEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name     string
		theme    *theme
		fraction float64
		want     rgb
	}{
		{ "no colors",            &theme{                                     }, 0.1, rgb{       } },
		{ "one color",            &theme{ colors: []rgb{{1, 2, 3}}            }, 0.2, rgb{1, 2, 3} },
		{ "fraction exceeds 1.0", &theme{ colors: []rgb{{4, 5, 6}, {7, 8, 9}} }, 3.0, rgb{7, 8, 9} },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.theme.bgColor(tt.fraction)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("bgColor(%f) mismatch (-want +got):\n%s", tt.fraction, diff)
			}
		})
	}
}
