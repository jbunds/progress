package progress

import "strconv"

// statusTracker tracks the current work completion progress status.
type statusTracker interface {
  store(float64, string) // handles interning / storage of the current status
  load()     any         // returns the comparable value (e.g., *string or Handle)
  value(any) string      // converts the loaded value back to a string for display in the UI
}

type strategy int

const (
	// Standard is the default tracker, and is suitable for tracking mostly unique status updates.
	Standard strategy = iota
	// Unique is suitable for tracking repetitive status updates.
	Unique
	// Fraction renders status updates to the UI as a proper fraction (x/y).
	Fraction
	// Percent renders only the percentage of completed work.
	Percent
)

func getTracker(strat strategy, totalUnits uint64) statusTracker {
	switch strat {
	case Standard: return &standardTracker{}
	case Unique:   return &uniqueTracker{}
	case Percent:  return &percentTracker{}
	case Fraction:
		if totalUnits == 0 {
			return &standardTracker{} // silently fall back to a sensible progress tracking strategy
		}
		return &fractionTracker{
			total: strconv.FormatUint(totalUnits, 10),
		}
	default: return &standardTracker{}
	}
}
