package investigation

import (
	"fmt"

	"github.com/pnathan/uncertain_logic/argumentation"
	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/temporal"
)

// BuildArgumentFramework constructs a Dung argumentation framework from
// temporally overlapping claims about a subject+predicate.
//
// The framework integrates all three existing legs:
//   - Temporal: only temporally overlapping claims can generate rebut attacks
//   - Belnap: each argument carries its evidence-derived Belnap status
//   - Subjective: each argument carries its credibility Opinion as strength
//
// Meta-claims with Refutes valence become Undercut attacks; those with
// Supports become support links (for causal chain propagation).
// Additional support links for causal chains can be added to the returned
// framework via AddSupport.
func (inv *Investigation) BuildArgumentFramework(subjectID, predicate string, at temporal.EventInterval) *argumentation.Framework {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	fw := argumentation.New()

	// Step 1: Find matching claims
	var matched []*models.Claim
	for _, c := range inv.claims {
		if c.SubjectID == subjectID && c.Predicate == predicate {
			if temporal.Overlapping(c.EventInterval, at) {
				matched = append(matched, c)
			}
		}
	}

	// Step 2: Build arguments from claims
	inFramework := make(map[string]bool)
	for _, c := range matched {
		analysis, err := inv.analyzeClaim(c.ID, 0)
		if err != nil {
			continue
		}
		fw.AddArgument(argumentation.Argument{
			ID:       c.ID,
			ClaimID:  c.ID,
			Strength: analysis.Credibility,
			Status:   analysis.BelnapStatus,
			Desc:     c.Content,
		})
		inFramework[c.ID] = true
	}

	// Step 3: Rebut attacks between contradictory claims
	for i := 0; i < len(matched); i++ {
		for j := i + 1; j < len(matched); j++ {
			a, b := matched[i], matched[j]
			if a.Value != b.Value && temporal.Overlapping(a.EventInterval, b.EventInterval) {
				fw.AddAttack(argumentation.Attack{
					AttackerID: a.ID, TargetID: b.ID, Type: argumentation.Rebut,
					Desc: fmt.Sprintf("%q contradicts %q", a.Value, b.Value),
				})
				fw.AddAttack(argumentation.Attack{
					AttackerID: b.ID, TargetID: a.ID, Type: argumentation.Rebut,
					Desc: fmt.Sprintf("%q contradicts %q", b.Value, a.Value),
				})
			}
		}
	}

	// Step 4: Meta-claims → attacks or support links (BFS to include chains)
	queue := make([]string, 0, len(inFramework))
	for id := range inFramework {
		queue = append(queue, id)
	}
	for len(queue) > 0 {
		targetID := queue[0]
		queue = queue[1:]
		for _, meta := range inv.metaClaimsAbout(targetID) {
			if inFramework[meta.ID] {
				continue
			}
			analysis, err := inv.analyzeClaim(meta.ID, 0)
			if err != nil {
				continue
			}
			fw.AddArgument(argumentation.Argument{
				ID:       meta.ID,
				ClaimID:  meta.ID,
				Strength: analysis.Credibility,
				Status:   analysis.BelnapStatus,
				Desc:     meta.Content,
			})
			inFramework[meta.ID] = true
			queue = append(queue, meta.ID)

			switch meta.Valence {
			case models.Refutes:
				fw.AddAttack(argumentation.Attack{
					AttackerID: meta.ID, TargetID: targetID,
					Type: argumentation.Rebut,
					Desc: fmt.Sprintf("meta-claim disputes %s", targetID),
				})
			case models.Supports:
				fw.AddSupport(argumentation.Support{
					SupporterID: meta.ID, SupportedID: targetID,
					Desc: fmt.Sprintf("meta-claim supports %s", targetID),
				})
			}
		}
	}

	return fw
}
