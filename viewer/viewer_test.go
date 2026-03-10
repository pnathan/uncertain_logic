package viewer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/investigation"
	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/temporal"
)

// ---------------------------------------------------------------------------
// 1. Helper functions
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"empty string", "", 10, ""},
		{"shorter than n", "hello", 10, "hello"},
		{"exactly n", "hello", 5, "hello"},
		{"longer than n", "hello world", 5, "hello..."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.s, tc.n)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q; want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}

func TestEsc(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"ampersand", "a & b", "a &amp; b"},
		{"less than", "a < b", "a &lt; b"},
		{"greater than", "a > b", "a &gt; b"},
		{"double quote", `a "b" c`, "a &quot;b&quot; c"},
		{"all together", `<a href="x">&`, `&lt;a href=&quot;x&quot;&gt;&amp;`},
		{"no special chars", "plain text", "plain text"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := esc(tc.in)
			if got != tc.want {
				t.Errorf("esc(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBelnapClass(t *testing.T) {
	tests := []struct {
		name string
		val  belnap.Value
		want string
	}{
		{"True", belnap.True, "t"},
		{"False", belnap.False, "f"},
		{"Both", belnap.Both, "b"},
		{"Neither", belnap.Neither, "n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := belnapClass(tc.val)
			if got != tc.want {
				t.Errorf("belnapClass(%v) = %q; want %q", tc.val, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers to build test investigations
// ---------------------------------------------------------------------------

func overlappingInterval() temporal.EventInterval {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	return temporal.EventInterval{Start: &start, End: &end}
}

func nonOverlappingEarly() temporal.EventInterval {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 6, 30, 0, 0, 0, 0, time.UTC)
	return temporal.EventInterval{Start: &start, End: &end}
}

func nonOverlappingLate() temporal.EventInterval {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	return temporal.EventInterval{Start: &start, End: &end}
}

func newTestServer(t *testing.T, setup func(inv *investigation.Investigation)) *server {
	t.Helper()
	inv := investigation.New("test question")
	setup(inv)
	s := &server{inv: inv}
	s.rebuild()
	return s
}

// ---------------------------------------------------------------------------
// 2. buildFullFramework tests
// ---------------------------------------------------------------------------

func TestBuildFullFramework_SingleClaim(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
	})

	args := s.fw.Arguments()
	attacks := s.fw.Attacks()
	if len(args) != 1 {
		t.Errorf("expected 1 argument, got %d", len(args))
	}
	if len(attacks) != 0 {
		t.Errorf("expected 0 attacks, got %d", len(attacks))
	}
}

func TestBuildFullFramework_ContradictoryClaims(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Analyst Two", models.Analyst, investigation.WithReliability(0.7))
		inv.AddSubject("s1", "Subject One", "entity")
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
		inv.AssertClaim("a2", models.Proposition{Subject: "s1", Predicate: "status", Value: "inactive"},
			time.Now(), overlappingInterval())
	})

	args := s.fw.Arguments()
	attacks := s.fw.Attacks()
	if len(args) != 2 {
		t.Errorf("expected 2 arguments, got %d", len(args))
	}
	// Mutual rebut = 2 attacks
	if len(attacks) < 2 {
		t.Errorf("expected at least 2 rebut attacks for contradictory claims, got %d", len(attacks))
	}
}

func TestBuildFullFramework_NonOverlappingNoAttacks(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Analyst Two", models.Analyst, investigation.WithReliability(0.7))
		inv.AddSubject("s1", "Subject One", "entity")
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), nonOverlappingEarly())
		inv.AssertClaim("a2", models.Proposition{Subject: "s1", Predicate: "status", Value: "inactive"},
			time.Now(), nonOverlappingLate())
	})

	args := s.fw.Arguments()
	attacks := s.fw.Attacks()
	if len(args) != 2 {
		t.Errorf("expected 2 arguments, got %d", len(args))
	}
	if len(attacks) != 0 {
		t.Errorf("expected 0 attacks for non-overlapping intervals, got %d", len(attacks))
	}
}

