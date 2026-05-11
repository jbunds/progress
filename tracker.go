package progress

import "strconv"

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

// statusTracker tracks the current work completion progress status.
type statusTracker interface {
	store(uint64, string) // handles interning / storage of the current status
	load()      any       // returns the comparable value (e.g., *string or Handle)
	value(any)  string    // converts the loaded value back to a string for display in the UI
	init()                // initializes tracker-specific UI layout configuration and metadata
	layout()    *layout   // returns the UI layout configuration and metadata for a tracker
}

func getTracker(strat strategy, totalUnits uint64) statusTracker {
	var tracker statusTracker

	switch strat {
	case Unique:  tracker = &uniqueTracker{}
	case Percent: tracker = &percentTracker{}
	case Fraction:
		switch totalUnits {
		case 0:  tracker = &standardTracker{} // silently fall back to a sensible progress tracking strategy
		default: tracker = &fractionTracker{ total: strconv.FormatUint(totalUnits, 10) }
		}
	default: tracker = &standardTracker{}
	}

	tracker.init()

	return tracker
}
