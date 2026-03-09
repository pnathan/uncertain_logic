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
	Claim           *models.Claim
	Actor           *models.Actor
	Subject         *models.Subject
	BelnapStatus    belnap.Value
	Credibility     subjective.Opinion
	SupportingCount int
	RefutingCount   int
	NeutralCount    int
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
	sb.WriteString(fmt.Sprintf("   Evidence: %d supporting, %d refuting, %d neutral\n",
		a.SupportingCount, a.RefutingCount, a.NeutralCount))

	if a.Actor != nil {
		subjectID := a.Claim.SubjectID
		adj := a.Actor.AdjustedReliability(subjectID)
		sb.WriteString(fmt.Sprintf("   Actor reliability: %.3f (base=%.3f, after conflicts)\n",
			adj, a.Actor.BaseReliability))
	}

	return sb.String()
}

// analyzeClaim is the depth-limited recursive credibility computation.
func (inv *Investigation) analyzeClaim(claimID string, depth int) (*ClaimAnalysis, error) {
	// If this ID refers to a registered Finding, return its already-computed
	// result directly rather than recomputing. This is the entry point for
	// layered investigations where a prior Finding is the target of a new
	// meta-claim.
	if f, ok := inv.findings[claimID]; ok {
		return &ClaimAnalysis{
			BelnapStatus:    f.BelnapStatus,
			Credibility:     f.Credibility,
			SupportingCount: f.SupportingCount,
			RefutingCount:   f.RefutingCount,
			NeutralCount:    f.NeutralCount,
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
		adjustedReliability = actor.AdjustedReliability(c.SubjectID)
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

	// Step 3: Accumulate evidence opinions
	var evidenceOpinions []subjective.Opinion
	supporting, refuting, neutral := 0, 0, 0

	for _, evID := range c.EvidenceIDs {
		ev, ok := inv.evidence[evID]
		if !ok {
			continue
		}
		w := ev.EffectiveWeight()
		switch ev.Valence {
		case models.Supports:
			supporting++
			evidenceOpinions = append(evidenceOpinions, subjective.Opinion{
				Belief: w, Disbelief: 0, Uncertainty: 1 - w, BaseRate: inv.BaseRate,
			})
		case models.Refutes:
			refuting++
			evidenceOpinions = append(evidenceOpinions, subjective.Opinion{
				Belief: 0, Disbelief: w, Uncertainty: 1 - w, BaseRate: inv.BaseRate,
			})
		case models.Neutral:
			neutral++
		}
	}

	// Step 4: Meta-claims as evidence (depth-limited)
	if depth < maxDepth {
		for _, meta := range inv.MetaClaimsAbout(claimID) {
			metaAnalysis, err := inv.analyzeClaim(meta.ID, depth+1)
			if err != nil {
				continue
			}
			ep := metaAnalysis.Credibility.ExpectedProbability()
			switch meta.Valence {
			case models.Supports:
				supporting++
				evidenceOpinions = append(evidenceOpinions, subjective.Opinion{
					Belief: ep, Disbelief: 0, Uncertainty: 1 - ep, BaseRate: inv.BaseRate,
				})
			case models.Refutes:
				refuting++
				evidenceOpinions = append(evidenceOpinions, subjective.Opinion{
					Belief: 0, Disbelief: ep, Uncertainty: 1 - ep, BaseRate: inv.BaseRate,
				})
			}
		}
	}

	// Step 5: Fuse all opinions
	allOpinions := append([]subjective.Opinion{actorOpinion}, evidenceOpinions...)
	credibility := subjective.ConsensusFuse(allOpinions...)

	// Step 6: Belnap status from evidence
	belnapStatus := belnap.FromCounts(supporting, refuting)
	// If no evidence at all, actor assertion alone → True (we trust the actor to some degree)
	if supporting == 0 && refuting == 0 {
		if adjustedReliability > 0.5 {
			belnapStatus = belnap.True
		} else if adjustedReliability < 0.3 {
			belnapStatus = belnap.Neither
		} else {
			belnapStatus = belnap.Neither
		}
	}

	return &ClaimAnalysis{
		Claim:           c,
		Actor:           actor,
		Subject:         subject,
		BelnapStatus:    belnapStatus,
		Credibility:     credibility,
		SupportingCount: supporting,
		RefutingCount:   refuting,
		NeutralCount:    neutral,
	}, nil
}
