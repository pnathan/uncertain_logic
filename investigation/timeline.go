package investigation

import (
	"sort"

	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/temporal"
)

// TimeSlice groups claims about a subject whose EventIntervals overlap.
// Claims in the same slice are candidates for Belnap comparison.
// Claims in different slices signal temporal evolution (fluent change).
type TimeSlice struct {
	Interval temporal.EventInterval
	Claims   []models.Claim
}

// SubjectTimeline returns the ordered sequence of TimeSlices for a subject.
// This reveals how the narrative about the subject has evolved over time.
func (inv *Investigation) SubjectTimeline(subjectID string) []TimeSlice {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	claims := inv.claimsAbout(subjectID)
	if len(claims) == 0 {
		return nil
	}

	// Sort claims by assertion time for deterministic ordering
	sort.Slice(claims, func(i, j int) bool {
		return claims[i].AssertionTime.Before(claims[j].AssertionTime)
	})

	// Greedy interval clustering: merge claims whose EventIntervals overlap
	// with the representative interval of the current cluster.
	var slices []TimeSlice
	used := make([]bool, len(claims))

	for i, c := range claims {
		if used[i] {
			continue
		}
		// Start a new cluster with c
		cluster := []*models.Claim{c}
		representative := c.EventInterval
		used[i] = true

		// Find all other claims that overlap with the representative interval
		changed := true
		for changed {
			changed = false
			for j, other := range claims {
				if used[j] {
					continue
				}
				if temporal.Overlapping(representative, other.EventInterval) {
					cluster = append(cluster, other)
					used[j] = true
					changed = true
					// Expand representative to cover the union (conservative)
					representative = unionInterval(representative, other.EventInterval)
				}
			}
		}

		// Clone claims for the return value
		clonedCluster := make([]models.Claim, len(cluster))
		for k, cl := range cluster {
			clonedCluster[k] = cl.Clone()
		}

		slices = append(slices, TimeSlice{
			Interval: representative,
			Claims:   clonedCluster,
		})
	}

	return slices
}

// unionInterval returns the interval covering both a and b.
// If either is open, returns open.
func unionInterval(a, b temporal.EventInterval) temporal.EventInterval {
	if a.Start == nil || b.Start == nil || a.End == nil || b.End == nil {
		return temporal.Open("(open union)")
	}
	start := a.Start
	if b.Start.Before(*a.Start) {
		start = b.Start
	}
	end := a.End
	if b.End.After(*a.End) {
		end = b.End
	}
	return temporal.EventInterval{Start: start, End: end}
}

// BeliefRevision records a pair of claims by the same actor about the same subject
// where the later claim may contradict the earlier one.
type BeliefRevision struct {
	Earlier models.Claim
	Later   models.Claim
	// TemporallyConsistent is true if the intervals don't overlap
	// (both could be true — the subject changed). False means genuine reversal.
	TemporallyConsistent bool
}

// ActorBeliefHistory returns the sequence of claims by an actor about a subject,
// ordered by AssertionTime, and the belief revisions detected.
func (inv *Investigation) ActorBeliefHistory(actorID, subjectID string) ([]models.Claim, []BeliefRevision) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	var internal []*models.Claim
	for _, c := range inv.claims {
		if c.ActorID == actorID && c.SubjectID == subjectID {
			internal = append(internal, c)
		}
	}
	sort.Slice(internal, func(i, j int) bool {
		return internal[i].AssertionTime.Before(internal[j].AssertionTime)
	})

	// Clone for return
	claims := make([]models.Claim, len(internal))
	for i, c := range internal {
		claims[i] = c.Clone()
	}

	var revisions []BeliefRevision
	for i := 1; i < len(claims); i++ {
		earlier := claims[i-1]
		later := claims[i]
		// A revision occurs when predicate+subject match but values differ
		if earlier.Predicate == later.Predicate && earlier.Value != later.Value {
			revisions = append(revisions, BeliefRevision{
				Earlier:              earlier,
				Later:                later,
				TemporallyConsistent: !temporal.Overlapping(earlier.EventInterval, later.EventInterval),
			})
		}
	}

	return claims, revisions
}
