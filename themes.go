package progress

import "github.com/google/go-cmp/cmp"

// https://jakob-bagterp.github.io/colorist-for-python/ansi-escape-codes/rgb-colors/

const (
	startFgR, startFgG, startFgB = 255, 255, 255 // always white foreground at 0% progress
)

type theme struct {
	startBgR, startBgG, startBgB int
	  endBgR,   endBgG,   endBgB int
	  endFgR,   endFgG,   endFgB int
	deltaBgR, deltaBgG, deltaBgB int
	deltaFgR, deltaFgG, deltaFgB int
}

func themeOrDefault(name string) *theme {
	if t, ok := getTheme(name); ok { return t }
	t, _ := getTheme("green")
	return t
}

func getTheme(name string) (*theme, bool) {
	switch name {
	case "green":
		return &theme{
			startBgR: 10,            startBgG:  25,            startBgB: 12,
			  endBgR: 40,              endBgG: 210,              endBgB: 85,
			  endFgR: 20,              endFgG:  30,              endFgB: 20,
			deltaBgR: 40 -       10, deltaBgG: 210 -       25, deltaBgB: 85 -       12,
			deltaFgR: 20 - startFgR, deltaFgG:  30 - startFgG, deltaFgB: 20 - startFgB,
		}, true
	case "red":
		return &theme{
			startBgR:  30,            startBgG:   5,            startBgB:   5,
			  endBgR: 210,              endBgG:  15,              endBgB:  25,
			  endFgR: 255,              endFgG: 220,              endFgB: 220,
			deltaBgR: 210 -       30, deltaBgG:  15 -        5, deltaBgB:  25 -        5,
			deltaFgR: 255 - startFgR, deltaFgG: 220 - startFgG, deltaFgB: 220 - startFgB,
		}, true
	case "orange":
		return &theme{
			startBgR:  26,            startBgG:  12,            startBgB: 12,
			  endBgR: 255,              endBgG: 150,              endBgB: 50,
			  endFgR:  42,              endFgG:  12,              endFgB: 12,
			deltaBgR: 255 -       26, deltaBgG: 150 -       12, deltaBgB: 50 -       12,
			deltaFgR:  42 - startFgR, deltaFgG:  12 - startFgG, deltaFgB: 12 - startFgB,
		}, true
	case "yellow":
		return &theme{
			startBgR:  55,            startBgG:  24,            startBgB:   2,
			endBgR:   255,              endBgG: 215,              endBgB:  10,
			endFgR:    35,              endFgG:  25,              endFgB:   5,
			deltaBgR: 255 -       55, deltaBgG: 215 -       24, deltaBgB:  10 -        2,
			deltaFgR:  35 - startFgR, deltaFgG:  25 - startFgG, deltaFgB:   5 - startFgB,
		}, true
	default:
		return &theme{}, false
	}
}

func (t *theme) Equal(other *theme) bool { // work around cmp's draconian strictures
	if t == nil || other == nil { return t == other }
	return cmp.Equal(t.startBgR, other.startBgR) &&
	       cmp.Equal(t.startBgG, other.startBgG) &&
	       cmp.Equal(t.startBgB, other.startBgB) &&
	       cmp.Equal(t.endBgR,   other.endBgR  ) &&
	       cmp.Equal(t.endBgG,   other.endBgG  ) &&
	       cmp.Equal(t.endBgB,   other.endBgB  ) &&
	       cmp.Equal(t.deltaBgR, other.deltaBgR) &&
	       cmp.Equal(t.deltaBgG, other.deltaBgG) &&
	       cmp.Equal(t.deltaBgB, other.deltaBgB) &&
	       cmp.Equal(t.deltaFgR, other.deltaFgR) &&
	       cmp.Equal(t.deltaFgG, other.deltaFgG) &&
	       cmp.Equal(t.deltaFgB, other.deltaFgB)
}
