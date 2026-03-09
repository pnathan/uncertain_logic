package investigation

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/temporal"
)

func mustTime(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func mkInterval(y1, m1, d1, y2, m2, d2 int) temporal.EventInterval {
	s := mustTime(y1, m1, d1)
	e := mustTime(y2, m2, d2)
	return temporal.EventInterval{Start: &s, End: &e}
}

func prop(subject, predicate, value string) models.Proposition {
	return models.Proposition{Subject: subject, Predicate: predicate, Value: value}
}

// TestNoEvidence: a claim with no evidence → Neither
func TestNoEvidence(t *testing.T) {
	inv := New("no-evidence test")
	inv.AddActor("anon", "Anonymous", models.Anonymous, WithReliability(0.5))
	inv.AddSubject("widget", "Widget Corp", "company")

	now := mustTime(2024, 1, 1)
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	id := inv.AssertClaim("anon", prop("widget", "profitable", "true"), now, iv)

	a, err := inv.AnalyzeClaim(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.BelnapStatus != belnap.Neither {
		t.Errorf("expected Neither, got %v", a.BelnapStatus)
	}
}

// TestAssertFact: AssertFact → high credibility
func TestAssertFact(t *testing.T) {
	inv := New("fact test")
	inv.AddSubject("acme", "Acme Corp", "company")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	id := inv.AssertFact(prop("acme", "revenue", "100M"), iv)

	a, err := inv.AnalyzeClaim(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.Credibility.ExpectedProbability() < 0.8 {
		t.Errorf("fact credibility too low: %.3f", a.Credibility.ExpectedProbability())
	}
}

// TestQReturnsResults: Q returns results for known subject+predicate
func TestQReturnsResults(t *testing.T) {
	inv := New("Q test")
	inv.AddSubject("acme", "Acme Corp", "company")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	inv.AssertFact(prop("acme", "revenue", "100M"), iv)

	results := inv.Q("acme", "revenue", iv)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
}

// TestContradicted: SEC filing refutes, Reddit post supports → Both, weighted toward disbelief
func TestContradicted(t *testing.T) {
	inv := New("Motley Fool scenario")
	inv.AddActor("analyst", "Bull Analyst", models.Analyst,
		WithReliability(0.7),
		WithConflict(models.ConflictOfInterest{
			Description: "Long position in stock",
			Direction:   models.Long,
			Disclosed:   false,
			SubjectID:   "stockx",
		}),
	)
	inv.AddSubject("stockx", "Stock X", "equity")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 15)

	claimID := inv.AssertClaim("analyst", prop("stockx", "valuation", "hidden gem"), now, iv)

	// SEC filing refutes it
	inv.AddEvidence(claimID, "SEC filing shows overstated revenue", models.Refutes,
		WithWeight(0.9), WithEvidenceURL("https://sec.gov/filing/123"))

	// Reddit post supports it
	inv.AddEvidence(claimID, "Reddit: this stock is great", models.Supports,
		WithWeight(0.2))

	a, err := inv.AnalyzeClaim(claimID)
	if err != nil {
		t.Fatal(err)
	}
	if a.BelnapStatus != belnap.Both {
		t.Errorf("expected Both, got %v", a.BelnapStatus)
	}
	ep := a.Credibility.ExpectedProbability()
	if ep > 0.6 {
		t.Errorf("credibility EP=%.3f should lean toward disbelief (<0.6)", ep)
	}
	// Conflict of interest (undisclosed long) reduces actor trust
	actor := inv.actors["analyst"]
	adjusted := actor.AdjustedReliability("stockx")
	if adjusted >= actor.BaseReliability {
		t.Errorf("adjusted reliability should be lower than base: %.3f vs %.3f", adjusted, actor.BaseReliability)
	}
}

// TestFiveQuestions: output contains all five answer sections
func TestFiveQuestions(t *testing.T) {
	inv := New("five questions test")
	inv.AddActor("senator", "Senator X", models.Expert, WithReliability(0.7))
	inv.AddSubject("bill42", "Bill S.42", "legislation")

	now := mustTime(2024, 3, 1)
	iv := mkInterval(2024, 1, 1, 2024, 12, 31)
	id := inv.AssertClaim("senator", prop("bill42", "cost", "5B"), now, iv,
		WithSourceURL("https://senate.gov/press-release"),
		WithClaimNotes("Floor speech"),
	)

	a, err := inv.AnalyzeClaim(id)
	if err != nil {
		t.Fatal(err)
	}
	out := a.FiveQuestions()

	for _, r := range []string{"Q1", "Q2", "Q3", "Q4", "Q5", "Senator X"} {
		if !strings.Contains(out, r) {
			t.Errorf("FiveQuestions output missing %q:\n%s", r, out)
		}
	}
}

// TestConflictOfInterestReducesCredibility
func TestConflictOfInterestReducesCredibility(t *testing.T) {
	inv := New("conflict test")
	inv.AddActor("inst", "Institution", models.Institutional, WithReliability(0.9))
	inv.AddActor("biased", "Biased Analyst", models.Analyst,
		WithReliability(0.9),
		WithConflict(models.ConflictOfInterest{
			Description: "Undisclosed long position",
			Direction:   models.Long,
			Disclosed:   false,
		}),
	)
	inv.AddSubject("co", "Company", "company")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	id1 := inv.AssertClaim("inst", prop("co", "outlook", "positive"), now, iv)
	id2 := inv.AssertClaim("biased", prop("co", "outlook", "positive"), now, iv)

	a1, _ := inv.AnalyzeClaim(id1)
	a2, _ := inv.AnalyzeClaim(id2)

	ep1 := a1.Credibility.ExpectedProbability()
	ep2 := a2.Credibility.ExpectedProbability()
	if ep2 >= ep1 {
		t.Errorf("biased analyst EP=%.3f should be less than institutional EP=%.3f", ep2, ep1)
	}
}

// TestGWBScenario: two editorials about different terms, both can be True
func TestGWBScenario(t *testing.T) {
	inv := New("GWB editorial scenario")
	inv.AddActor("writer", "Political Writer", models.Journalist, WithReliability(0.75))
	inv.AddSubject("gwb", "George W. Bush", "politician")

	term1 := mkInterval(1985, 1, 1, 1992, 12, 31)
	t1 := mustTime(1989, 6, 1)
	id1 := inv.AssertClaim("writer", prop("gwb", "political_stance", "moderate"), t1, term1)

	term2 := mkInterval(1993, 1, 1, 2001, 1, 1)
	t2 := mustTime(1995, 3, 1)
	id2 := inv.AssertClaim("writer", prop("gwb", "political_stance", "conservative"), t2, term2)

	a1, _ := inv.AnalyzeClaim(id1)
	a2, _ := inv.AnalyzeClaim(id2)

	// Neither should be Both (no contradicting evidence within each interval)
	if a1.BelnapStatus == belnap.Both {
		t.Errorf("GWB 1985-1992 claim should not be Both: %v", a1.BelnapStatus)
	}
	if a2.BelnapStatus == belnap.Both {
		t.Errorf("GWB 1993-2001 claim should not be Both: %v", a2.BelnapStatus)
	}

	// ActorBeliefHistory reveals revision
	claims, revisions := inv.ActorBeliefHistory("writer", "gwb")
	if len(claims) != 2 {
		t.Errorf("expected 2 claims, got %d", len(claims))
	}
	if len(revisions) == 0 {
		t.Error("expected at least one belief revision")
	}
	for _, rev := range revisions {
		if !rev.TemporallyConsistent {
			t.Errorf("GWB revision should be temporally consistent (non-overlapping intervals)")
		}
	}

	// SubjectTimeline should show two separate time slices
	timeline := inv.SubjectTimeline("gwb")
	if len(timeline) != 2 {
		t.Errorf("expected 2 time slices, got %d", len(timeline))
	}

	_ = id1
	_ = id2
}

// TestPoliticalMetaClaim: senator asserts cost; journalist attributes; fact-checker disputes
func TestPoliticalMetaClaim(t *testing.T) {
	inv := New("political meta-claim scenario")
	inv.AddActor("senator", "Senator X", models.Expert, WithReliability(0.7))
	inv.AddActor("journalist", "Journalist J", models.Journalist, WithReliability(0.8))
	inv.AddActor("factchecker", "Fact Checker", models.Expert, WithReliability(0.85))
	inv.AddSubject("bill42", "Bill S.42", "legislation")

	iv := mkInterval(2024, 1, 1, 2024, 12, 31)
	t1 := mustTime(2024, 2, 1)
	t2 := mustTime(2024, 2, 5)
	t3 := mustTime(2024, 2, 10)

	senatorClaimID := inv.AssertClaim("senator", prop("bill42", "cost", "5B"), t1, iv)

	// Journalist attributes the claim (supports it as real)
	inv.AssertMetaClaim("journalist", senatorClaimID, "authorship", "stated", t2,
		WithValence(models.Supports))

	// Fact-checker disputes accuracy
	inv.AssertMetaClaim("factchecker", senatorClaimID, "accuracy", "false", t3,
		WithValence(models.Refutes))

	a, err := inv.AnalyzeClaim(senatorClaimID)
	if err != nil {
		t.Fatal(err)
	}
	if a.BelnapStatus != belnap.Both {
		t.Errorf("senator cost claim should be Both (journalist supports, fact-checker refutes), got %v", a.BelnapStatus)
	}

	out := a.FiveQuestions()
	if !strings.Contains(out, "Q1") || !strings.Contains(out, "Q5") {
		t.Errorf("FiveQuestions missing expected sections:\n%s", out)
	}
}

// TestCycleSafety: mutual refutation cycle must terminate
func TestCycleSafety(t *testing.T) {
	inv := New("cycle safety test")
	inv.AddActor("a", "Actor A", models.Analyst, WithReliability(0.6))
	inv.AddActor("b", "Actor B", models.Analyst, WithReliability(0.6))
	inv.AddSubject("topic", "Topic", "general")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	claimA := inv.AssertClaim("a", prop("topic", "status", "good"), now, iv)
	claimB := inv.AssertClaim("b", prop("topic", "status", "bad"), now, iv)

	// A says B is wrong; B says A is wrong
	inv.AssertMetaClaim("a", claimB, "accuracy", "false", now, WithValence(models.Refutes))
	inv.AssertMetaClaim("b", claimA, "accuracy", "false", now, WithValence(models.Refutes))

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := inv.AnalyzeClaim(claimA)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		_, err = inv.AnalyzeClaim(claimB)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cycle detection: AnalyzeClaim did not terminate within 5s")
	}
}

// TestLoadAndRegister: load corpus → analyze → register finding → reload in new investigation
func TestLoadAndRegister(t *testing.T) {
	// --- Session 1: original investigation ---
	inv1 := New("Is the cost estimate credible?")
	inv1.LoadActors([]models.Actor{
		{ID: "senator", Name: "Senator X", SourceType: models.Expert, BaseReliability: 0.7},
	})
	inv1.LoadSubjects([]models.Subject{
		{ID: "bill42", Name: "Bill S.42", SubjectType: "legislation"},
	})

	iv := mkInterval(2024, 1, 1, 2024, 12, 31)
	w := 0.85
	costClaim := models.Claim{
		ID:            "claim_cost_001",
		ActorID:       "senator",
		SubjectID:     "bill42",
		Predicate:     "cost",
		Value:         "5B",
		Content:       "bill42 cost = 5B",
		ClaimType:     models.Factual,
		AssertionTime: mustTime(2024, 2, 1),
		EventInterval: iv,
		EvidenceIDs:   []string{"ev_001"},
	}
	ev1 := models.Evidence{
		ID:      "ev_001",
		ClaimID: "claim_cost_001",
		Content: "CBO score: $4.8B",
		Valence: models.Supports,
		Weight:  &w,
	}
	inv1.LoadClaims([]models.Claim{costClaim})
	inv1.LoadEvidence([]models.Evidence{ev1})

	a, err := inv1.AnalyzeClaim("claim_cost_001")
	if err != nil {
		t.Fatal(err)
	}
	finding := inv1.RegisterAnalysis(a)

	if finding.ID == "" {
		t.Fatal("finding has no ID")
	}
	if finding.ClaimID != "claim_cost_001" {
		t.Errorf("finding.ClaimID = %q, want claim_cost_001", finding.ClaimID)
	}

	// "Persist" — extract what the storage layer would write back
	persistedFindings := inv1.Findings()
	if len(persistedFindings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(persistedFindings))
	}

	// --- Session 2: oversight investigation loads prior finding ---
	inv2 := New("Does the oversight body accept the prior finding?")
	inv2.LoadActors([]models.Actor{
		{ID: "oversight", Name: "Oversight Board", SourceType: models.Regulator, BaseReliability: 0.9},
	})
	inv2.LoadFindings(persistedFindings)

	// Oversight disputes the prior finding
	inv2.AssertMetaClaim("oversight", finding.ID, "validity", "disputed", mustTime(2024, 6, 1),
		WithValence(models.Refutes))

	// AnalyzeClaim on the finding ID resolves without recomputing
	resolved, err := inv2.AnalyzeClaim(finding.ID)
	if err != nil {
		t.Fatalf("AnalyzeClaim on finding ID: %v", err)
	}
	if resolved.BelnapStatus != finding.BelnapStatus {
		t.Errorf("resolved BelnapStatus = %v, want %v", resolved.BelnapStatus, finding.BelnapStatus)
	}

	// The oversight meta-claim is visible
	metas := inv2.MetaClaimsAbout(finding.ID)
	if len(metas) != 1 {
		t.Errorf("expected 1 meta-claim about finding, got %d", len(metas))
	}
}

