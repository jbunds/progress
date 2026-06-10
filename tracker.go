package progress

type strategy int

const (
	// Standard is the default tracker, and is suitable for tracking mostly unique status updates.
	Standard strategy = iota

	// Unique is suitable for tracking repetitive status updates.
	Unique

	// Fraction renders status updates as a proper fraction (x/y).
	Fraction

	// Percent renders only the percentage of completed work.
	Percent
)

// statusTracker tracks the current work completion progress status.
type statusTracker interface {
	init()                // initializes tracker-specific UI layout configuration and metadata
	store(uint64, string) // stores the current status string
	load() string         // returns the current status string
	addTotal(uint64)      // used by fractionTracker to add to the total units (denominator) when workers call AddTotal()
	baseLayout() layout   // returns the UI layout configuration and metadata for a tracker
}

func getTracker(strat strategy, totalUnits uint64) statusTracker {
	var tracker statusTracker

	switch strat {
	case Unique:  tracker = &uniqueTracker{}
	case Percent: tracker = &percentTracker{}
	case Fraction:
		switch totalUnits {
		case 0:  tracker = &standardTracker{} // silently fall back to a sensible progress tracking strategy
		default: tracker = &fractionTracker{ initialTotal: totalUnits }
		}
	default: tracker = &standardTracker{}
	}

	tracker.init()

	return tracker
}
