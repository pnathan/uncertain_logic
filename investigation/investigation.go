package investigation

import (
	"fmt"
	"time"

	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/subjective"
	"github.com/pnathan/uncertain_logic/temporal"
)

// Investigation holds all actors, subjects, claims, evidence, and findings for a research question.
type Investigation struct {
	ResearchQuestion string
	BaseRate         float64
	actors           map[string]*models.Actor
	subjects         map[string]*models.Subject
	claims           map[string]*models.Claim
	evidence         map[string]*models.Evidence
	findings         map[string]*Finding
	idCounter        int
}

// New creates a new Investigation.
func New(question string) *Investigation {
	return &Investigation{
		ResearchQuestion: question,
		BaseRate:         0.5,
		actors:           make(map[string]*models.Actor),
		subjects:         make(map[string]*models.Subject),
		claims:           make(map[string]*models.Claim),
		evidence:         make(map[string]*models.Evidence),
		findings:         make(map[string]*Finding),
	}
}

// --- Load (bulk import from storage) ---

// LoadActors registers pre-existing actors by their stored IDs.
func (inv *Investigation) LoadActors(actors []*models.Actor) {
	for _, a := range actors {
		inv.actors[a.ID] = a
	}
}

// LoadSubjects registers pre-existing subjects by their stored IDs.
func (inv *Investigation) LoadSubjects(subjects []*models.Subject) {
	for _, s := range subjects {
		inv.subjects[s.ID] = s
	}
}

// LoadClaims registers pre-existing claims by their stored IDs.
func (inv *Investigation) LoadClaims(claims []*models.Claim) {
	for _, c := range claims {
		inv.claims[c.ID] = c
	}
}

// LoadEvidence registers pre-existing evidence by their stored IDs.
// Evidence must be loaded after the claims it references so that
// claim.EvidenceIDs remain authoritative.
func (inv *Investigation) LoadEvidence(evidence []*models.Evidence) {
	for _, e := range evidence {
		inv.evidence[e.ID] = e
	}
}

// LoadFindings registers pre-existing findings by their stored IDs,
// making them addressable as TargetClaimID targets in new meta-claims.
func (inv *Investigation) LoadFindings(findings []*Finding) {
	for _, f := range findings {
		inv.findings[f.ID] = f
	}
}

// --- Read accessors (for persisting back to storage) ---

// Actors returns all registered actors.
func (inv *Investigation) Actors() []*models.Actor {
	out := make([]*models.Actor, 0, len(inv.actors))
	for _, a := range inv.actors {
		out = append(out, a)
	}
	return out
}

// Subjects returns all registered subjects.
func (inv *Investigation) Subjects() []*models.Subject {
	out := make([]*models.Subject, 0, len(inv.subjects))
	for _, s := range inv.subjects {
		out = append(out, s)
	}
	return out
}

// Claims returns all registered claims.
func (inv *Investigation) Claims() []*models.Claim {
	out := make([]*models.Claim, 0, len(inv.claims))
	for _, c := range inv.claims {
		out = append(out, c)
	}
	return out
}

// Evidence returns all registered evidence.
func (inv *Investigation) Evidence() []*models.Evidence {
	out := make([]*models.Evidence, 0, len(inv.evidence))
	for _, e := range inv.evidence {
		out = append(out, e)
	}
	return out
}

// Findings returns all registered findings — the outputs ready to persist.
func (inv *Investigation) Findings() []*Finding {
	out := make([]*Finding, 0, len(inv.findings))
	for _, f := range inv.findings {
		out = append(out, f)
	}
	return out
}

func (inv *Investigation) nextID(prefix string) string {
	inv.idCounter++
	return fmt.Sprintf("%s_%d", prefix, inv.idCounter)
}

// --- Actor options ---

type ActorOption func(*models.Actor)

func WithReliability(r float64) ActorOption {
	return func(a *models.Actor) { a.BaseReliability = r }
}

func WithConflict(c models.ConflictOfInterest) ActorOption {
	return func(a *models.Actor) { a.Conflicts = append(a.Conflicts, c) }
}

func WithActorNotes(notes string) ActorOption {
	return func(a *models.Actor) { a.Notes = notes }
}

// AddActor registers an actor. Default reliability: 0.6.
func (inv *Investigation) AddActor(id, name string, sourceType models.SourceType, opts ...ActorOption) *models.Actor {
	a := &models.Actor{
		ID:              id,
		Name:            name,
		SourceType:      sourceType,
		BaseReliability: 0.6,
	}
	for _, o := range opts {
		o(a)
	}
	inv.actors[id] = a
	return a
}

// AddSubject registers a subject entity.
func (inv *Investigation) AddSubject(id, name, subjectType string) *models.Subject {
	s := &models.Subject{ID: id, Name: name, SubjectType: subjectType}
	inv.subjects[id] = s
	return s
}

// --- Claim options ---

type ClaimOption func(*models.Claim)

func WithClaimType(ct models.ClaimType) ClaimOption {
	return func(c *models.Claim) { c.ClaimType = ct }
}