// TestReadAccessors: Actors/Subjects/Claims/Evidence/Findings round-trip counts
func TestReadAccessors(t *testing.T) {
	inv := New("accessor test")
	inv.AddActor("a1", "Actor 1", models.Analyst)
	inv.AddActor("a2", "Actor 2", models.Journalist)
	inv.AddSubject("s1", "Subject 1", "company")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	id := inv.AssertClaim("a1", prop("s1", "foo", "bar"), mustTime(2024, 1, 1), iv)
	inv.AddEvidence(id, "some evidence", models.Supports)

	a, _ := inv.AnalyzeClaim(id)
	inv.RegisterAnalysis(a)

	if len(inv.Actors()) != 2 {
		t.Errorf("Actors() = %d, want 2", len(inv.Actors()))
	}
	if len(inv.Subjects()) != 1 {
		t.Errorf("Subjects() = %d, want 1", len(inv.Subjects()))
	}
	if len(inv.Claims()) != 1 {
		t.Errorf("Claims() = %d, want 1", len(inv.Claims()))
	}
	if len(inv.Evidence()) != 1 {
		t.Errorf("Evidence() = %d, want 1", len(inv.Evidence()))
	}
	if len(inv.Findings()) != 1 {
		t.Errorf("Findings() = %d, want 1", len(inv.Findings()))
	}
}