func TestBuildFullFramework_MetaClaimRefutes(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Critic", models.Expert, investigation.WithReliability(0.9))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "revenue", Value: "50M"},
			time.Now(), overlappingInterval())
		inv.AssertMetaClaim("a2", claimID, "accuracy", "false", time.Now(),
			investigation.WithValence(models.Refutes))
	})

	attacks := s.fw.Attacks()
	foundRebut := false
	for _, atk := range attacks {
		if strings.Contains(atk.Desc, "meta-claim disputes") {
			foundRebut = true
			break
		}
	}
	if !foundRebut {
		t.Errorf("expected a rebut attack from meta-claim with Refutes valence, attacks: %+v", attacks)
	}
}

func TestBuildFullFramework_MetaClaimSupports(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Endorser", models.Expert, investigation.WithReliability(0.9))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "revenue", Value: "50M"},
			time.Now(), overlappingInterval())
		inv.AssertMetaClaim("a2", claimID, "accuracy", "true", time.Now(),
			investigation.WithValence(models.Supports))
	})

	supports := s.fw.Supports()
	foundSupport := false
	for _, sup := range supports {
		if strings.Contains(sup.Desc, "meta-claim supports") {
			foundSupport = true
			break
		}
	}
	if !foundSupport {
		t.Errorf("expected a support link from meta-claim with Supports valence, supports: %+v", supports)
	}
}

func TestBuildFullFramework_MetaClaimConflict(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Source", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Critic", models.Expert, investigation.WithReliability(0.9))
		inv.AddActor("a3", "Endorser", models.Expert, investigation.WithReliability(0.9))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "revenue", Value: "50M"},
			time.Now(), overlappingInterval())
		// Opposing meta-claims about the same target
		inv.AssertMetaClaim("a2", claimID, "accuracy", "false", time.Now(),
			investigation.WithValence(models.Refutes))
		inv.AssertMetaClaim("a3", claimID, "accuracy", "true", time.Now(),
			investigation.WithValence(models.Supports))
	})

	attacks := s.fw.Attacks()
	foundOpposing := false
	for _, atk := range attacks {
		if strings.Contains(atk.Desc, "opposing assessments") {
			foundOpposing = true
			break
		}
	}
	if !foundOpposing {
		t.Errorf("expected mutual rebut between opposing meta-claims, attacks: %+v", attacks)
	}
}

func TestBuildFullFramework_ActorCoherence(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		inv.AddSubject("s2", "Subject Two", "entity")
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
		inv.AssertClaim("a1", models.Proposition{Subject: "s2", Predicate: "status", Value: "growing"},
			time.Now(), overlappingInterval())
	})

	supports := s.fw.Supports()
	foundCoherence := false
	for _, sup := range supports {
		if strings.Contains(sup.Desc, "same-actor coherence") {
			foundCoherence = true
			break
		}
	}
	if !foundCoherence {
		t.Errorf("expected a same-actor coherence support link, supports: %+v", supports)
	}
}

