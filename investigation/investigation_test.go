package investigation

import (
	"strings"
	"testing"
	"time"

	"uncertain_logic/belnap"
	"uncertain_logic/models"
	"uncertain_logic/temporal"
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
	inv1.LoadActors([]*models.Actor{
		{ID: "senator", Name: "Senator X", SourceType: models.Expert, BaseReliability: 0.7},
	})
	inv1.LoadSubjects([]*models.Subject{
		{ID: "bill42", Name: "Bill S.42", SubjectType: "legislation"},
	})

	iv := mkInterval(2024, 1, 1, 2024, 12, 31)
	w := 0.85
	costClaim := &models.Claim{
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
	ev1 := &models.Evidence{
		ID:      "ev_001",
		ClaimID: "claim_cost_001",
		Content: "CBO score: $4.8B",
		Valence: models.Supports,
		Weight:  &w,
	}
	inv1.LoadClaims([]*models.Claim{costClaim})
	inv1.LoadEvidence([]*models.Evidence{ev1})

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
	inv2.LoadActors([]*models.Actor{
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

// TestDanglingEvidenceID: a claim referencing a non-loaded evidence ID gracefully skips it.
func TestDanglingEvidenceID(t *testing.T) {
	inv := New("dangling evidence test")
	inv.AddActor("a", "Actor", models.Analyst, WithReliability(0.7))
	inv.AddSubject("s", "S", "general")
	iv := mkInterval(2023, 1, 1, 2023, 12, 31)

	c := &models.Claim{
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
	inv.LoadClaims([]*models.Claim{c})

	a, err := inv.AnalyzeClaim("claim_dangling")
	if err != nil {
		t.Fatalf("dangling evidence ID should not cause error: %v", err)
	}
	// Evidence was skipped; counts stay at zero
	if a.SupportingCount != 0 || a.RefutingCount != 0 {
		t.Errorf("dangling evidence should be skipped, got s=%d r=%d", a.SupportingCount, a.RefutingCount)
	}
}