func WithSourceURL(url string) ClaimOption {
	return func(c *models.Claim) { c.SourceURL = url }
}

func WithSourceDescription(desc string) ClaimOption {
	return func(c *models.Claim) { c.SourceDescription = desc }
}

func WithClaimNotes(notes string) ClaimOption {
	return func(c *models.Claim) { c.Notes = notes }
}

func WithValence(v models.Valence) ClaimOption {
	return func(c *models.Claim) { c.Valence = v }
}

// --- Assert DSL ---

// AssertFact adds a high-confidence ground truth claim (system actor, reliability≈1.0).
func (inv *Investigation) AssertFact(p models.Proposition, interval temporal.EventInterval, opts ...ClaimOption) string {
	// Ensure system actor exists
	if _, ok := inv.actors["_system"]; !ok {
		inv.actors["_system"] = &models.Actor{
			ID:              "_system",
			Name:            "System (ground truth)",
			SourceType:      models.Institutional,
			BaseReliability: 1.0,
		}
	}
	id := inv.nextID("claim")
	c := &models.Claim{
		ID:            id,
		ActorID:       "_system",
		SubjectID:     p.Subject,
		Predicate:     p.Predicate,
		Value:         p.Value,
		Content:       fmt.Sprintf("%s %s = %s", p.Subject, p.Predicate, p.Value),
		ClaimType:     models.Factual,
		AssertionTime: time.Now(),
		EventInterval: interval,
	}
	for _, o := range opts {
		o(c)
	}
	inv.claims[id] = c
	return id
}

// AssertClaim adds a claim made by an actor. Returns the claim ID.
func (inv *Investigation) AssertClaim(actorID string, p models.Proposition, assertionTime time.Time, interval temporal.EventInterval, opts ...ClaimOption) string {
	id := inv.nextID("claim")
	c := &models.Claim{
		ID:            id,
		ActorID:       actorID,
		SubjectID:     p.Subject,
		Predicate:     p.Predicate,
		Value:         p.Value,
		Content:       fmt.Sprintf("%s %s = %s", p.Subject, p.Predicate, p.Value),
		ClaimType:     models.Factual,
		AssertionTime: assertionTime,
		EventInterval: interval,
	}
	for _, o := range opts {
		o(c)
	}
	inv.claims[id] = c
	return id
}

// AssertMetaClaim adds a claim about another claim (reification).
// predicate describes the relationship (e.g. "accuracy", "authorship").
// value is the asserted value (e.g. "false", "stated").
// The Valence option should be set to indicate whether this supports or refutes the target.
func (inv *Investigation) AssertMetaClaim(actorID, targetClaimID string, predicate, value string, assertionTime time.Time, opts ...ClaimOption) string {
	id := inv.nextID("claim")
	c := &models.Claim{
		ID:            id,
		ActorID:       actorID,
		TargetClaimID: targetClaimID,
		Predicate:     predicate,
		Value:         value,
		Content:       fmt.Sprintf("re:%s %s = %s", targetClaimID, predicate, value),
		ClaimType:     models.Attribution,
		AssertionTime: assertionTime,
		EventInterval: temporal.Open("same as target"),
	}
	for _, o := range opts {
		o(c)
	}
	inv.claims[id] = c
	return id
}

// --- Evidence options ---

type EvidenceOption func(*models.Evidence)

func WithWeight(w float64) EvidenceOption {
	return func(e *models.Evidence) { e.Weight = &w }
}

func WithEvidenceURL(url string) EvidenceOption {
	return func(e *models.Evidence) { e.SourceURL = url }
}

func WithSourceClaimID(id string) EvidenceOption {
	return func(e *models.Evidence) { e.SourceClaimID = id }
}

func WithEvidenceNotes(notes string) EvidenceOption {
	return func(e *models.Evidence) { e.Notes = notes }
}

// AddEvidence attaches evidence to a claim. Returns evidence ID.
func (inv *Investigation) AddEvidence(claimID, content string, valence models.Valence, opts ...EvidenceOption) string {
	id := inv.nextID("evidence")
	e := &models.Evidence{
		ID:      id,
		ClaimID: claimID,
		Content: content,
		Valence: valence,
	}
	for _, o := range opts {
		o(e)
	}
	inv.evidence[id] = e
	// Register in claim
	if c, ok := inv.claims[claimID]; ok {
		c.EvidenceIDs = append(c.EvidenceIDs, id)
	}
	return id
}

// --- Query DSL ---

// QueryResult is the evaluated logical status of a proposition at a time slice.
type QueryResult struct {
	Proposition   models.Proposition
	Interval      temporal.EventInterval
	Belnap        belnap.Value
	Opinion       subjective.Opinion
	MatchedClaims []*models.Claim
}