// TestSummary: Summary returns actor/subject/claim counts
func TestSummary(t *testing.T) {
	inv := New("summary test")
	inv.AddActor("a", "Actor", models.Analyst)
	inv.AddSubject("s", "Subject", "general")
	inv.AssertClaim("a", prop("s", "foo", "bar"), mustTime(2024, 1, 1), temporal.Open("now"))

	s := inv.Summary()
	if !strings.Contains(s, "Actors: 1") {
		t.Errorf("Summary missing actor count: %s", s)
	}
	if !strings.Contains(s, "Claims: 1") {
		t.Errorf("Summary missing claim count: %s", s)
	}
}

// TestWithOptions exercises all option constructors for actors, claims, and evidence.
func TestWithOptions(t *testing.T) {
	inv := New("options test")
	inv.AddActor("a", "Actor", models.Analyst, WithActorNotes("domain expert"))
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	id := inv.AssertClaim("a", prop("s", "quality", "high"), mustTime(2024, 1, 1), iv,
		WithClaimType(models.Evaluative),
		WithSourceDescription("internal audit"),
	)
	inv.AddEvidence(id, "supporting doc", models.Supports,
		WithSourceClaimID("prior_claim_001"),
		WithEvidenceNotes("from public record"),
	)
	// Just verify no panics and claim was stored
	if len(inv.Claims()) != 1 {
		t.Errorf("expected 1 claim, got %d", len(inv.Claims()))
	}
}

// TestAssertFactWithOption exercises the option-execution branch inside AssertFact.
func TestAssertFactWithOption(t *testing.T) {
	inv := New("fact option test")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	// Passing an option causes the loop body inside AssertFact to execute.
	inv.AssertFact(prop("s", "p", "v"), iv, WithSourceURL("https://example.com"))
	if len(inv.Claims()) != 1 {
		t.Errorf("expected 1 claim, got %d", len(inv.Claims()))
	}
}

// TestAssertFactReuseSystemActor: second AssertFact call reuses the existing _system actor.
func TestAssertFactReuseSystemActor(t *testing.T) {
	inv := New("fact twice test")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	inv.AssertFact(prop("s", "p", "v1"), iv)
	inv.AssertFact(prop("s", "p", "v2"), iv) // _system actor already present
	if len(inv.Claims()) != 2 {
		t.Errorf("expected 2 claims, got %d", len(inv.Claims()))
	}
}

// TestQNoMatch: Q with no matching claims returns Neither.
func TestQNoMatch(t *testing.T) {
	inv := New("Q no match")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	results := inv.Q("unknown_subject", "unknown_predicate", iv)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Belnap != belnap.Neither {
		t.Errorf("no-match Q should be Neither, got %v", results[0].Belnap)
	}
}

// TestQBothStatus: a contradicted claim (Both) causes Q to set both support and refute counts.
func TestQBothStatus(t *testing.T) {
	inv := New("Q both test")
	inv.AddActor("a", "Actor", models.Expert, WithReliability(0.8))
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	id := inv.AssertClaim("a", prop("s", "outlook", "positive"), mustTime(2024, 1, 1), iv)
	inv.AddEvidence(id, "positive signal", models.Supports, WithWeight(0.8))
	inv.AddEvidence(id, "negative signal", models.Refutes, WithWeight(0.8))

	results := inv.Q("s", "outlook", iv)
	if results[0].Belnap != belnap.Both {
		t.Errorf("contradicted claim in Q should yield Both, got %v", results[0].Belnap)
	}
}

// TestQFalseStatus: a fully refuted claim causes Q to increment refuteCount.
func TestQFalseStatus(t *testing.T) {
	inv := New("Q false test")
	inv.AddActor("a", "Strong Source", models.Regulator, WithReliability(0.95))
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	id := inv.AssertClaim("a", prop("s", "claim", "value"), mustTime(2024, 1, 1), iv)
	inv.AddEvidence(id, "strong refutation", models.Refutes, WithWeight(0.95))

	results := inv.Q("s", "claim", iv)
	if results[0].Belnap != belnap.False {
		t.Errorf("refuted claim in Q should yield False, got %v", results[0].Belnap)
	}
}

// TestQueryResultOps: And, Or, Not on QueryResults.
func TestQueryResultOps(t *testing.T) {
	inv := New("ops test")
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	inv.AssertFact(prop("s", "x", "true"), iv)
	inv.AssertFact(prop("s", "y", "true"), iv)

	rx := inv.Q("s", "x", iv)[0]
	ry := inv.Q("s", "y", iv)[0]

	andResult := And(rx, ry)
	if andResult.Belnap != belnap.And(rx.Belnap, ry.Belnap) {
		t.Errorf("And Belnap mismatch")
	}
	orResult := Or(rx, ry)
	if orResult.Belnap != belnap.Or(rx.Belnap, ry.Belnap) {
		t.Errorf("Or Belnap mismatch")
	}
	notResult := Not(rx)
	if notResult.Belnap != belnap.Not(rx.Belnap) {
		t.Errorf("Not Belnap mismatch")
	}
}

