package progress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestThemeOrDefault(t *testing.T) {
	t.Parallel()
	opts := cmp.Options{
		cmpopts.IgnoreFields(theme{},
			"startBgR", "startBgG", "startBgB",
			  "endBgR",   "endBgG",   "endBgB",
			  "endFgR",   "endFgG",   "endFgB",
		),
		cmp.AllowUnexported(theme{}),
		cmpopts.EquateComparable(),
	}
	tests := []struct {
		name string
		want *theme
	}{
		{
			name: "default", // no theme named "default"; should return "green"
			want: &theme{
				deltaBgR:   30, deltaBgG:  185, deltaBgB:   73,
				deltaFgR: -235, deltaFgG: -225, deltaFgB: -235,
			},
		},
		{
			name: "red",
			want: &theme{
				deltaBgR: 180, deltaBgG:  10, deltaBgB:  20,
				deltaFgR:   0, deltaFgG: -35, deltaFgB: -35,
			},
		},
		{
			name: "orange",
			want: &theme{
				deltaBgR:  229, deltaBgG:  138, deltaBgB:   38,
				deltaFgR: -213, deltaFgG: -243, deltaFgB: -243,
			},
		},
		{
			name: "yellow",
			want: &theme{
				deltaBgR:  200, deltaBgG:  191, deltaBgB:    8,
				deltaFgR: -220, deltaFgG: -230, deltaFgB: -250,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := themeOrDefault(tt.name)
			if diff := cmp.Diff(tt.want, got, opts); diff != "" {
				t.Errorf("themeOrDefault(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
