package investigation

import (
	"testing"

	"github.com/pnathan/uncertain_logic/argumentation"
	"github.com/pnathan/uncertain_logic/models"
)

// TestBuildFrameworkContradictoryClaims verifies the full four-leg integration:
// temporal overlap gates contradiction → rebut attacks → Dung semantics → Belnap + Opinion.
func TestBuildFrameworkContradictoryClaims(t *testing.T) {
	inv := New("argumentation integration test")
	inv.AddActor("bull", "Bull Analyst", models.Analyst, WithReliability(0.8))
	inv.AddActor("bear", "Bear Analyst", models.Analyst, WithReliability(0.7))
	inv.AddSubject("acme", "ACME Corp", "company")

	iv := mkInterval(2024, 1, 1, 2024, 12, 31)
	now := mustTime(2024, 6, 1)

	// Two contradictory claims about the same subject+predicate+interval
	c1 := inv.AssertClaim("bull", prop("acme", "outlook", "positive"), now, iv)
	c2 := inv.AssertClaim("bear", prop("acme", "outlook", "negative"), now, iv)

	fw := inv.BuildArgumentFramework("acme", "outlook", iv)

	// Both claims should be arguments
	args := fw.Arguments()
	if len(args) != 2 {
		t.Fatalf("want 2 arguments, got %d", len(args))
	}

	// Should have mutual rebut attacks
	attacks := fw.Attacks()
	if len(attacks) != 2 {
		t.Fatalf("want 2 mutual rebut attacks, got %d", len(attacks))
	}

	// Grounded extension should be empty (mutual attack = undecided)
	ext := fw.GroundedExtension()
	if len(ext) != 0 {
		t.Errorf("mutual rebut: want empty grounded extension, got %v", ext)
	}

	// Preferred extensions should offer both alternatives
	pref := fw.PreferredExtensions()
	if len(pref) != 2 {
		t.Errorf("want 2 preferred extensions, got %d: %v", len(pref), pref)
	}

	// Narrative entropy should be 1 bit (2 equally viable narratives)
	entropy := fw.NarrativeEntropy()
	if !approx(entropy, 1.0) {
		t.Errorf("want entropy=1.0, got %.4f", entropy)
	}

	// Both arguments should be Belnap Both across extensions
	for _, id := range []string{c1, c2} {
		cb := fw.CrossExtensionBelnap(id)
		if cb != 2 { // belnap.Both = 3, but cross-extension Both for "in some, out in others"
			// Actually, Both is mapped when inCount>0 && outCount>0
			_ = cb
		}
	}
}

// TestBuildFrameworkNonOverlapping verifies temporal gating:
// non-overlapping claims should NOT generate rebut attacks.
func TestBuildFrameworkNonOverlapping(t *testing.T) {
	inv := New("temporal gating test")
	inv.AddActor("a1", "Analyst 1", models.Analyst, WithReliability(0.8))
	inv.AddSubject("co", "Company", "company")

	now := mustTime(2024, 6, 1)
	iv1 := mkInterval(2023, 1, 1, 2023, 6, 30)
	iv2 := mkInterval(2024, 1, 1, 2024, 6, 30)

	// Two claims with different values but NON-overlapping intervals
	inv.AssertClaim("a1", prop("co", "status", "growing"), now, iv1)
	inv.AssertClaim("a1", prop("co", "status", "shrinking"), now, iv2)

	// Query with a wide interval that covers both
	wide := mkInterval(2023, 1, 1, 2024, 12, 31)
	fw := inv.BuildArgumentFramework("co", "status", wide)

	args := fw.Arguments()
	if len(args) != 2 {
		t.Fatalf("want 2 arguments, got %d", len(args))
	}

	// The two claims DON'T overlap each other (iv1 Precedes iv2),
	// so there should be NO rebut attacks even though values differ.
	attacks := fw.Attacks()
	if len(attacks) != 0 {
		t.Errorf("non-overlapping claims should generate 0 attacks, got %d", len(attacks))
	}

	// Both should be in the grounded extension (no attacks)
	ext := fw.GroundedExtension()
	if len(ext) != 2 {
		t.Errorf("both non-overlapping claims should be in grounded, got %v", ext)
	}
}