// TestAnalyzeAll: AnalyzeAll returns one ClaimAnalysis per claim.
func TestAnalyzeAll(t *testing.T) {
	inv := New("analyze all test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.7))
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	inv.AssertClaim("a", prop("s", "foo", "bar"), mustTime(2024, 1, 1), iv)
	inv.AssertClaim("a", prop("s", "baz", "qux"), mustTime(2024, 1, 1), iv)

	results := inv.AnalyzeAll()
	if len(results) != 2 {
		t.Errorf("expected 2 analyses, got %d", len(results))
	}
}

// TestSubjectTimelineOpenIntervals: open intervals all overlap → single slice; exercises unionInterval nil path.
func TestSubjectTimelineOpenIntervals(t *testing.T) {
	inv := New("open interval timeline test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.7))
	inv.AddSubject("s", "S", "general")
	t1 := mustTime(2024, 1, 1)
	t2 := mustTime(2024, 6, 1)

	inv.AssertClaim("a", prop("s", "status", "good"), t1, temporal.Open("vague window 1"))
	inv.AssertClaim("a", prop("s", "status", "great"), t2, temporal.Open("vague window 2"))

	// Both open → Overlapping = true → merged into 1 slice; unionInterval with nil bounds
	timeline := inv.SubjectTimeline("s")
	if len(timeline) != 1 {
		t.Errorf("open intervals should merge into 1 slice, got %d", len(timeline))
	}
}

// TestFiveQuestionsMetaClaim: FiveQuestions on a meta-claim exercises the TargetClaimID branch
// (Q3 shows "claim:..." instead of a subject name) and the Desc branch (Q4 shows Open() desc).
func TestFiveQuestionsMetaClaim(t *testing.T) {
	inv := New("meta fivequestions test")
	inv.AddActor("journalist", "J", models.Journalist, WithReliability(0.75))
	iv := mkInterval(2024, 1, 1, 2024, 12, 31)

	targetID := inv.AssertClaim("journalist", prop("bill42", "cost", "5B"), mustTime(2024, 1, 1), iv)
	metaID := inv.AssertMetaClaim("journalist", targetID, "accuracy", "disputed", mustTime(2024, 2, 1),
		WithValence(models.Refutes))

	a, err := inv.AnalyzeClaim(metaID)
	if err != nil {
		t.Fatal(err)
	}
	out := a.FiveQuestions()

	// Q3: SubjectID is empty → shows "claim:<targetID>"
	if !strings.Contains(out, "claim:") {
		t.Errorf("FiveQuestions Q3 should reference target claim ID:\n%s", out)
	}
	// Q4: EventInterval is Open("same as target") → Desc branch
	if !strings.Contains(out, "same as target") {
		t.Errorf("FiveQuestions Q4 should show open interval description:\n%s", out)
	}
}

// TestLowReliabilityActor: actor with reliability < 0.3 and no evidence → Neither.
func TestLowReliabilityActor(t *testing.T) {
	inv := New("low reliability test")
	inv.AddActor("troll", "Troll", models.Troll, WithReliability(0.1))
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	id := inv.AssertClaim("troll", prop("s", "status", "great"), mustTime(2024, 1, 1), iv)
	a, err := inv.AnalyzeClaim(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.BelnapStatus != belnap.Neither {
		t.Errorf("low reliability + no evidence: expected Neither, got %v", a.BelnapStatus)
	}
}

// TestNeutralEvidence: neutral evidence increments NeutralCount but does not affect Belnap.
func TestNeutralEvidence(t *testing.T) {
	inv := New("neutral evidence test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.7))
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	id := inv.AssertClaim("a", prop("s", "foo", "bar"), mustTime(2024, 1, 1), iv)
	inv.AddEvidence(id, "background context, no clear valence", models.Neutral)

	a, err := inv.AnalyzeClaim(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.NeutralCount != 1 {
		t.Errorf("expected NeutralCount=1, got %d", a.NeutralCount)
	}
	if a.SupportingCount != 0 || a.RefutingCount != 0 {
		t.Errorf("neutral evidence should not affect supporting/refuting counts")
	}
}

// TestAnalyzeClaimNotFound: AnalyzeClaim on an unknown ID returns an error.
func TestAnalyzeClaimNotFound(t *testing.T) {
	inv := New("not found test")
	_, err := inv.AnalyzeClaim("no_such_claim")
	if err == nil {
		t.Error("expected error for nonexistent claim ID, got nil")
	}
}

// TestSubjectTimelineEmpty: SubjectTimeline on a subject with no claims returns nil.
func TestSubjectTimelineEmpty(t *testing.T) {
	inv := New("empty timeline test")
	inv.AddSubject("s", "S", "general")
	timeline := inv.SubjectTimeline("s")
	if timeline != nil {
		t.Errorf("expected nil for subject with no claims, got %v", timeline)
	}
}

// TestSubjectTimelineOverlappingConcrete: two claims with overlapping concrete intervals
// are merged into one slice; exercises the unionInterval date comparison branches.
func TestSubjectTimelineOverlappingConcrete(t *testing.T) {
	inv := New("overlapping concrete timeline test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.7))
	inv.AddSubject("s", "S", "general")

	// iv2 starts before iv1 and ends after iv1 (b.Start < a.Start, b.End > a.End)
	// so unionInterval must expand both the start and the end.
	iv1 := mkInterval(2023, 3, 1, 2023, 9, 30)
	iv2 := mkInterval(2023, 1, 1, 2023, 12, 31)
	t1 := mustTime(2024, 1, 1)
	t2 := mustTime(2024, 2, 1)

	inv.AssertClaim("a", prop("s", "status", "early"), t1, iv1)
	inv.AssertClaim("a", prop("s", "status", "late"), t2, iv2)

	timeline := inv.SubjectTimeline("s")
	// Overlapping → merged into 1 slice; union should span Jan–Dec
	if len(timeline) != 1 {
		t.Errorf("overlapping intervals should merge into 1 slice, got %d", len(timeline))
	}
	slice := timeline[0]
	if slice.Interval.Start == nil || slice.Interval.End == nil {
		t.Fatal("merged interval should have concrete bounds")
	}
	// Union should span iv2's start (Jan 1) to iv2's end (Dec 31)
	if !slice.Interval.Start.Equal(mustTime(2023, 1, 1)) {
		t.Errorf("merged start = %v, want 2023-01-01", *slice.Interval.Start)
	}
	if !slice.Interval.End.Equal(mustTime(2023, 12, 31)) {
		t.Errorf("merged end = %v, want 2023-12-31", *slice.Interval.End)
	}
}

// TestRefutingClaimValenceProducesDisbelief: a claim with Valence=Refutes should
// produce E[p] < 0.5, because the actor prior now starts from DogmaticFalse.
func TestRefutingClaimValenceProducesDisbelief(t *testing.T) {
	inv := New("refuting valence test")
	inv.AddActor("sec", "SEC", models.Regulator, WithReliability(0.95))
	inv.AddSubject("co", "Company", "company")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	// Bearish claim: asserts the "positive" proposition is false.
	bearishID := inv.AssertClaim("sec", prop("co", "outlook", "positive"), now, iv,
		WithValence(models.Refutes))
	inv.AddEvidence(bearishID, "Revenue declined 40% YoY", models.Refutes, WithWeight(0.8))

	// Bullish claim: asserts the "positive" proposition is true (default).
	bullishID := inv.AssertClaim("sec", prop("co", "outlook", "positive"), now, iv)
	inv.AddEvidence(bullishID, "Revenue grew 40% YoY", models.Supports, WithWeight(0.8))

	bearish, err := inv.AnalyzeClaim(bearishID)
	if err != nil {
		t.Fatal(err)
	}
	bullish, err := inv.AnalyzeClaim(bullishID)
	if err != nil {
		t.Fatal(err)
	}

	bearishEP := bearish.Credibility.ExpectedProbability()
	bullishEP := bullish.Credibility.ExpectedProbability()

	if bearishEP >= 0.5 {
		t.Errorf("bearish claim E[p]=%.3f, want < 0.5", bearishEP)
	}
	if bullishEP <= 0.5 {
		t.Errorf("bullish claim E[p]=%.3f, want > 0.5", bullishEP)
	}
	if bearishEP >= bullishEP {
		t.Errorf("bearish E[p]=%.3f should be less than bullish E[p]=%.3f", bearishEP, bullishEP)
	}
}

// TestRefutingValenceNoEvidenceStillBearish: a Refutes-valence claim with no evidence
// should still lean bearish (E[p] < 0.5) rather than defaulting to bullish.
func TestRefutingValenceNoEvidenceStillBearish(t *testing.T) {
	inv := New("refuting no-evidence test")
	inv.AddActor("sec", "SEC", models.Regulator, WithReliability(0.95))
	inv.AddSubject("co", "Company", "company")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	id := inv.AssertClaim("sec", prop("co", "outlook", "positive"), now, iv,
		WithValence(models.Refutes))

	a, err := inv.AnalyzeClaim(id)
	if err != nil {
		t.Fatal(err)
	}
	ep := a.Credibility.ExpectedProbability()
	if ep >= 0.5 {
		t.Errorf("refuting claim with no evidence: E[p]=%.3f, want < 0.5", ep)
	}
}

// TestRefutingValenceNoEvidenceBelnapFalse: a Refutes-valence claim from a reliable
// actor with no evidence should have BelnapStatus=False, not True.
func TestRefutingValenceNoEvidenceBelnapFalse(t *testing.T) {
	inv := New("belnap valence test")
	inv.AddActor("sec", "SEC", models.Regulator, WithReliability(0.95))
	inv.AddSubject("co", "Company", "company")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	id := inv.AssertClaim("sec", prop("co", "outlook", "positive"), now, iv,
		WithValence(models.Refutes))

	a, err := inv.AnalyzeClaim(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.BelnapStatus != belnap.False {
		t.Errorf("refuting claim from reliable actor: BelnapStatus=%v, want F", a.BelnapStatus)
	}
}

// TestAndOpMultipliesExpectedProbability: And on QueryResults should produce
// E[A∧B] = E[A]·E[B] (Jøsang multiplication), not fused evidence.
func TestAndOpMultipliesExpectedProbability(t *testing.T) {
	inv := New("and op test")
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	inv.AssertFact(prop("s", "x", "true"), iv)
	inv.AssertFact(prop("s", "y", "true"), iv)

	rx := inv.Q("s", "x", iv)[0]
	ry := inv.Q("s", "y", iv)[0]

	andResult := And(rx, ry)
	epX := rx.Opinion.ExpectedProbability()
	epY := ry.Opinion.ExpectedProbability()
	wantEP := epX * epY
	gotEP := andResult.Opinion.ExpectedProbability()
	// Allow tolerance for floating point
	if diff := gotEP - wantEP; diff > 0.001 || diff < -0.001 {
		t.Errorf("And E[p] = %.4f, want E[x]·E[y] = %.4f·%.4f = %.4f",
			gotEP, epX, epY, wantEP)
	}
}

// TestOrOpCoMultipliesExpectedProbability: Or on QueryResults should produce
// E[A∨B] = E[A]+E[B]−E[A]·E[B] (Jøsang co-multiplication).
func TestOrOpCoMultipliesExpectedProbability(t *testing.T) {
	inv := New("or op test")
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	inv.AssertFact(prop("s", "x", "true"), iv)
	inv.AssertFact(prop("s", "y", "true"), iv)

	rx := inv.Q("s", "x", iv)[0]
	ry := inv.Q("s", "y", iv)[0]

	orResult := Or(rx, ry)
	epX := rx.Opinion.ExpectedProbability()
	epY := ry.Opinion.ExpectedProbability()
	wantEP := epX + epY - epX*epY
	gotEP := orResult.Opinion.ExpectedProbability()
	if diff := gotEP - wantEP; diff > 0.001 || diff < -0.001 {
		t.Errorf("Or E[p] = %.4f, want E[x]+E[y]-E[x]·E[y] = %.4f",
			gotEP, wantEP)
	}
}

// TestDanglingEvidenceID: a claim referencing a non-loaded evidence ID gracefully skips it.
func TestDanglingEvidenceID(t *testing.T) {
	inv := New("dangling evidence test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.7))
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	c := models.Claim{
		ID:            "claim_dangling",
		ActorID:       "a",
		SubjectID:     "s",
		Predicate:     "foo",
		Value:         "bar",
		Content:       "s foo = bar",
		ClaimType:     models.Factual,
		AssertionTime: mustTime(2024, 1, 1),
		EventInterval: iv,
		EvidenceIDs:   []string{"evidence_that_was_never_loaded"},
	}
	inv.LoadClaims([]models.Claim{c})

	a, err := inv.AnalyzeClaim("claim_dangling")
	if err != nil {
		t.Fatalf("dangling evidence ID should not cause error: %v", err)
	}
	// Evidence was skipped; counts stay at zero
	if a.SupportingCount != 0 || a.RefutingCount != 0 {
		t.Errorf("dangling evidence should be skipped, got s=%d r=%d", a.SupportingCount, a.RefutingCount)
	}
}

// TestConcurrentAccess: concurrent reads and writes must not race.
// Run with -race to verify: go test -race -run TestConcurrentAccess ./investigation/...
func TestConcurrentAccess(t *testing.T) {
	inv := New("concurrent test")
	inv.AddActor("a1", "Actor 1", models.Analyst, WithReliability(0.7))
	inv.AddSubject("s1", "Subject 1", "company")

	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	var wg sync.WaitGroup
	const goroutines = 10

	// Concurrent writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := inv.AssertClaim("a1", prop("s1", "metric", fmt.Sprintf("v%d", n)),
				mustTime(2024, 1, 1), iv)
			inv.AddEvidence(id, fmt.Sprintf("evidence %d", n), models.Supports)
		}(i)
	}

	// Concurrent readers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv.Actors()
			inv.Subjects()
			inv.Claims()
			inv.Evidence()
			inv.Findings()
			inv.Q("s1", "metric", iv)
			inv.ClaimsAbout("s1")
			inv.MetaClaimsAbout("nonexistent")
			inv.Summary()
			inv.SubjectTimeline("s1")
			inv.ActorBeliefHistory("a1", "s1")
			inv.AnalyzeAll()
		}()
	}

	wg.Wait()
}

// TestMutationMethods: Update* and AddActorConflict methods modify internal state correctly.
func TestMutationMethods(t *testing.T) {
	inv := New("mutation test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.7))
	inv.AddSubject("s", "Subject", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	claimID := inv.AssertClaim("a", prop("s", "foo", "bar"), mustTime(2024, 1, 1), iv)
	evID := inv.AddEvidence(claimID, "some evidence", models.Supports)
	a, _ := inv.AnalyzeClaim(claimID)
	finding := inv.RegisterAnalysis(a)

	// UpdateActorReliability
	if err := inv.UpdateActorReliability("a", 0.9); err != nil {
		t.Fatal(err)
	}
	if err := inv.UpdateActorReliability("missing", 0.5); err == nil {
		t.Error("expected error for missing actor")
	}

	// AddActorConflict
	if err := inv.AddActorConflict("a", models.ConflictOfInterest{
		Description: "test conflict", Direction: models.Long, Disclosed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := inv.AddActorConflict("missing", models.ConflictOfInterest{}); err == nil {
		t.Error("expected error for missing actor")
	}

	// UpdateActorNotes
	if err := inv.UpdateActorNotes("a", "updated notes"); err != nil {
		t.Fatal(err)
	}
	if err := inv.UpdateActorNotes("missing", "x"); err == nil {
		t.Error("expected error for missing actor")
	}

	// UpdateSubjectNotes
	if err := inv.UpdateSubjectNotes("s", "subject notes"); err != nil {
		t.Fatal(err)
	}
	if err := inv.UpdateSubjectNotes("missing", "x"); err == nil {
		t.Error("expected error for missing subject")
	}

	// UpdateClaimNotes
	if err := inv.UpdateClaimNotes(claimID, "claim notes"); err != nil {
		t.Fatal(err)
	}
	if err := inv.UpdateClaimNotes("missing", "x"); err == nil {
		t.Error("expected error for missing claim")
	}

	// UpdateEvidenceWeight
	if err := inv.UpdateEvidenceWeight(evID, 0.95); err != nil {
		t.Fatal(err)
	}
	if err := inv.UpdateEvidenceWeight("missing", 0.5); err == nil {
		t.Error("expected error for missing evidence")
	}

	// UpdateEvidenceNotes
	if err := inv.UpdateEvidenceNotes(evID, "evidence notes"); err != nil {
		t.Fatal(err)
	}
	if err := inv.UpdateEvidenceNotes("missing", "x"); err == nil {
		t.Error("expected error for missing evidence")
	}

	// UpdateFindingNotes
	if err := inv.UpdateFindingNotes(finding.ID, "finding notes"); err != nil {
		t.Fatal(err)
	}
	if err := inv.UpdateFindingNotes("missing", "x"); err == nil {
		t.Error("expected error for missing finding")
	}
}

// TestConcurrentWriteWriteRace: many goroutines mutating the same Investigation simultaneously.
// The race detector should find no issues.
func TestConcurrentWriteWriteRace(t *testing.T) {
	inv := New("write-write race test")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	const N = 50

	var wg sync.WaitGroup

	// Many goroutines adding actors with unique IDs
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("actor_%d", n)
			inv.AddActor(id, fmt.Sprintf("Actor %d", n), models.Analyst, WithReliability(0.7))
		}(i)
	}

	// Many goroutines adding subjects
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("subj_%d", n)
			inv.AddSubject(id, fmt.Sprintf("Subject %d", n), "company")
		}(i)
	}

	wg.Wait()

	// Now concurrent claims from those actors
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			actorID := fmt.Sprintf("actor_%d", n)
			subjID := fmt.Sprintf("subj_%d", n%10)
			claimID := inv.AssertClaim(actorID, prop(subjID, "metric", fmt.Sprintf("v%d", n)),
				mustTime(2024, 1, 1), iv)
			// Immediately add evidence to the claim we just created
			inv.AddEvidence(claimID, fmt.Sprintf("evidence for %d", n), models.Supports, WithWeight(0.8))
			inv.AddEvidence(claimID, fmt.Sprintf("counter-evidence for %d", n), models.Refutes, WithWeight(0.3))
		}(i)
	}
	wg.Wait()

	// Concurrent AssertFact (all touch _system actor creation path)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.AssertFact(prop(fmt.Sprintf("subj_%d", n%10), "ground_truth", fmt.Sprintf("v%d", n)), iv)
		}(i)
	}
	wg.Wait()

	// Concurrent meta-claims
	claims := inv.Claims()
	for i := 0; i < len(claims) && i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			targetID := claims[n].ID
			inv.AssertMetaClaim("actor_0", targetID, "accuracy", "disputed",
				mustTime(2024, 6, 1), WithValence(models.Refutes))
		}(i)
	}
	wg.Wait()

	if got := len(inv.Actors()); got < N {
		t.Errorf("expected at least %d actors, got %d", N, got)
	}
}

