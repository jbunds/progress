package progress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetTheme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want *theme
	}{
		{
			name: "default", // no theme named "default"; should return "sunset"
			want: &theme{
				name:   "sunset",
				colors: []rgb{
					{ 48,  25, 52},
					{199,   0, 57},
					{255,  87, 51},
					{255, 195,  0},
				},
			},
    },
		{
			name: "ocean",
			want: &theme{
				name:   "ocean",
				colors: []rgb{
					{  0,  10,  45},
					{  0, 128, 128},
					{  0, 200, 150},
					{100, 255, 150},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newThemeRegistry().get(tt.name)
			if diff := cmp.Diff(tt.want, got, getCmpOpts()); diff != "" {
				t.Errorf("themeOrDefault(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()
	theme := &theme{}
	if theme.Equal(nil) {
		t.Errorf("Equal() mismatch: expected false; got true")
	}
}
