package investigation

import (
	"fmt"
	"strings"

	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/subjective"
)

const maxDepth = 3

// ClaimAnalysis holds the evaluated logical status and credibility of a claim.
type ClaimAnalysis struct {
	Claim            models.Claim
	Actor            *models.Actor
	Subject          *models.Subject
	BelnapStatus     belnap.Value
	Credibility      subjective.Opinion
	SupportingCount  int
	RefutingCount    int
	NeutralCount     int
	PositiveEvidence float64 // r: weighted positive evidence count
	NegativeEvidence float64 // s: weighted negative evidence count
}

// FiveQuestions prints a structured answer to the five investigative questions.
func (a *ClaimAnalysis) FiveQuestions() string {
	var sb strings.Builder

	// Q1: What does someone claim?
	sb.WriteString("=== Five Questions ===\n")
	sb.WriteString(fmt.Sprintf("Q1 (What):  %s\n", a.Claim.Content))

	// Q2: When did they claim it?
	actorName := "Unknown"
	if a.Actor != nil {
		actorName = a.Actor.Name
	}
	sb.WriteString(fmt.Sprintf("Q2 (When claimed): %s claimed at %s\n",
		actorName, a.Claim.AssertionTime.Format("2006-01-02")))

	// Q3: What are they claiming it about?
	about := a.Claim.SubjectID
	if about == "" {
		about = fmt.Sprintf("claim:%s", a.Claim.TargetClaimID)
	}
	if a.Subject != nil {
		about = fmt.Sprintf("%s (%s)", a.Subject.Name, a.Subject.ID)
	}
	sb.WriteString(fmt.Sprintf("Q3 (About):  %s\n", about))

	// Q4: When did they claim it happened?
	interval := a.Claim.EventInterval
	if interval.Desc != "" {
		sb.WriteString(fmt.Sprintf("Q4 (Event interval): %s\n", interval.Desc))
	} else {
		start := "unspecified"
		end := "unspecified"
		if interval.Start != nil {
			start = interval.Start.Format("2006-01-02")
		}
		if interval.End != nil {
			end = interval.End.Format("2006-01-02")
		}
		sb.WriteString(fmt.Sprintf("Q4 (Event interval): %s to %s\n", start, end))
	}

	// Q5: How much do we believe them, and why?
	ep := a.Credibility.ExpectedProbability()
	sb.WriteString(fmt.Sprintf("Q5 (Credibility): Belnap=%s, E[p]=%.3f (b=%.3f d=%.3f u=%.3f)\n",
		a.BelnapStatus, ep, a.Credibility.Belief, a.Credibility.Disbelief, a.Credibility.Uncertainty))
	sb.WriteString(fmt.Sprintf("   Evidence: %d supporting, %d refuting, %d neutral (r=%.2f, s=%.2f)\n",
		a.SupportingCount, a.RefutingCount, a.NeutralCount, a.PositiveEvidence, a.NegativeEvidence))

	if a.Actor != nil {
		subjectID := a.Claim.SubjectID
		adj := a.Actor.AdjustedReliability(subjectID, a.Claim.Predicate)
		sb.WriteString(fmt.Sprintf("   Actor reliability: %.3f (base=%.3f, after competence+conflicts)\n",
			adj, a.Actor.BaseReliability))
	}

	return sb.String()
}

