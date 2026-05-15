package progress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestThemeOrDefault(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want *theme
	}{
		{
			name: "default", // no theme named "default"; should return "green"
			want: &theme{
				startBgR:   10, startBgG:   25, startBgB:   12,
				  endBgR:   40,   endBgG:  210,   endBgB:   85,
				  endFgR:   20,   endFgG:   30,   endFgB:   20,
				deltaBgR:   30, deltaBgG:  185, deltaBgB:   73,
				deltaFgR: -235, deltaFgG: -225, deltaFgB: -235,
			},
		},
		{
			name: "red",
			want: &theme{
				startBgR:  30, startBgG:   5, startBgB:   5,
				  endBgR: 210,   endBgG:  15,   endBgB:  25,
				  endFgR: 255,   endFgG: 220,   endFgB: 220,
				deltaBgR: 180, deltaBgG:  10, deltaBgB:  20,
				deltaFgR:   0, deltaFgG: -35, deltaFgB: -35,
			},
		},
		{
			name: "orange",
			want: &theme{
				startBgR:   26, startBgG:   12, startBgB:   12,
				  endBgR:  255,   endBgG:  150,   endBgB:   50,
				  endFgR:   42,   endFgG:   12,   endFgB:   12,
				deltaBgR:  229, deltaBgG:  138, deltaBgB:   38,
				deltaFgR: -213, deltaFgG: -243, deltaFgB: -243,
			},
		},
		{
			name: "yellow",
			want: &theme{
				startBgR:   55, startBgG:   24, startBgB:    2,
				  endBgR:  255,   endBgG:  215,   endBgB:   10,
				  endFgR:   35,   endFgG:   25,   endFgB:    5,
				deltaBgR:  200, deltaBgG:  191, deltaBgB:    8,
				deltaFgR: -220, deltaFgG: -230, deltaFgB: -250,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := themeOrDefault(tt.name)
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