// TestConcurrentReadWriteHeavy: sustained mixed read/write traffic from many goroutines.
func TestConcurrentReadWriteHeavy(t *testing.T) {
	inv := New("heavy read-write test")
	inv.AddActor("writer", "Writer", models.Analyst, WithReliability(0.8))
	inv.AddSubject("target", "Target", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	const writers = 20
	const readers = 30
	const opsPerGoroutine = 20

	var wg sync.WaitGroup

	// Writers: add claims, evidence, meta-claims, facts, register findings
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				claimID := inv.AssertClaim("writer",
					prop("target", fmt.Sprintf("prop_%d", n), fmt.Sprintf("val_%d_%d", n, j)),
					mustTime(2024, 1, 1), iv)
				inv.AddEvidence(claimID, "supporting", models.Supports)
				inv.AddEvidence(claimID, "refuting", models.Refutes)

				if j%5 == 0 {
					inv.AssertFact(prop("target", fmt.Sprintf("fact_%d_%d", n, j), "true"), iv)
				}
				if j%3 == 0 {
					a, err := inv.AnalyzeClaim(claimID)
					if err == nil {
						inv.RegisterAnalysis(a)
					}
				}
			}
		}(i)
	}

	// Readers: continuous querying while writers are active
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				inv.Q("target", fmt.Sprintf("prop_%d", n%writers), iv)
				inv.ClaimsAbout("target")
				inv.MetaClaimsAbout("nonexistent")
				inv.Actors()
				inv.Subjects()
				inv.Claims()
				inv.Evidence()
				inv.Findings()
				inv.Summary()
				inv.SubjectTimeline("target")
				inv.ActorBeliefHistory("writer", "target")
				inv.AnalyzeAll()
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentMutationMethods: mutation methods racing against each other and readers.
func TestConcurrentMutationMethods(t *testing.T) {
	inv := New("mutation race test")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	// Set up some entities to mutate
	inv.AddActor("a1", "Actor 1", models.Analyst, WithReliability(0.5))
	inv.AddSubject("s1", "Subject 1", "company")
	claimID := inv.AssertClaim("a1", prop("s1", "metric", "value"), mustTime(2024, 1, 1), iv)
	evID := inv.AddEvidence(claimID, "evidence", models.Supports)
	a, _ := inv.AnalyzeClaim(claimID)
	finding := inv.RegisterAnalysis(a)

	const N = 30
	var wg sync.WaitGroup

	// Concurrent reliability updates
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.UpdateActorReliability("a1", float64(n)/float64(N))
		}(i)
	}

	// Concurrent conflict additions
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.AddActorConflict("a1", models.ConflictOfInterest{
				Description: fmt.Sprintf("conflict %d", n),
				Direction:   models.Long,
				Disclosed:   n%2 == 0,
			})
		}(i)
	}

	// Concurrent notes updates
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.UpdateActorNotes("a1", fmt.Sprintf("notes %d", n))
			inv.UpdateSubjectNotes("s1", fmt.Sprintf("notes %d", n))
			inv.UpdateClaimNotes(claimID, fmt.Sprintf("notes %d", n))
			inv.UpdateEvidenceNotes(evID, fmt.Sprintf("notes %d", n))
			inv.UpdateEvidenceWeight(evID, float64(n)/float64(N))
			inv.UpdateFindingNotes(finding.ID, fmt.Sprintf("notes %d", n))
		}(i)
	}

	// Concurrent readers while mutations are happening
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv.Actors()
			inv.Claims()
			inv.Evidence()
			inv.Findings()
			inv.AnalyzeClaim(claimID)
			inv.Q("s1", "metric", iv)
		}()
	}

	wg.Wait()
}

