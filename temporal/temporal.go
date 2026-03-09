package temporal

import "time"

// EventInterval represents the time period a claim is *about*.
// nil Start/End means unspecified (open interval).
type EventInterval struct {
	Start *time.Time
	End   *time.Time
	Desc  string // fuzzy description: "Q3 2024", "around the IPO"
}

// PointInTime creates a degenerate interval for a single moment.
func PointInTime(t time.Time) EventInterval {
	tc := t
	return EventInterval{Start: &tc, End: &tc}
}

// Open creates a fully unspecified interval (conservative: overlaps everything).
func Open(desc string) EventInterval {
	return EventInterval{Desc: desc}
}

// AllenRelation is one of Allen's 13 temporal relations.
type AllenRelation int

const (
	Precedes     AllenRelation = iota // a before b, no contact
	Meets                             // a ends exactly when b starts
	Overlaps                          // a starts before b, they share some time, a ends before b ends
	FinishedBy                        // a contains b's endpoint; a and b end together
	Contains                          // a contains b entirely (a starts before b and ends after b)
	Starts                            // a starts with b, a ends before b
	Equals                            // a and b are identical
	StartedBy                         // b starts with a, b ends before a
	During                            // b contains a entirely
	Finishes                          // a ends with b, a starts after b
	OverlappedBy                      // b starts before a, they share some time, b ends before a ends
	MetBy                             // b ends exactly when a starts
	PrecededBy                        // b before a, no contact
)

// Relate returns the Allen relation between two intervals.
// Returns Equals if either interval has nil bounds (conservative — treat as potentially overlapping).
//
// Reference: Allen, "Maintaining Knowledge about Temporal Intervals",
// Communications of the ACM 26(11), 1983, pp. 832–843.
// Allen's framework assumes non-degenerate intervals (Start < End).
// Degenerate point intervals (Start == End) are handled correctly by
// checking Equals before boundary-contact relations like Meets/MetBy.
func Relate(a, b EventInterval) AllenRelation {
	// Any open/unspecified bound → conservative: treat as Equals (overlapping)
	if a.Start == nil || a.End == nil || b.Start == nil || b.End == nil {
		return Equals
	}
	aS, aE := *a.Start, *a.End
	bS, bE := *b.Start, *b.End

	// Check Equals first so that degenerate point intervals (Start == End)
	// are not misclassified as Meets when all four times coincide.
	switch {
	case aS.Equal(bS) && aE.Equal(bE):
		return Equals
	case aE.Before(bS):
		return Precedes
	case aE.Equal(bS):
		return Meets
	case aS.Before(bS) && aE.Before(bE) && aE.After(bS):
		return Overlaps
	case aS.Before(bS) && aE.Equal(bE):
		return FinishedBy
	case aS.Before(bS) && aE.After(bE):
		return Contains
	case aS.Equal(bS) && aE.Before(bE):
		return Starts
	case aS.Equal(bS) && aE.After(bE):
		return StartedBy
	case aS.After(bS) && aE.Before(bE):
		return During
	case aS.After(bS) && aE.Equal(bE):
		return Finishes
	case aS.After(bS) && aS.Before(bE) && aE.After(bE):
		return OverlappedBy
	case aS.Equal(bE):
		return MetBy
	default:
		return PrecededBy
	}
}

// Clone returns a deep copy of the EventInterval.
func (ei EventInterval) Clone() EventInterval {
	clone := ei
	if ei.Start != nil {
		t := *ei.Start
		clone.Start = &t
	}
	if ei.End != nil {
		t := *ei.End
		clone.End = &t
	}
	return clone
}

// Overlapping returns true if the two intervals could refer to the same time period.
// Non-overlapping intervals (Precedes, PrecededBy) indicate temporal evolution, not contradiction.
func Overlapping(a, b EventInterval) bool {
	r := Relate(a, b)
	return r != Precedes && r != PrecededBy
}
