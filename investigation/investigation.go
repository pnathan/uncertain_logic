package investigation

import (
	"fmt"
	"sync"
	"time"

	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/subjective"
	"github.com/pnathan/uncertain_logic/temporal"
)

// Investigation holds all actors, subjects, claims, evidence, and findings for a research question.
// An Investigation is safe for concurrent use by multiple goroutines. Accessor methods return
// value copies; to mutate entities, use the Update*/Add* methods which acquire the write lock.
type Investigation struct {
	ResearchQuestion string
	BaseRate         float64
	mu               sync.RWMutex
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
func (inv *Investigation) LoadActors(actors []models.Actor) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	for _, a := range actors {
		a := a
		inv.actors[a.ID] = &a
	}
}

// LoadSubjects registers pre-existing subjects by their stored IDs.
func (inv *Investigation) LoadSubjects(subjects []models.Subject) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	for _, s := range subjects {
		s := s
		inv.subjects[s.ID] = &s
	}
}

// LoadClaims registers pre-existing claims by their stored IDs.
func (inv *Investigation) LoadClaims(claims []models.Claim) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	for _, c := range claims {
		c := c
		inv.claims[c.ID] = &c
	}
}

// LoadEvidence registers pre-existing evidence by their stored IDs.
// Evidence must be loaded after the claims it references so that
// claim.EvidenceIDs remain authoritative.
func (inv *Investigation) LoadEvidence(evidence []models.Evidence) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	for _, e := range evidence {
		e := e
		inv.evidence[e.ID] = &e
	}
}

// LoadFindings registers pre-existing findings by their stored IDs,
// making them addressable as TargetClaimID targets in new meta-claims.
func (inv *Investigation) LoadFindings(findings []Finding) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	for _, f := range findings {
		f := f
		inv.findings[f.ID] = &f
	}
}

// --- Read accessors (for persisting back to storage) ---

// Actors returns all registered actors as value copies.
func (inv *Investigation) Actors() []models.Actor {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	out := make([]models.Actor, 0, len(inv.actors))
	for _, a := range inv.actors {
		out = append(out, a.Clone())
	}
	return out
}

// Subjects returns all registered subjects as value copies.
func (inv *Investigation) Subjects() []models.Subject {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	out := make([]models.Subject, 0, len(inv.subjects))
	for _, s := range inv.subjects {
		out = append(out, s.Clone())
	}
	return out
}

// Claims returns all registered claims as value copies.
func (inv *Investigation) Claims() []models.Claim {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	out := make([]models.Claim, 0, len(inv.claims))
	for _, c := range inv.claims {
		out = append(out, c.Clone())
	}
	return out
}

// Evidence returns all registered evidence as value copies.
func (inv *Investigation) Evidence() []models.Evidence {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	out := make([]models.Evidence, 0, len(inv.evidence))
	for _, e := range inv.evidence {
		out = append(out, e.Clone())
	}
	return out
}