// TestConcurrentLoadAndQuery: bulk loads racing against queries.
func TestConcurrentLoadAndQuery(t *testing.T) {
	inv := New("load race test")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	const N = 20

	var wg sync.WaitGroup

	// Concurrent LoadActors
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.LoadActors([]models.Actor{
				{ID: fmt.Sprintf("loaded_actor_%d", n), Name: fmt.Sprintf("LA %d", n),
					SourceType: models.Expert, BaseReliability: 0.8},
			})
		}(i)
	}

	// Concurrent LoadSubjects
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.LoadSubjects([]models.Subject{
				{ID: fmt.Sprintf("loaded_subj_%d", n), Name: fmt.Sprintf("LS %d", n), SubjectType: "co"},
			})
		}(i)
	}

	// Concurrent LoadClaims
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.LoadClaims([]models.Claim{
				{ID: fmt.Sprintf("loaded_claim_%d", n), ActorID: "loaded_actor_0",
					SubjectID: "loaded_subj_0", Predicate: "p", Value: "v",
					Content: "text", ClaimType: models.Factual,
					AssertionTime: mustTime(2024, 1, 1), EventInterval: iv},
			})
		}(i)
	}

	// Concurrent LoadEvidence
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.LoadEvidence([]models.Evidence{
				{ID: fmt.Sprintf("loaded_ev_%d", n), ClaimID: "loaded_claim_0",
					Content: "ev", Valence: models.Supports},
			})
		}(i)
	}

	// Concurrent LoadFindings
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			inv.LoadFindings([]Finding{
				{ID: fmt.Sprintf("loaded_finding_%d", n), ClaimID: "loaded_claim_0"},
			})
		}(i)
	}

	// Concurrent readers during all those loads
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv.Actors()
			inv.Subjects()
			inv.Claims()
			inv.Evidence()
			inv.Findings()
			inv.Summary()
		}()
	}

	wg.Wait()

	if got := len(inv.Actors()); got < N {
		t.Errorf("expected at least %d loaded actors, got %d", N, got)
	}
}