// Q evaluates all claims matching subject+predicate at the given interval.
// Returns one QueryResult per overlapping cluster of claims.
func (inv *Investigation) Q(subjectID, predicate string, at temporal.EventInterval) []QueryResult {
	var matched []*models.Claim
	for _, c := range inv.claims {
		if c.SubjectID == subjectID && c.Predicate == predicate {
			if temporal.Overlapping(c.EventInterval, at) {
				matched = append(matched, c)
			}
		}
	}
	if len(matched) == 0 {
		return []QueryResult{{
			Proposition: models.Proposition{Subject: subjectID, Predicate: predicate},
			Interval:    at,
			Belnap:      belnap.Neither,
			Opinion:     subjective.Vacuous(inv.BaseRate),
		}}
	}

	// Compute composite result
	var opinions []subjective.Opinion
	supportCount, refuteCount := 0, 0
	for _, c := range matched {
		analysis, err := inv.analyzeClaim(c.ID, 0)
		if err != nil {
			continue
		}
		opinions = append(opinions, analysis.Credibility)
		switch analysis.BelnapStatus {
		case belnap.True:
			supportCount++
		case belnap.False:
			refuteCount++
		case belnap.Both:
			supportCount++
			refuteCount++
		}
	}

	status := belnap.FromCounts(supportCount, refuteCount)
	var fusedOpinion subjective.Opinion
	if len(opinions) > 0 {
		fusedOpinion = subjective.ConsensusFuse(opinions...)
	} else {
		fusedOpinion = subjective.Vacuous(inv.BaseRate)
	}

	return []QueryResult{{
		Proposition:   models.Proposition{Subject: subjectID, Predicate: predicate},
		Interval:      at,
		Belnap:        status,
		Opinion:       fusedOpinion,
		MatchedClaims: matched,
	}}
}

// Logical operations on QueryResults.
//
// And/Or combine DIFFERENT propositions (e.g. "is X profitable?" AND
// "is X growing?"). They use Jøsang's multiplication / co-multiplication
// operators so that E[A∧B] = E[A]·E[B] (independence assumption).
//
// To combine independent assessments of the SAME proposition from different
// sources, use ConsensusFuse directly.
//
// Reference: Jøsang, "Subjective Logic" (Springer 2016), §14.3.

// And computes logical AND of two QueryResults.
// Belnap: truth-lattice meet (per Belnap 1977).
// Opinion: binomial multiplication (per Jøsang 2016 §14.3).
func And(a, b QueryResult) QueryResult {
	return QueryResult{
		Proposition:   a.Proposition,
		Interval:      a.Interval,
		Belnap:        belnap.And(a.Belnap, b.Belnap),
		Opinion:       subjective.Multiply(a.Opinion, b.Opinion),
		MatchedClaims: append(a.MatchedClaims, b.MatchedClaims...),
	}
}

// Or computes logical OR of two QueryResults.
// Belnap: truth-lattice join (per Belnap 1977).
// Opinion: binomial co-multiplication (per Jøsang 2016 §14.3).
func Or(a, b QueryResult) QueryResult {
	return QueryResult{
		Proposition:   a.Proposition,
		Interval:      a.Interval,
		Belnap:        belnap.Or(a.Belnap, b.Belnap),
		Opinion:       subjective.CoMultiply(a.Opinion, b.Opinion),
		MatchedClaims: append(a.MatchedClaims, b.MatchedClaims...),
	}
}

// Not negates a QueryResult.
func Not(a QueryResult) QueryResult {
	return QueryResult{
		Proposition:   a.Proposition,
		Interval:      a.Interval,
		Belnap:        belnap.Not(a.Belnap),
		Opinion:       subjective.Negate(a.Opinion),
		MatchedClaims: a.MatchedClaims,
	}
}

// --- Low-level access ---

// ClaimsAbout returns all claims about a subject.
func (inv *Investigation) ClaimsAbout(subjectID string) []*models.Claim {
	var result []*models.Claim
	for _, c := range inv.claims {
		if c.SubjectID == subjectID {
			result = append(result, c)
		}
	}
	return result
}

// MetaClaimsAbout returns all meta-claims targeting a claim or finding ID.
func (inv *Investigation) MetaClaimsAbout(claimID string) []*models.Claim {
	var result []*models.Claim
	for _, c := range inv.claims {
		if c.TargetClaimID == claimID {
			result = append(result, c)
		}
	}
	return result
}

// AnalyzeClaim performs depth-limited recursive credibility analysis.
func (inv *Investigation) AnalyzeClaim(claimID string) (*ClaimAnalysis, error) {
	return inv.analyzeClaim(claimID, 0)
}

// AnalyzeAll analyzes all claims.
func (inv *Investigation) AnalyzeAll() []*ClaimAnalysis {
	var results []*ClaimAnalysis
	for id := range inv.claims {
		a, err := inv.analyzeClaim(id, 0)
		if err == nil {
			results = append(results, a)
		}
	}
	return results
}

// Summary returns a brief summary of the investigation.
func (inv *Investigation) Summary() string {
	return fmt.Sprintf("Investigation: %s\n  Actors: %d, Subjects: %d, Claims: %d, Evidence: %d, Findings: %d",
		inv.ResearchQuestion,
		len(inv.actors), len(inv.subjects), len(inv.claims), len(inv.evidence), len(inv.findings))
}
