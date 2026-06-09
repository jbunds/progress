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
			total: 3,
			strat: Fraction,
			want:  &fractionTracker{ initialTotal: 3 },
		},
		{
			name:  "fraction",
			total: 0,
			strat: Fraction,
			want:  &standardTracker{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.want.init()
			got := getTracker(tt.strat, tt.total)
			if diff := cmp.Diff(tt.want, got, getCmpOpts()); diff != "" {
				t.Errorf("getTracker(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestUniqueTrackerStoreAndLoad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status string
		want1  string
		want2  string
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
			u   := getTracker(Unique, 0)
			got := u.load()
			if diff := cmp.Diff(tt.want1, got); diff != "" {
				t.Errorf("load(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			u.store(0, tt.status)
			got = u.load()
			if diff := cmp.Diff(tt.want2, got); diff != "" {
				t.Errorf("store(%q} / load() mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestUniqueTrackerEqualMethod(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	t.Run("standardTracker.Equal", func(t *testing.T) {
		t.Parallel()
		s1 := getTracker(Standard, 0)
		s2 := (*standardTracker)(nil)
		if cmp.Equal(s1, s2) {
			t.Errorf("Equal(%v, %v) mismatch (-want +got):\n%t", s1, s2, false)
		}
	})
}

func TestPercentTrackerStoreAndLoad(t *testing.T) {
	t.Parallel()
	t.Run("percentTracker store, load", func(t *testing.T) {
		t.Parallel()
		p := getTracker(Percent, 0)
		if diff := cmp.Diff("", p.load()); diff != "" {
			t.Errorf("load() mismatch (-want +got):\n%s", diff)
		}
		p.store(0, "discarded")
		if diff := cmp.Diff("", p.load()); diff != "" {
			t.Errorf("store(%d, %q) / load() mismatch (-want +got):\n%s", 0, "", diff)
		}
	})
}

func TestUniqueTrackerLoad(t *testing.T) {
	t.Parallel()
	t.Run("uniqueTracker.load", func(t *testing.T) {
		t.Parallel()
		u   := getTracker(Unique, 0)
		got := u.load()
		if diff := cmp.Diff("", got); diff != "" {
			t.Errorf("load() mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestFractionTrackerLoad(t *testing.T) {
	t.Parallel()
	t.Run("fractionTracker.load", func(t *testing.T) {
		t.Parallel()
		f   := getTracker(Fraction, 3)
		got := f.load()
		if diff := cmp.Diff("0/3", got); diff != "" {
			t.Errorf("load() mismatch (-want +got):\n%s", diff)
		}
	})
}
