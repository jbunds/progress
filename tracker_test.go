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

func TestUniqueTrackerStoresAndLoads(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status string
		want1   string
		want2   string
	}{
		{
			name:   "succeeds",
			status: "foo",
			want1:  "",
			want2:  "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := getTracker(Unique, 0)
			got := u.value(u.load())
			if diff := cmp.Diff(tt.want1, got); diff != "" {
				t.Errorf("load(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			u.store(0, tt.status)
			got = u.value(u.load())
			if diff := cmp.Diff(tt.want2, got); diff != "" {
				t.Errorf("store(%q} / load() mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestUniqueTrackerEqualMethod(t *testing.T) {
	t.Run("uniqueTracker.Equal", func(t *testing.T) {
		t.Parallel()
		u1 := getTracker(Unique, 0)
		u2 := (*uniqueTracker)(nil)
		if cmp.Equal(u1, u2) {
			t.Errorf("Equal(%v, %v) mismatch (-want +got):\n%t", u1, u2, false)
		}
	})
}

func TestStandardTrackerEqualMethod(t *testing.T) {
	t.Run("standardTracker.Equal", func(t *testing.T) {
		t.Parallel()
		s1 := getTracker(Standard, 0)
		s2 := (*standardTracker)(nil)
		if cmp.Equal(s1, s2) {
			t.Errorf("Equal(%v, %v) mismatch (-want +got):\n%t", s1, s2, false)
		}
	})
}

func TestPercentTrackerMethods(t *testing.T) {
	t.Run("percentTracker store, load, value", func(t *testing.T) {
		t.Parallel()
		p := getTracker(Percent, 0)
		if diff := cmp.Diff(nil, p.load()); diff != "" {
			t.Errorf("load() mismatch (-want +got):\n%s", diff)
		}
		p.store(0, "")
		if diff := cmp.Diff("", p.value(p.load())); diff != "" {
			t.Errorf("store(%d, %q) / load() mismatch (-want +got):\n%s", 0, "", diff)
		}
	})
}

func TestUniqueTrackerValue(t *testing.T) {
	t.Run("uniqueTracker.value", func(t *testing.T) {
		t.Parallel()
		u   := getTracker(Unique, 0)
		got := u.value("bogus")
		if diff := cmp.Diff("", got); diff != "" {
			t.Errorf("value(%q) mismatch (-want +got):\n%s", "bogus", diff)
		}
	})
}

func TestFractionTrackerValue(t *testing.T) {
	t.Run("fractionTracker.value", func(t *testing.T) {
		t.Parallel()
		f   := getTracker(Fraction, 3)
		got := f.value("bogus")
		if diff := cmp.Diff("0/3", got); diff != "" {
			t.Errorf("value(%q) mismatch (-want +got):\n%s", "bogus", diff)
		}
	})
}
