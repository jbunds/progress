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
		{
			name:     "sunset",
			fraction: 0.5,
			want:     rgb{r: 227, g: 44, b: 54},
		},
		{
			name:     "ocean",
			fraction: 0.7,
			want:     rgb{r: 10, g: 206, b: 150},
		},
		{
			name:     "thermal",
			fraction: 0.3,
			want:     rgb{r: 171, g: 14, b: 120},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newThemeRegistry().get(tt.name).bgColor(tt.fraction)
			if diff := cmp.Diff(tt.want, got); diff != "" {
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
		{
			name:     "no colors",
			theme:    &theme{},
			fraction: 0.1,
			want:     rgb{},
		},
		{
			name:     "one color",
			theme:    &theme{ colors: []rgb{{1, 2, 3}} },
			fraction: 0.2,
			want:     rgb{1, 2, 3},
		},
		{
			name:    "fraction exceeds 1.0",
			theme:    &theme{ colors: []rgb{
				{4, 5, 6},
				{7, 8, 9},
			}},
			fraction: 3,
			want:     rgb{7, 8, 9},
		},
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