// analyzeClaim is the depth-limited recursive credibility computation.
// Caller must hold at least RLock.
func (inv *Investigation) analyzeClaim(claimID string, depth int) (*ClaimAnalysis, error) {
	// If this ID refers to a registered Finding, return its already-computed
	// result directly rather than recomputing. This is the entry point for
	// layered investigations where a prior Finding is the target of a new
	// meta-claim.
	if f, ok := inv.findings[claimID]; ok {
		return &ClaimAnalysis{
			BelnapStatus:     f.BelnapStatus,
			Credibility:      f.Credibility,
			SupportingCount:  f.SupportingCount,
			RefutingCount:    f.RefutingCount,
			NeutralCount:     f.NeutralCount,
			PositiveEvidence: f.PositiveEvidence,
			NegativeEvidence: f.NegativeEvidence,
		}, nil
	}

	c, ok := inv.claims[claimID]
	if !ok {
		return nil, fmt.Errorf("claim %q not found", claimID)
	}

	actor := inv.actors[c.ActorID]
	subject := inv.subjects[c.SubjectID]

	// Step 1: Actor trust opinion
	var adjustedReliability float64 = 0.6
	if actor != nil {
		adjustedReliability = actor.AdjustedReliability(c.SubjectID, c.Predicate)
	}
	trust := subjective.FromReliability(adjustedReliability, inv.BaseRate)

	// Step 2: Actor asserts claim with certainty; discount by our trust.
	// If the claim's own Valence is Refutes, the actor is asserting the
	// proposition is false, so we start from DogmaticFalse.
	var actorAssertion subjective.Opinion
	if c.Valence == models.Refutes {
		actorAssertion = subjective.DogmaticFalse(inv.BaseRate)
	} else {
		actorAssertion = subjective.DogmaticTrue(inv.BaseRate)
	}
	actorOpinion := subjective.TrustDiscount(trust, actorAssertion)

	// Step 3: Count evidence items for Belnap status
	supporting, refuting, neutral := 0, 0, 0
	for _, evID := range c.EvidenceIDs {
		ev, ok := inv.evidence[evID]
		if !ok {
			continue
		}
		switch ev.Valence {
		case models.Supports:
			supporting++
		case models.Refutes:
			refuting++
		case models.Neutral:
			neutral++
		}
	}

	// D1: Dogmatic actors short-circuit — facts are axioms, not evidence.
	// Josang (2016) §3.4: dogmatic = infinite evidence.
	if actorOpinion.Uncertainty == 0 {
		belnapStatus := belnap.FromCounts(supporting, refuting)
		if supporting == 0 && refuting == 0 {
			if c.Valence == models.Refutes {
				belnapStatus = belnap.False
			} else {
				belnapStatus = belnap.True
			}
		}

		clonedClaim := c.Clone()
		var clonedActor *models.Actor
		if actor != nil {
			a := actor.Clone()
			clonedActor = &a
		}
		var clonedSubject *models.Subject
		if subject != nil {
			s := subject.Clone()
			clonedSubject = &s
		}

		return &ClaimAnalysis{
			Claim:            clonedClaim,
			Actor:            clonedActor,
			Subject:          clonedSubject,
			BelnapStatus:     belnapStatus,
			Credibility:      actorOpinion,
			SupportingCount:  supporting,
			RefutingCount:    refuting,
			NeutralCount:     neutral,
			PositiveEvidence: 0,
			NegativeEvidence: 0,
		}, nil
	}

	// D2: Inverse-map actor opinion to evidence counts.
	// r = W·b/u, s = W·d/u per Josang (2016) Ch. 3 inverse bijection.
	r, s := subjective.EvidenceCounts(actorOpinion)

	// Step 4: Accumulate weighted evidence counts
	for _, evID := range c.EvidenceIDs {
		ev, ok := inv.evidence[evID]
		if !ok {
			continue
		}
		w := ev.EffectiveWeight()
		switch ev.Valence {
		case models.Supports:
			r += w
		case models.Refutes:
			s += w
		}
	}

	// Step 5: D5 Meta-claims: trust-discount on evidence counts (depth-limited).
	// EBSL analogy (Josang & Ismail 2002) extended to recursive chains.
	if depth < maxDepth {
		for _, meta := range inv.metaClaimsAbout(claimID) {
			metaAnalysis, err := inv.analyzeClaim(meta.ID, depth+1)
			if err != nil {
				continue
			}
			// Meta-actor's adjusted reliability as trust factor
			metaActor := inv.actors[meta.ActorID]
			var trustFactor float64
			if metaActor != nil {
				trustFactor = metaActor.AdjustedReliability(c.SubjectID, c.Predicate)
			}

			// Dogmatic meta-claims contribute nothing (r=0, s=0 from short-circuit)
			rMeta := metaAnalysis.PositiveEvidence
			sMeta := metaAnalysis.NegativeEvidence

			switch meta.Valence {
			case models.Supports:
				supporting++
				r += trustFactor * rMeta
				s += trustFactor * sMeta
			case models.Refutes:
				refuting++
				r += trustFactor * sMeta
				s += trustFactor * rMeta
			}
		}
	}

	// Step 6: Build credibility opinion from accumulated evidence counts.
	credibility := subjective.OpinionFromEvidence(r, s, inv.BaseRate)

	// Step 7: Belnap status from evidence.
	belnapStatus := belnap.FromCounts(supporting, refuting)
	if supporting == 0 && refuting == 0 {
		if adjustedReliability > 0.5 {
			if c.Valence == models.Refutes {
				belnapStatus = belnap.False
			} else {
				belnapStatus = belnap.True
			}
		} else {
			belnapStatus = belnap.Neither
		}
	}

	// Clone entities for the return value to prevent aliasing
	clonedClaim := c.Clone()
	var clonedActor *models.Actor
	if actor != nil {
		a := actor.Clone()
		clonedActor = &a
	}
	var clonedSubject *models.Subject
	if subject != nil {
		s := subject.Clone()
		clonedSubject = &s
	}

	return &ClaimAnalysis{
		Claim:            clonedClaim,
		Actor:            clonedActor,
		Subject:          clonedSubject,
		BelnapStatus:     belnapStatus,
		Credibility:      credibility,
		SupportingCount:  supporting,
		RefutingCount:    refuting,
		NeutralCount:     neutral,
		PositiveEvidence: r,
		NegativeEvidence: s,
	}, nil
}