func TestBuildFullFramework_NeutralMetaClaimNoLink(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Source", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Observer", models.Journalist, investigation.WithReliability(0.7))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "revenue", Value: "50M"},
			time.Now(), overlappingInterval())
		// Neutral valence meta-claim should produce neither attack nor support
		inv.AssertMetaClaim("a2", claimID, "noted", "acknowledged", time.Now(),
			investigation.WithValence(models.Neutral))
	})

	attacks := s.fw.Attacks()
	supports := s.fw.Supports()
	for _, atk := range attacks {
		if strings.Contains(atk.Desc, "meta-claim") {
			t.Errorf("neutral meta-claim should not produce an attack, got: %+v", atk)
		}
	}
	for _, sup := range supports {
		if strings.Contains(sup.Desc, "meta-claim") {
			t.Errorf("neutral meta-claim should not produce a support, got: %+v", sup)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. buildGraphData tests
// ---------------------------------------------------------------------------

func TestBuildGraphData(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Analyst Two", models.Analyst, investigation.WithReliability(0.7))
		inv.AddSubject("s1", "Subject One", "entity")
		c1 := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
		c2 := inv.AssertClaim("a2", models.Proposition{Subject: "s1", Predicate: "status", Value: "inactive"},
			time.Now(), overlappingInterval())
		// Add a supporting meta-claim to get a support link in the graph
		inv.AssertMetaClaim("a1", c2, "accuracy", "false", time.Now(),
			investigation.WithValence(models.Refutes))
		_ = c1
	})

	gd := s.buildGraphData()

	// Verify nodes
	if len(gd.Nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(gd.Nodes))
	}
	for _, n := range gd.Nodes {
		if n.ID == "" {
			t.Error("node has empty ID")
		}
		if n.Label == "" {
			t.Error("node has empty Label")
		}
		// Actor may be empty for system claims but shouldn't be for test claims
		if n.Belnap == "" {
			t.Errorf("node %s has empty Belnap field", n.ID)
		}
		if n.Status == "" {
			t.Errorf("node %s has empty Status field", n.ID)
		}
		// Group should match Subject
		if n.Group == "" && !n.IsMeta {
			// Non-meta claims should have a group
			t.Errorf("non-meta node %s has empty Group field", n.ID)
		}
	}

	// Verify links include attacks (from contradictory claims and meta-claim)
	hasAttack := false
	for _, l := range gd.Links {
		if l.LinkType == "attack" {
			hasAttack = true
			if l.Source == "" || l.Target == "" {
				t.Error("attack link has empty source or target")
			}
			if l.AttackType == "" {
				t.Error("attack link has empty AttackType")
			}
		}
	}
	if !hasAttack {
		t.Error("expected at least one attack link in graph data")
	}

	// Grounded and Entropy fields should be populated (Entropy >= 0)
	if gd.Grounded == nil {
		// Grounded may be empty (nil slice) but that's acceptable
		// Just ensure it doesn't panic and the field exists
	}
	if gd.Entropy < 0 {
		t.Errorf("expected non-negative entropy, got %f", gd.Entropy)
	}
}

func TestBuildGraphData_IncludesSupports(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Endorser", models.Expert, investigation.WithReliability(0.9))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "revenue", Value: "50M"},
			time.Now(), overlappingInterval())
		inv.AssertMetaClaim("a2", claimID, "accuracy", "true", time.Now(),
			investigation.WithValence(models.Supports))
	})

	gd := s.buildGraphData()
	hasSupport := false
	for _, l := range gd.Links {
		if l.LinkType == "support" {
			hasSupport = true
			if l.Source == "" || l.Target == "" {
				t.Error("support link has empty source or target")
			}
			break
		}
	}
	if !hasSupport {
		t.Error("expected at least one support link in graph data")
	}
}

func TestBuildGraphData_MetaNodeHasIsMeta(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Critic", models.Expert, investigation.WithReliability(0.9))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "revenue", Value: "50M"},
			time.Now(), overlappingInterval())
		inv.AssertMetaClaim("a2", claimID, "accuracy", "false", time.Now(),
			investigation.WithValence(models.Refutes))
	})

	gd := s.buildGraphData()
	foundMeta := false
	for _, n := range gd.Nodes {
		if n.IsMeta {
			foundMeta = true
			break
		}
	}
	if !foundMeta {
		t.Error("expected at least one node with IsMeta=true")
	}
}

// ---------------------------------------------------------------------------
// 4. HTTP handler tests
// ---------------------------------------------------------------------------

func TestHandleCSS(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/viewer.css", nil)
	handleCSS(w, r)

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); ct != "text/css" {
		t.Errorf("Content-Type = %q; want text/css", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "body{") || !strings.Contains(body, "font-family") {
		t.Error("CSS response does not contain expected CSS content")
	}
}

