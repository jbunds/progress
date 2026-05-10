package progress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTracker(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		strat strategy
		total uint64
		want  statusTracker
	}{
		{
			name:  "standard",
			strat: Standard,
			want:  &standardTracker{},
		},
		{
			name:  "unique",
			strat: Unique,
			want:  &uniqueTracker{},
		},
		{
			name:  "percent",
			strat: Percent,
			want:  &percentTracker{},
		},
		{
			name:  "fraction",
			strat: Fraction,
			want:  &standardTracker{},
		},
		{
			name:  "fraction",
			total: 1,
			strat: Fraction,
			want:  &fractionTracker{ total: "1" },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTracker(tt.strat, tt.total)
			if diff := cmp.Diff(tt.want, got, opts); diff != "" {
				t.Errorf("getTracker(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