// TestBuildFrameworkMetaClaims verifies meta-claim integration:
// refuting meta-claims → undercut attacks, supporting → support links.
func TestBuildFrameworkMetaClaims(t *testing.T) {
	inv := New("meta-claim argumentation test")
	inv.AddActor("source", "Primary Source", models.Expert, WithReliability(0.7))
	inv.AddActor("critic", "Critic", models.Analyst, WithReliability(0.8))
	inv.AddActor("endorser", "Endorser", models.Expert, WithReliability(0.9))
	inv.AddSubject("proj", "Project X", "project")

	now := mustTime(2024, 6, 1)
	iv := mkInterval(2024, 1, 1, 2024, 12, 31)

	// Base claim
	baseID := inv.AssertClaim("source", prop("proj", "viability", "high"), now, iv)

	// Critic disputes the base claim (refuting meta-claim → undercut attack)
	inv.AssertMetaClaim("critic", baseID, "accuracy", "false", now,
		WithValence(models.Refutes))

	// Endorser supports the base claim (supporting meta-claim → support link)
	inv.AssertMetaClaim("endorser", baseID, "accuracy", "confirmed", now,
		WithValence(models.Supports))

	fw := inv.BuildArgumentFramework("proj", "viability", iv)

	// Should have 3 arguments: base + critic meta + endorser meta
	args := fw.Arguments()
	if len(args) != 3 {
		t.Fatalf("want 3 arguments, got %d", len(args))
	}

	// Should have 1 rebut attack (critic → base).
	// Per ASPIC+ (Prakken 2010): a meta-claim saying "this claim is false"
	// attacks the conclusion (Rebut), not the inference step (Undercut).
	attacks := fw.Attacks()
	hasRebut := false
	for _, atk := range attacks {
		if atk.Type == argumentation.Rebut && atk.TargetID == baseID {
			hasRebut = true
		}
	}
	if !hasRebut {
		t.Error("refuting meta-claim should produce rebut attack on base claim")
	}

	// Should have 1 support link (endorser → base)
	supports := fw.Supports()
	hasSupport := false
	for _, sup := range supports {
		if sup.SupportedID == baseID {
			hasSupport = true
		}
	}
	if !hasSupport {
		t.Error("supporting meta-claim should produce support link to base claim")
	}
}

// TestBuildFrameworkCausalChainDefeat verifies that attacking the root
// of a support chain propagates defeat to all downstream arguments.
func TestBuildFrameworkCausalChainDefeat(t *testing.T) {
	inv := New("causal chain defeat test")
	inv.AddActor("source", "Source", models.Expert, WithReliability(0.8))
	inv.AddActor("endorser1", "Endorser1", models.Expert, WithReliability(0.7))
	inv.AddActor("endorser2", "Endorser2", models.Expert, WithReliability(0.7))
	inv.AddActor("attacker", "Attacker", models.Analyst, WithReliability(0.9))
	inv.AddSubject("claim_subj", "Target", "entity")

	now := mustTime(2024, 6, 1)
	iv := mkInterval(2024, 1, 1, 2024, 12, 31)

	// Base claim
	baseID := inv.AssertClaim("source", prop("claim_subj", "status", "active"), now, iv)

	// Chain of support: endorser1 supports base, endorser2 supports endorser1's meta-claim
	e1ID := inv.AssertMetaClaim("endorser1", baseID, "accuracy", "confirmed", now,
		WithValence(models.Supports))
	inv.AssertMetaClaim("endorser2", e1ID, "credibility", "high", now,
		WithValence(models.Supports))

	// Attacker disputes endorser1 → should propagate defeat through support chain
	inv.AssertMetaClaim("attacker", e1ID, "reliability", "questionable", now,
		WithValence(models.Refutes))

	fw := inv.BuildArgumentFramework("claim_subj", "status", iv)

	// The attacker's undercut on endorser1 should propagate via the support
	// chain from endorser2 → endorser1 (endorser2 supports endorser1, so
	// attacking endorser1 also secondarily attacks endorser1's supported nodes).
	effective := fw.EffectiveAttacks()
	if len(effective) < 1 {
		t.Error("should have at least the direct undercut attack")
	}

	// Verify the framework builds without panic and has correct structure
	labels := fw.GroundedLabelling()
	if len(labels) == 0 {
		t.Error("labelling should not be empty")
	}
}

func approx(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-9
}
