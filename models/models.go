package models

import (
	"time"

	"github.com/pnathan/uncertain_logic/temporal"
)

// SourceType classifies the kind of actor making claims.
type SourceType int

const (
	Analyst SourceType = iota
	Journalist
	Expert
	Insider
	Regulator
	Institutional
	Anonymous
	SocialMedia
	Troll
)

func (s SourceType) String() string {
	switch s {
	case Analyst:
		return "Analyst"
	case Journalist:
		return "Journalist"
	case Expert:
		return "Expert"
	case Insider:
		return "Insider"
	case Regulator:
		return "Regulator"
	case Institutional:
		return "Institutional"
	case Anonymous:
		return "Anonymous"
	case SocialMedia:
		return "SocialMedia"
	case Troll:
		return "Troll"
	default:
		return "Unknown"
	}
}

// ClaimType classifies what kind of assertion is being made.
type ClaimType int

const (
	Factual ClaimType = iota
	Predictive
	Evaluative
	Causal
	Attribution // "Actor X asserted claim Y" — journalist meta-claim type
)

func (c ClaimType) String() string {
	switch c {
	case Factual:
		return "Factual"
	case Predictive:
		return "Predictive"
	case Evaluative:
		return "Evaluative"
	case Causal:
		return "Causal"
	case Attribution:
		return "Attribution"
	default:
		return "Unknown"
	}
}

// Valence describes whether evidence/meta-claim supports or refutes.
type Valence int

const (
	Supports Valence = iota
	Refutes
	Neutral
)

func (v Valence) String() string {
	switch v {
	case Supports:
		return "Supports"
	case Refutes:
		return "Refutes"
	case Neutral:
		return "Neutral"
	default:
		return "Unknown"
	}
}

// ConflictDirection describes the nature of a conflict of interest.
type ConflictDirection int

const (
	Long       ConflictDirection = iota // financially long the subject
	Short                               // financially short the subject
	EmployedBy                          // employed by the subject
	Competitor                          // competes with the subject
	Personal                            // personal relationship
	Other
)

// ConflictOfInterest represents a known bias an actor has toward a subject.
type ConflictOfInterest struct {
	Description string
	Direction   ConflictDirection
	Disclosed   bool
	SubjectID   string // empty = applies to all subjects
}

// TrustPenalty returns the multiplicative factor to apply to the actor's reliability.
// Disclosed conflicts reduce trust to 0.80; undisclosed to 0.50.
func (c ConflictOfInterest) TrustPenalty() float64 {
	if c.Disclosed {
		return 0.80
	}
	return 0.50
}

// Actor is a source that makes claims.
type Actor struct {
	ID              string
	Name            string
	SourceType      SourceType
	BaseReliability float64
	Conflicts       []ConflictOfInterest
	Notes           string
}

// AdjustedReliability computes the actor's effective reliability for a given subject,
// applying all relevant conflict-of-interest penalties.
func (a *Actor) AdjustedReliability(subjectID string) float64 {
	r := a.BaseReliability
	for _, c := range a.Conflicts {
		if c.SubjectID == "" || c.SubjectID == subjectID {
			r *= c.TrustPenalty()
		}
	}
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// Subject is an entity (company, person, event) that claims are about.
type Subject struct {
	ID          string
	Name        string
	SubjectType string
	Notes       string
}

// Evidence is a piece of supporting or refuting material attached to a claim.
type Evidence struct {
	ID                string
	ClaimID           string
	Content           string
	Valence           Valence
	SourceURL         string
	SourceDescription string
	SourceClaimID     string   // if this evidence is itself a Claim in the system
	Weight            *float64 // nil → default 1.0 (1 evidence = 1 observation)
	Notes             string
}

// EffectiveWeight returns the evidence weight, defaulting to 1.0.
// Weight acts as a strength multiplier: 1 evidence item = 1 observation.
func (e *Evidence) EffectiveWeight() float64 {
	if e.Weight != nil {
		return *e.Weight
	}
	return 1.0
}

// Proposition is the logical unit of a claim: what is being asserted.
type Proposition struct {
	Subject   string // entity ID or claim ID for reification
	Predicate string // property name: "revenue", "stance", "accuracy"
	Value     string // asserted value: "50M", "conservative", "false"
}

// Claim is an assertion made by an Actor about a Subject or another Claim.
type Claim struct {
	ID      string
	ActorID string

	// Exactly one of SubjectID or TargetClaimID is set.
	SubjectID     string // entity subject
	TargetClaimID string // meta: this claim is about another claim

	// The proposition fields (maps to Proposition)
	Predicate string
	Value     string

	Content   string // full text of the claim
	ClaimType ClaimType

	// For meta-claims, Valence describes the relationship to the target:
	// Supports = endorses/attributes, Refutes = disputes/contradicts
	Valence Valence

	AssertionTime     time.Time              // Q2: when did they claim it?
	EventInterval     temporal.EventInterval // Q4: when does the claim say it happened?
	SourceURL         string
	SourceDescription string
	EvidenceIDs       []string
	Notes             string
}

// Clone returns a deep copy of the Actor.
func (a Actor) Clone() Actor {
	clone := a
	if len(a.Conflicts) > 0 {
		clone.Conflicts = make([]ConflictOfInterest, len(a.Conflicts))
		copy(clone.Conflicts, a.Conflicts)
	}
	return clone
}

// Clone returns a deep copy of the Subject.
func (s Subject) Clone() Subject {
	return s
}

// Clone returns a deep copy of the Evidence.
func (e Evidence) Clone() Evidence {
	clone := e
	if e.Weight != nil {
		w := *e.Weight
		clone.Weight = &w
	}
	return clone
}

// Clone returns a deep copy of the Claim.
func (c Claim) Clone() Claim {
	clone := c
	if len(c.EvidenceIDs) > 0 {
		clone.EvidenceIDs = make([]string, len(c.EvidenceIDs))
		copy(clone.EvidenceIDs, c.EvidenceIDs)
	}
	clone.EventInterval = c.EventInterval.Clone()
	return clone
}

// ToProposition extracts the Proposition from a Claim.
func (c *Claim) ToProposition() Proposition {
	subj := c.SubjectID
	if subj == "" {
		subj = c.TargetClaimID
	}
	return Proposition{
		Subject:   subj,
		Predicate: c.Predicate,
		Value:     c.Value,
	}
}
