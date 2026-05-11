package progress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetTracker(t *testing.T) {
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
			want:  &standardTracker{ lo: defaultLayout() },
		},
		{
			name:  "unique",
			strat: Unique,
			want:  &uniqueTracker{ lo: defaultLayout() },
		},
		{
			name:  "percent",
			strat: Percent,
			want:  &percentTracker{
				lo: &layout{
					staticWidth:    17,
					prefix:         "processing (",
					suffix:         "%)",
					doneSeq:        "\n",
					lineTerminator: "\n",
				},
			},
		},
		{
			name:  "fraction",
			strat: Fraction,
			want:  &standardTracker{ lo: defaultLayout() },
		},
		{
			name:  "fraction",
			total: 1,
			strat: Fraction,
			want:  &fractionTracker{
				total: "1",
				lo:    defaultLayout(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getTracker(tt.strat, tt.total)
			if diff := cmp.Diff(tt.want, got, opts...); diff != "" {
				t.Errorf("getTracker(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