// Findings returns all registered findings as value copies.
func (inv *Investigation) Findings() []Finding {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	out := make([]Finding, 0, len(inv.findings))
	for _, f := range inv.findings {
		out = append(out, f.Clone())
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
// Returns a value copy of the created actor.
func (inv *Investigation) AddActor(id, name string, sourceType models.SourceType, opts ...ActorOption) models.Actor {
	inv.mu.Lock()
	defer inv.mu.Unlock()
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
	return a.Clone()
}

// AddSubject registers a subject entity.
// Returns a value copy of the created subject.
func (inv *Investigation) AddSubject(id, name, subjectType string) models.Subject {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	s := &models.Subject{ID: id, Name: name, SubjectType: subjectType}
	inv.subjects[id] = s
	return s.Clone()
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
	inv.mu.Lock()
	defer inv.mu.Unlock()
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
	inv.mu.Lock()
	defer inv.mu.Unlock()
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
	inv.mu.Lock()
	defer inv.mu.Unlock()
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
	inv.mu.Lock()
	defer inv.mu.Unlock()
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
	MatchedClaims []models.Claim
}

// Q evaluates all claims matching subject+predicate at the given interval.
// Returns one QueryResult per overlapping cluster of claims.
func (inv *Investigation) Q(subjectID, predicate string, at temporal.EventInterval) []QueryResult {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

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
	// D3: ABF within same actor, CBF across actors.
	// Josang, Diaz & Rifqi (2010) §3-4: CBF for independent, ABF for dependent sources.
	actorOpinions := make(map[string][]subjective.Opinion)
	supportCount, refuteCount := 0, 0
	for _, c := range matched {
		analysis, err := inv.analyzeClaim(c.ID, 0)
		if err != nil {
			continue
		}
		actorOpinions[c.ActorID] = append(actorOpinions[c.ActorID], analysis.Credibility)
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
	if len(actorOpinions) > 0 {
		// Within each actor: ABF (idempotent, same-source dependency)
		var perActorFused []subjective.Opinion
		for _, group := range actorOpinions {
			perActorFused = append(perActorFused, subjective.AveragingFuse(group...))
		}
		// Across actors: CBF (independent sources)
		fusedOpinion = subjective.ConsensusFuse(perActorFused...)
	} else {
		fusedOpinion = subjective.Vacuous(inv.BaseRate)
	}

	// Clone matched claims for the return value
	clonedMatched := make([]models.Claim, len(matched))
	for i, c := range matched {
		clonedMatched[i] = c.Clone()
	}

	return []QueryResult{{
		Proposition:   models.Proposition{Subject: subjectID, Predicate: predicate},
		Interval:      at,
		Belnap:        status,
		Opinion:       fusedOpinion,
		MatchedClaims: clonedMatched,
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

// claimsAbout returns internal claim pointers for a subject. Caller must hold at least RLock.
func (inv *Investigation) claimsAbout(subjectID string) []*models.Claim {
	var result []*models.Claim
	for _, c := range inv.claims {
		if c.SubjectID == subjectID {
			result = append(result, c)
		}
	}
	return result
}

// ClaimsAbout returns all claims about a subject as value copies.
func (inv *Investigation) ClaimsAbout(subjectID string) []models.Claim {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	internal := inv.claimsAbout(subjectID)
	result := make([]models.Claim, len(internal))
	for i, c := range internal {
		result[i] = c.Clone()
	}
	return result
}

// metaClaimsAbout returns internal claim pointers targeting a claim or finding ID.
// Caller must hold at least RLock.
func (inv *Investigation) metaClaimsAbout(claimID string) []*models.Claim {
	var result []*models.Claim
	for _, c := range inv.claims {
		if c.TargetClaimID == claimID {
			result = append(result, c)
		}
	}
	return result
}

// MetaClaimsAbout returns all meta-claims targeting a claim or finding ID as value copies.
func (inv *Investigation) MetaClaimsAbout(claimID string) []models.Claim {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	internal := inv.metaClaimsAbout(claimID)
	result := make([]models.Claim, len(internal))
	for i, c := range internal {
		result[i] = c.Clone()
	}
	return result
}

// AnalyzeClaim performs depth-limited recursive credibility analysis.
func (inv *Investigation) AnalyzeClaim(claimID string) (*ClaimAnalysis, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return inv.analyzeClaim(claimID, 0)
}

// AnalyzeAll analyzes all claims.
func (inv *Investigation) AnalyzeAll() []*ClaimAnalysis {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
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
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return fmt.Sprintf("Investigation: %s\n  Actors: %d, Subjects: %d, Claims: %d, Evidence: %d, Findings: %d",
		inv.ResearchQuestion,
		len(inv.actors), len(inv.subjects), len(inv.claims), len(inv.evidence), len(inv.findings))
}

// --- Mutation methods ---

// UpdateActorReliability sets the base reliability for an actor.
func (inv *Investigation) UpdateActorReliability(id string, r float64) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	a, ok := inv.actors[id]
	if !ok {
		return fmt.Errorf("actor %q not found", id)
	}
	a.BaseReliability = r
	return nil
}

// AddActorConflict appends a conflict of interest to an actor.
func (inv *Investigation) AddActorConflict(id string, c models.ConflictOfInterest) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	a, ok := inv.actors[id]
	if !ok {
		return fmt.Errorf("actor %q not found", id)
	}
	a.Conflicts = append(a.Conflicts, c)
	return nil
}

// UpdateActorNotes sets the notes for an actor.
func (inv *Investigation) UpdateActorNotes(id string, notes string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	a, ok := inv.actors[id]
	if !ok {
		return fmt.Errorf("actor %q not found", id)
	}
	a.Notes = notes
	return nil
}

// UpdateSubjectNotes sets the notes for a subject.
func (inv *Investigation) UpdateSubjectNotes(id string, notes string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	s, ok := inv.subjects[id]
	if !ok {
		return fmt.Errorf("subject %q not found", id)
	}
	s.Notes = notes
	return nil
}

// UpdateClaimNotes sets the notes for a claim.
func (inv *Investigation) UpdateClaimNotes(id string, notes string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	c, ok := inv.claims[id]
	if !ok {
		return fmt.Errorf("claim %q not found", id)
	}
	c.Notes = notes
	return nil
}

// UpdateEvidenceWeight sets the weight for a piece of evidence.
func (inv *Investigation) UpdateEvidenceWeight(id string, w float64) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	e, ok := inv.evidence[id]
	if !ok {
		return fmt.Errorf("evidence %q not found", id)
	}
	e.Weight = &w
	return nil
}

// UpdateEvidenceNotes sets the notes for a piece of evidence.
func (inv *Investigation) UpdateEvidenceNotes(id string, notes string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	e, ok := inv.evidence[id]
	if !ok {
		return fmt.Errorf("evidence %q not found", id)
	}
	e.Notes = notes
	return nil
}

// UpdateFindingNotes sets the notes for a finding.
func (inv *Investigation) UpdateFindingNotes(id string, notes string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	f, ok := inv.findings[id]
	if !ok {
		return fmt.Errorf("finding %q not found", id)
	}
	f.Notes = notes
	return nil
}