// TestConcurrentAnalyzeWithMetaClaims: recursive analysis with concurrent meta-claim creation.
// Exercises the depth-limited MetaClaimsAbout path under contention.
func TestConcurrentAnalyzeWithMetaClaims(t *testing.T) {
	inv := New("meta-claim race test")
	inv.AddActor("a", "Analyst", models.Analyst, WithReliability(0.7))
	inv.AddActor("b", "Checker", models.Expert, WithReliability(0.85))
	inv.AddSubject("s", "Subject", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	// Create a base claim
	baseID := inv.AssertClaim("a", prop("s", "outlook", "positive"), mustTime(2024, 1, 1), iv)
	inv.AddEvidence(baseID, "quarterly report", models.Supports, WithWeight(0.8))

	const N = 30
	var wg sync.WaitGroup

	// Goroutines adding meta-claims targeting the base claim
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			valence := models.Supports
			if n%2 == 0 {
				valence = models.Refutes
			}
			inv.AssertMetaClaim("b", baseID, "accuracy", fmt.Sprintf("v%d", n),
				mustTime(2024, 6, 1), WithValence(valence))
		}(i)
	}

	// Goroutines analyzing the base claim while meta-claims are being added
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This triggers recursive analysis through MetaClaimsAbout
			inv.AnalyzeClaim(baseID)
		}()
	}

	// Goroutines calling Q which internally calls analyzeClaim
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv.Q("s", "outlook", iv)
		}()
	}

	wg.Wait()

	// Verify base claim is analyzable and has Both status (supports + refutes)
	a, err := inv.AnalyzeClaim(baseID)
	if err != nil {
		t.Fatal(err)
	}
	if a.SupportingCount == 0 && a.RefutingCount == 0 {
		t.Error("expected meta-claims to have registered as supporting/refuting")
	}
}