func TestHandleJS(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/viewer.js", nil)
	handleJS(w, r)

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); ct != "application/javascript" {
		t.Errorf("Content-Type = %q; want application/javascript", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "window.addEventListener") {
		t.Error("JS response does not contain expected JavaScript content")
	}
}

func TestHandleOverview(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/overview", nil)
	s.handleOverview(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "test question") {
		t.Error("overview should contain the investigation question")
	}
	if !strings.Contains(body, "Analyst One") {
		t.Error("overview should contain the actor name in the table")
	}
	if !strings.Contains(body, "Actors") {
		t.Error("overview should contain Actors heading")
	}
	if !strings.Contains(body, "Subjects") {
		t.Error("overview should contain Subjects heading")
	}
	if !strings.Contains(body, "Subject One") {
		t.Error("overview should contain the subject name")
	}
}

func TestHandleNode_ValidClaim(t *testing.T) {
	var claimID string
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID = inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
	})

	// Use a real mux so PathValue works with Go 1.22 routing
	mux := http.NewServeMux()
	mux.HandleFunc("GET /htmx/node/{id}", s.handleNode)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/node/"+claimID, nil)
	mux.ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "Claim") {
		t.Error("node response should contain 'Claim' heading")
	}
	if !strings.Contains(body, "Analyst One") {
		t.Error("node response should contain the actor name")
	}
	if !strings.Contains(body, "Belnap") {
		t.Error("node response should contain Belnap status")
	}
}

func TestHandleNode_InvalidID(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /htmx/node/{id}", s.handleNode)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/node/nonexistent_id", nil)
	mux.ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "not found") {
		t.Errorf("expected 'not found' in response for invalid ID, got: %s", body)
	}
}

func TestHandleExtensions(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/extensions", nil)
	s.handleExtensions(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "Grounded Extension") {
		t.Error("extensions response should contain 'Grounded Extension'")
	}
	if !strings.Contains(body, "Preferred Extensions") {
		t.Error("extensions response should contain 'Preferred Extensions'")
	}
	if !strings.Contains(body, "Narrative Entropy") {
		t.Error("extensions response should contain 'Narrative Entropy'")
	}
}

func TestHandleChains(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Endorser", models.Expert, investigation.WithReliability(0.9))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "revenue", Value: "50M"},
			time.Now(), overlappingInterval())
		// Add a support meta-claim to generate a causal chain
		inv.AssertMetaClaim("a2", claimID, "accuracy", "true", time.Now(),
			investigation.WithValence(models.Supports))
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/chains", nil)
	s.handleChains(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "Causal Chains") {
		t.Error("chains response should contain 'Causal Chains'")
	}
}

func TestHandleChains_NoChainsMessage(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		// Single claim, no support links, so no chains
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/chains", nil)
	s.handleChains(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "No causal chains") {
		t.Error("chains response with no support links should contain 'No causal chains'")
	}
}

func TestHandleNode_WithAttackersAndEvidence(t *testing.T) {
	var claimID string
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Critic", models.Expert, investigation.WithReliability(0.9))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID = inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
		// Add an attacker (contradictory claim)
		inv.AssertClaim("a2", models.Proposition{Subject: "s1", Predicate: "status", Value: "inactive"},
			time.Now(), overlappingInterval())
		// Add evidence to the claim
		inv.AddEvidence(claimID, "supporting document A", models.Supports)
		inv.AddEvidence(claimID, "refuting report B", models.Refutes)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /htmx/node/{id}", s.handleNode)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/node/"+claimID, nil)
	mux.ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "Attackers") {
		t.Error("node with attackers should contain 'Attackers' heading")
	}
	if !strings.Contains(body, "Evidence") {
		t.Error("node with evidence should contain 'Evidence' heading")
	}
	if !strings.Contains(body, "ev-supports") {
		t.Error("evidence section should contain supporting evidence CSS class")
	}
	if !strings.Contains(body, "ev-refutes") {
		t.Error("evidence section should contain refuting evidence CSS class")
	}
}

