package investigation

import (
	"fmt"
	"time"

	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/subjective"
)

// Finding is the persisted result of analyzing a claim.
// It has an ID so it can be referenced by TargetClaimID in a subsequent
// meta-claim, either within the same investigation or after being loaded
// from storage into a new one.
type Finding struct {
	ID              string
	ClaimID         string
	BelnapStatus    belnap.Value
	Credibility     subjective.Opinion
	SupportingCount int
	RefutingCount   int
	NeutralCount    int
	AnalyzedAt      time.Time
	Notes           string
}

// RegisterAnalysis stores a ClaimAnalysis as a Finding in the investigation
// and returns it. The Finding's ID is addressable as a TargetClaimID in
// subsequent meta-claims; its already-computed credibility is reused
// rather than recomputed.
func (inv *Investigation) RegisterAnalysis(a *ClaimAnalysis) *Finding {
	id := inv.nextID("finding")
	f := &Finding{
		ID:              id,
		ClaimID:         a.Claim.ID,
		BelnapStatus:    a.BelnapStatus,
		Credibility:     a.Credibility,
		SupportingCount: a.SupportingCount,
		RefutingCount:   a.RefutingCount,
		NeutralCount:    a.NeutralCount,
		AnalyzedAt:      time.Now(),
		Notes:           fmt.Sprintf("Analysis of claim %s", a.Claim.ID),
	}
	inv.findings[id] = f
	return f
}