// TestReturnedCopiesAreIndependent: verify that values returned from accessors
// cannot mutate internal state.
func TestReturnedCopiesAreIndependent(t *testing.T) {
	inv := New("copy independence test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.7),
		WithConflict(models.ConflictOfInterest{Description: "original", Disclosed: true}))
	inv.AddSubject("s", "Subject", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	claimID := inv.AssertClaim("a", prop("s", "metric", "value"), mustTime(2024, 1, 1), iv)
	inv.AddEvidence(claimID, "evidence", models.Supports, WithWeight(0.8))

	// Mutate returned actors
	actors := inv.Actors()
	actors[0].Name = "MUTATED"
	actors[0].BaseReliability = 0.0
	actors[0].Conflicts[0].Description = "MUTATED"

	// Internal state should be unchanged
	actors2 := inv.Actors()
	for _, a := range actors2 {
		if a.ID == "a" {
			if a.Name == "MUTATED" {
				t.Error("mutating returned Actor.Name affected internal state")
			}
			if a.BaseReliability == 0.0 {
				t.Error("mutating returned Actor.BaseReliability affected internal state")
			}
			if len(a.Conflicts) > 0 && a.Conflicts[0].Description == "MUTATED" {
				t.Error("mutating returned Actor.Conflicts affected internal state")
			}
		}
	}

	// Mutate returned claims
	claims := inv.Claims()
	claims[0].Notes = "MUTATED"
	claims[0].EvidenceIDs = append(claims[0].EvidenceIDs, "injected_id")

	claims2 := inv.Claims()
	for _, c := range claims2 {
		if c.ID == claimID {
			if c.Notes == "MUTATED" {
				t.Error("mutating returned Claim.Notes affected internal state")
			}
			if len(c.EvidenceIDs) != 1 {
				t.Errorf("mutating returned Claim.EvidenceIDs affected internal state: len=%d", len(c.EvidenceIDs))
			}
		}
	}

	// Mutate returned evidence
	evidence := inv.Evidence()
	if len(evidence) > 0 && evidence[0].Weight != nil {
		*evidence[0].Weight = 0.0
	}

	evidence2 := inv.Evidence()
	for _, e := range evidence2 {
		if e.Weight != nil && *e.Weight == 0.0 {
			t.Error("mutating returned Evidence.Weight affected internal state")
		}
	}

	// Mutate returned ClaimAnalysis
	analysis, err := inv.AnalyzeClaim(claimID)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Actor != nil {
		analysis.Actor.Name = "MUTATED_ANALYSIS"
	}

	analysis2, _ := inv.AnalyzeClaim(claimID)
	if analysis2.Actor != nil && analysis2.Actor.Name == "MUTATED_ANALYSIS" {
		t.Error("mutating returned ClaimAnalysis.Actor affected internal state")
	}

	// Mutate returned findings
	finding := inv.RegisterAnalysis(analysis)
	finding.Notes = "MUTATED"

	findings := inv.Findings()
	for _, f := range findings {
		if f.Notes == "MUTATED" {
			t.Error("mutating returned Finding.Notes affected internal state")
		}
	}

	// Mutate returned QueryResult.MatchedClaims
	results := inv.Q("s", "metric", iv)
	if len(results) > 0 && len(results[0].MatchedClaims) > 0 {
		results[0].MatchedClaims[0].Notes = "MUTATED_Q"
	}
	results2 := inv.Q("s", "metric", iv)
	if len(results2) > 0 && len(results2[0].MatchedClaims) > 0 {
		if results2[0].MatchedClaims[0].Notes == "MUTATED_Q" {
			t.Error("mutating returned QueryResult.MatchedClaims affected internal state")
		}
	}

	// Mutate returned timeline
	timeline := inv.SubjectTimeline("s")
	if len(timeline) > 0 && len(timeline[0].Claims) > 0 {
		timeline[0].Claims[0].Notes = "MUTATED_TL"
	}
	timeline2 := inv.SubjectTimeline("s")
	if len(timeline2) > 0 && len(timeline2[0].Claims) > 0 {
		if timeline2[0].Claims[0].Notes == "MUTATED_TL" {
			t.Error("mutating returned TimeSlice.Claims affected internal state")
		}
	}

	// Mutate returned ClaimAnalysis evidence counts
	if analysis.PositiveEvidence != 0 || analysis.NegativeEvidence != 0 {
		// Evidence counts should be set (non-zero for claims with evidence)
		_ = analysis.PositiveEvidence
	}

	// Mutate returned belief history
	histClaims, _ := inv.ActorBeliefHistory("a", "s")
	if len(histClaims) > 0 {
		histClaims[0].Notes = "MUTATED_HIST"
	}
	histClaims2, _ := inv.ActorBeliefHistory("a", "s")
	if len(histClaims2) > 0 && histClaims2[0].Notes == "MUTATED_HIST" {
		t.Error("mutating returned ActorBeliefHistory claims affected internal state")
	}
}

// TestBijectionSixIndependentActors: 6 different actors (reliability 2/3), each 1 claim,
// same subject/predicate/interval. Q() → CBF across 6 per-actor opinions.
// Each actor's analyzeClaim: r=1.0, s=0 → OFE(1,0,0.5) → {b≈0.333, d=0, u≈0.667}.
// But trust discount: FromReliability(2/3) gives b=(2/3-0.5)*2=1/3, so trust.b=1/3.
// Discounted assertion: b=1/3*1=1/3, d=0, u=2/3.
// EvidenceCounts: r=W*(1/3)/(2/3)=1.0, s=0.
// OFE(1,0,0.5) per actor. CBF of 6 = OFE(6,0,0.5). EP=0.875.
func TestBijectionSixIndependentActors(t *testing.T) {
	inv := New("bijection test")
	inv.AddSubject("co", "Company", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("actor_%d", i)
		inv.AddActor(id, fmt.Sprintf("Actor %d", i), models.Analyst,
			WithReliability(2.0/3.0))
		inv.AssertClaim(id, prop("co", "outlook", "positive"), now, iv)
	}

	results := inv.Q("co", "outlook", iv)
	ep := results[0].Opinion.ExpectedProbability()
	if diff := ep - 0.875; diff > 0.01 || diff < -0.01 {
		t.Errorf("6 independent actors EP=%.4f, want ≈0.875", ep)
	}
}

// TestBijectionSameActorIdempotent: 1 actor makes 6 identical claims.
// Q() → ABF within actor (idempotent) → EP = single-claim EP.
func TestBijectionSameActorIdempotent(t *testing.T) {
	inv := New("idempotent test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(2.0/3.0))
	inv.AddSubject("co", "Company", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	// Single claim for reference
	invRef := New("reference")
	invRef.AddActor("a", "Actor", models.Analyst, WithReliability(2.0/3.0))
	invRef.AddSubject("co", "Company", "company")
	invRef.AssertClaim("a", prop("co", "outlook", "positive"), now, iv)
	refResults := invRef.Q("co", "outlook", iv)
	refEP := refResults[0].Opinion.ExpectedProbability()

	// 6 claims from same actor
	for i := 0; i < 6; i++ {
		inv.AssertClaim("a", prop("co", "outlook", "positive"), now, iv)
	}
	results := inv.Q("co", "outlook", iv)
	ep := results[0].Opinion.ExpectedProbability()

	if diff := ep - refEP; diff > 0.01 || diff < -0.01 {
		t.Errorf("same actor 6 claims EP=%.4f, want single-claim EP=%.4f (ABF idempotency)", ep, refEP)
	}
}

// TestSameActorClaimsUseABF: 1 actor, 3 claims. Q() EP = single claim EP.
func TestSameActorClaimsUseABF(t *testing.T) {
	inv := New("ABF test")
	inv.AddActor("a", "Actor", models.Expert, WithReliability(0.8))
	inv.AddSubject("co", "Company", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	for i := 0; i < 3; i++ {
		inv.AssertClaim("a", prop("co", "outlook", "positive"), now, iv)
	}

	results := inv.Q("co", "outlook", iv)
	ep := results[0].Opinion.ExpectedProbability()

	// Single claim reference
	invRef := New("ref")
	invRef.AddActor("a", "Actor", models.Expert, WithReliability(0.8))
	invRef.AddSubject("co", "Company", "company")
	invRef.AssertClaim("a", prop("co", "outlook", "positive"), now, iv)
	refEP := invRef.Q("co", "outlook", iv)[0].Opinion.ExpectedProbability()

	if diff := ep - refEP; diff > 0.01 || diff < -0.01 {
		t.Errorf("same actor 3 claims EP=%.4f, want single-claim EP=%.4f", ep, refEP)
	}
}

// TestCrossActorClaimsUseCBF: 3 actors, 1 claim each. Q() uncertainty < any single claim.
func TestCrossActorClaimsUseCBF(t *testing.T) {
	inv := New("CBF test")
	inv.AddSubject("co", "Company", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("actor_%d", i)
		inv.AddActor(id, fmt.Sprintf("Actor %d", i), models.Analyst, WithReliability(0.7))
		inv.AssertClaim(id, prop("co", "outlook", "positive"), now, iv)
	}

	results := inv.Q("co", "outlook", iv)
	fusedU := results[0].Opinion.Uncertainty

	// Single actor reference
	invRef := New("ref")
	invRef.AddActor("actor_0", "Actor 0", models.Analyst, WithReliability(0.7))
	invRef.AddSubject("co", "Company", "company")
	invRef.AssertClaim("actor_0", prop("co", "outlook", "positive"), now, iv)
	singleU := invRef.Q("co", "outlook", iv)[0].Opinion.Uncertainty

	if fusedU >= singleU {
		t.Errorf("cross-actor fused uncertainty %.4f should be < single actor uncertainty %.4f", fusedU, singleU)
	}
}

// TestMetaClaimTrustDiscountPropagation: meta-claim from 0.5 reliability actor
// contributes less evidence than from 0.9 reliability actor.
func TestMetaClaimTrustDiscountPropagation(t *testing.T) {
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	setup := func(metaReliability float64) float64 {
		inv := New("meta trust test")
		inv.AddActor("base", "Base Actor", models.Analyst, WithReliability(0.7))
		inv.AddActor("meta", "Meta Actor", models.Expert, WithReliability(metaReliability))
		inv.AddSubject("co", "Company", "company")

		baseID := inv.AssertClaim("base", prop("co", "outlook", "positive"), now, iv)
		inv.AssertMetaClaim("meta", baseID, "accuracy", "confirmed", now,
			WithValence(models.Supports))

		a, _ := inv.AnalyzeClaim(baseID)
		return a.Credibility.ExpectedProbability()
	}

	epLow := setup(0.5)
	epHigh := setup(0.9)

	// Higher reliability meta-actor should produce higher EP (more trusted support)
	if epHigh <= epLow {
		t.Errorf("high-reliability meta EP=%.4f should be > low-reliability meta EP=%.4f", epHigh, epLow)
	}
}

// TestDefaultWeightIsOne: claim with nil-weight evidence uses weight=1.0.
func TestDefaultWeightIsOne(t *testing.T) {
	inv := New("default weight test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.8))
	inv.AddSubject("co", "Company", "company")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)
	now := mustTime(2024, 1, 1)

	id := inv.AssertClaim("a", prop("co", "outlook", "positive"), now, iv)
	inv.AddEvidence(id, "supporting doc", models.Supports) // no WithWeight → default

	a, err := inv.AnalyzeClaim(id)
	if err != nil {
		t.Fatal(err)
	}

	// The evidence should have contributed weight 1.0 to positive evidence
	// Actor r + evidence 1.0 should be reflected in PositiveEvidence
	if a.PositiveEvidence < 1.0 {
		t.Errorf("PositiveEvidence=%.4f, should include at least 1.0 from default-weight evidence", a.PositiveEvidence)
	}
}