func TestHandleNode_MultiplePreferredExtensions(t *testing.T) {
	var claimID string
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Analyst Two", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		// Two contradictory claims → mutual rebut → 2 preferred extensions
		claimID = inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
		inv.AssertClaim("a2", models.Proposition{Subject: "s1", Predicate: "status", Value: "inactive"},
			time.Now(), overlappingInterval())
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /htmx/node/{id}", s.handleNode)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/node/"+claimID, nil)
	mux.ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "Extension Membership") {
		t.Error("node in contested framework should show Extension Membership section")
	}
	if !strings.Contains(body, "Preferred") {
		t.Error("extension membership should mention Preferred extensions")
	}
}

func TestHandleExtensions_NonEmptyGrounded(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		// Single claim, no attacks → in grounded extension
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/extensions", nil)
	s.handleExtensions(w, r)

	body := w.Body.String()
	// Non-empty grounded should NOT contain the "Empty" message
	if strings.Contains(body, "no arguments are unconditionally defensible") {
		t.Error("single unattacked claim should be in grounded extension, not empty")
	}
	if !strings.Contains(body, "ext-grounded") {
		t.Error("non-empty grounded extension should have ext-grounded CSS class")
	}
	// Should have stable extensions too (single claim)
	if !strings.Contains(body, "Stable") {
		t.Error("extensions page should contain Stable Extensions section")
	}
}

func TestHandleChains_WithDefensibleChain(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddActor("a2", "Endorser", models.Expert, investigation.WithReliability(0.9))
		inv.AddSubject("s1", "Subject One", "entity")
		claimID := inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "revenue", Value: "50M"},
			time.Now(), overlappingInterval())
		// Support link creates a causal chain; no attacks → chain should be defensible
		inv.AssertMetaClaim("a2", claimID, "accuracy", "true", time.Now(),
			investigation.WithValence(models.Supports))
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/htmx/chains", nil)
	s.handleChains(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "chain-defensible") {
		t.Error("unattacked chain should have chain-defensible CSS class")
	}
	if !strings.Contains(body, "Strength") {
		t.Error("chain display should include Strength metric")
	}
	if !strings.Contains(body, "Weakest link") {
		t.Error("chain display should include Weakest link info")
	}
}

func TestHandleIndex(t *testing.T) {
	s := newTestServer(t, func(inv *investigation.Investigation) {
		inv.AddActor("a1", "Analyst One", models.Analyst, investigation.WithReliability(0.8))
		inv.AddSubject("s1", "Subject One", "entity")
		inv.AssertClaim("a1", models.Proposition{Subject: "s1", Predicate: "status", Value: "active"},
			time.Now(), overlappingInterval())
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	s.handleIndex(w, r)

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q; want text/html", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("index should be a full HTML document")
	}
	if !strings.Contains(body, "test question") {
		t.Error("index should contain the investigation question")
	}
	if !strings.Contains(body, "graph-data") {
		t.Error("index should contain graph-data script tag")
	}

	// Verify the embedded JSON is valid
	startTag := `<script id="graph-data" type="application/json">`
	endTag := `</script>`
	startIdx := strings.Index(body, startTag)
	if startIdx < 0 {
		t.Fatal("could not find graph-data script tag")
	}
	jsonStart := startIdx + len(startTag)
	jsonEnd := strings.Index(body[jsonStart:], endTag)
	if jsonEnd < 0 {
		t.Fatal("could not find closing script tag for graph-data")
	}
	jsonStr := body[jsonStart : jsonStart+jsonEnd]
	var gd graphData
	if err := json.Unmarshal([]byte(jsonStr), &gd); err != nil {
		t.Errorf("graph-data JSON is invalid: %v", err)
	}
	if len(gd.Nodes) == 0 {
		t.Error("graph-data should contain at least one node")
	}
}
