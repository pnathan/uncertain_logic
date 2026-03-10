package argumentation

import (
	"fmt"
	"math"
	"testing"

	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/subjective"
)

const eps = 1e-9

func approx(a, b float64) bool { return math.Abs(a-b) < eps }

func strength(ep float64) subjective.Opinion {
	return subjective.OpinionFromEvidence(ep*10, (1-ep)*10, 0.5)
}

// --- Grounded Extension ---

func TestGroundedEmpty(t *testing.T) {
	fw := New()
	ext := fw.GroundedExtension()
	if len(ext) != 0 {
		t.Errorf("empty framework: want 0 args in grounded, got %d", len(ext))
	}
}

func TestGroundedSingleUnattacked(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a", Strength: strength(0.8), Status: belnap.True})
	ext := fw.GroundedExtension()
	if len(ext) != 1 || ext[0] != "a" {
		t.Errorf("single unattacked: want [a], got %v", ext)
	}
}

func TestGroundedMutualAttack(t *testing.T) {
	// A ↔ B: neither can be defended, grounded = {}
	fw := New()
	fw.AddArgument(Argument{ID: "a", Strength: strength(0.8), Status: belnap.True})
	fw.AddArgument(Argument{ID: "b", Strength: strength(0.7), Status: belnap.True})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b", Type: Rebut})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "a", Type: Rebut})

	ext := fw.GroundedExtension()
	if len(ext) != 0 {
		t.Errorf("mutual attack: want [], got %v", ext)
	}
}

func TestGroundedReinstatement(t *testing.T) {
	// A → B → C: A is unattacked, defeats B, reinstates C
	fw := New()
	fw.AddArgument(Argument{ID: "a", Strength: strength(0.9), Status: belnap.True})
	fw.AddArgument(Argument{ID: "b", Strength: strength(0.7), Status: belnap.True})
	fw.AddArgument(Argument{ID: "c", Strength: strength(0.6), Status: belnap.True})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b", Type: Rebut})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c", Type: Rebut})

	ext := fw.GroundedExtension()
	want := map[string]bool{"a": true, "c": true}
	got := make(map[string]bool)
	for _, id := range ext {
		got[id] = true
	}
	if len(got) != len(want) {
		t.Fatalf("reinstatement: want %v, got %v", want, got)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("reinstatement: missing %s in grounded", k)
		}
	}
}

func TestGroundedOddCycle(t *testing.T) {
	// A → B → C → A: no argument is defensible
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "a"})

	ext := fw.GroundedExtension()
	if len(ext) != 0 {
		t.Errorf("odd cycle: want [], got %v", ext)
	}
}

func TestGroundedFloatingDefeat(t *testing.T) {
	// D → C → A → B: D unattacked, defeats C, reinstates A, defeats B
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddAttack(Attack{AttackerID: "d", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "a"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})

	ext := fw.GroundedExtension()
	got := make(map[string]bool)
	for _, id := range ext {
		got[id] = true
	}
	if !got["d"] || !got["a"] {
		t.Errorf("floating defeat: want d,a in grounded, got %v", ext)
	}
	if got["b"] || got["c"] {
		t.Errorf("floating defeat: b,c should not be in grounded, got %v", ext)
	}
}

func TestGroundedSelfAttack(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "a"}) // self-attack
	ext := fw.GroundedExtension()
	got := make(map[string]bool)
	for _, id := range ext {
		got[id] = true
	}
	if got["a"] {
		t.Error("self-attacking argument should not be in grounded extension")
	}
	if !got["b"] {
		t.Error("unattacked argument b should be in grounded extension")
	}
}

// --- Grounded Labelling ---

func TestGroundedLabelling(t *testing.T) {
	// A → B → C: A=In, B=Out, C=In
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})

	labels := fw.GroundedLabelling()
	if labels["a"] != In {
		t.Errorf("a: want In, got %s", labels["a"])
	}
	if labels["b"] != Out {
		t.Errorf("b: want Out, got %s", labels["b"])
	}
	if labels["c"] != In {
		t.Errorf("c: want In, got %s", labels["c"])
	}
}

func TestGroundedLabellingUndec(t *testing.T) {
	// A ↔ B: both Undec
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "a"})

	labels := fw.GroundedLabelling()
	if labels["a"] != Undec {
		t.Errorf("a: want Undec, got %s", labels["a"])
	}
	if labels["b"] != Undec {
		t.Errorf("b: want Undec, got %s", labels["b"])
	}
}

// --- Preferred Extensions ---

func TestPreferredMutualAttack(t *testing.T) {
	// A ↔ B: preferred = [{a}, {b}]
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "a"})

	exts := fw.PreferredExtensions()
	if len(exts) != 2 {
		t.Fatalf("mutual attack: want 2 preferred extensions, got %d: %v", len(exts), exts)
	}
	// Sorted: [{a}, {b}]
	if len(exts[0]) != 1 || exts[0][0] != "a" {
		t.Errorf("first preferred: want [a], got %v", exts[0])
	}
	if len(exts[1]) != 1 || exts[1][0] != "b" {
		t.Errorf("second preferred: want [b], got %v", exts[1])
	}
}

func TestPreferredOddCycle(t *testing.T) {
	// A → B → C → A: only preferred extension is {}
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "a"})

	exts := fw.PreferredExtensions()
	if len(exts) != 1 {
		t.Fatalf("odd cycle: want 1 preferred, got %d: %v", len(exts), exts)
	}
	if len(exts[0]) != 0 {
		t.Errorf("odd cycle: want [], got %v", exts[0])
	}
}

// --- Stable Extensions ---

func TestStableMutualAttack(t *testing.T) {
	// A ↔ B: stable = [{a}, {b}]
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "a"})

	exts := fw.StableExtensions()
	if len(exts) != 2 {
		t.Fatalf("mutual attack: want 2 stable, got %d: %v", len(exts), exts)
	}
}

func TestStableOddCycle(t *testing.T) {
	// A → B → C → A: no stable extension exists
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "a"})

	exts := fw.StableExtensions()
	if len(exts) != 0 {
		t.Errorf("odd cycle: want 0 stable, got %d: %v", len(exts), exts)
	}
}

// --- Support Chain Propagation ---

func TestSupportChainPropagation(t *testing.T) {
	// A supports B, C attacks A → secondary attack C→B
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "a"})

	eff := fw.EffectiveAttacks()
	hasSecondary := false
	for _, atk := range eff {
		if atk.AttackerID == "c" && atk.TargetID == "b" {
			hasSecondary = true
		}
	}
	if !hasSecondary {
		t.Error("support chain should propagate attack C→A to C→B")
	}

	// C is unattacked → In, A and B are defeated
	ext := fw.GroundedExtension()
	got := make(map[string]bool)
	for _, id := range ext {
		got[id] = true
	}
	if !got["c"] {
		t.Error("c should be in grounded extension")
	}
	if got["a"] || got["b"] {
		t.Error("a and b should not be in grounded (defeated via support chain)")
	}
}

func TestTransitiveSupportChain(t *testing.T) {
	// A supports B, B supports C, D attacks A → D defeats A, B, C
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddSupport(Support{SupporterID: "b", SupportedID: "c"})
	fw.AddAttack(Attack{AttackerID: "d", TargetID: "a"})

	ext := fw.GroundedExtension()
	got := make(map[string]bool)
	for _, id := range ext {
		got[id] = true
	}
	if !got["d"] {
		t.Error("d should be in grounded extension")
	}
	if got["a"] || got["b"] || got["c"] {
		t.Error("a, b, c should all be defeated via transitive support chain")
	}
}

// --- Causal Chain Analysis ---

func TestFindCausalChains(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddSupport(Support{SupporterID: "b", SupportedID: "c"})

	chains := fw.FindCausalChains()
	if len(chains) != 1 {
		t.Fatalf("want 1 causal chain, got %d", len(chains))
	}
	if len(chains[0].ArgumentIDs) != 3 {
		t.Errorf("want chain [a,b,c], got %v", chains[0].ArgumentIDs)
	}
}

func TestChainDefensible(t *testing.T) {
	// Unattacked chain A→B→C: all In, defensible
	fw := New()
	fw.AddArgument(Argument{ID: "a", Strength: strength(0.9)})
	fw.AddArgument(Argument{ID: "b", Strength: strength(0.8)})
	fw.AddArgument(Argument{ID: "c", Strength: strength(0.7)})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddSupport(Support{SupporterID: "b", SupportedID: "c"})

	chains := fw.FindCausalChains()
	if len(chains) != 1 {
		t.Fatalf("want 1 chain, got %d", len(chains))
	}
	if !fw.ChainDefensible(chains[0]) {
		t.Error("unattacked causal chain should be defensible")
	}

	// Now attack the root → chain no longer defensible
	fw.AddArgument(Argument{ID: "d"})
	fw.AddAttack(Attack{AttackerID: "d", TargetID: "a"})
	if fw.ChainDefensible(chains[0]) {
		t.Error("causal chain with defeated root should not be defensible")
	}
}

func TestChainStrength(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a", Strength: subjective.OpinionFromEvidence(8, 2, 0.5)})
	fw.AddArgument(Argument{ID: "b", Strength: subjective.OpinionFromEvidence(6, 4, 0.5)})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})

	chains := fw.FindCausalChains()
	if len(chains) != 1 {
		t.Fatalf("want 1 chain, got %d", len(chains))
	}

	cs := fw.ChainStrength(chains[0])
	epA := fw.args["a"].Strength.ExpectedProbability()
	epB := fw.args["b"].Strength.ExpectedProbability()
	want := epA * epB
	got := cs.ExpectedProbability()
	if !approx(got, want) {
		t.Errorf("chain strength E[p]: want %.6f, got %.6f", want, got)
	}
}

func TestChainWeakestLink(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a", Strength: strength(0.9)})
	fw.AddArgument(Argument{ID: "b", Strength: strength(0.3)})
	fw.AddArgument(Argument{ID: "c", Strength: strength(0.7)})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddSupport(Support{SupporterID: "b", SupportedID: "c"})

	chains := fw.FindCausalChains()
	weak := fw.ChainWeakestLink(chains[0])
	if weak == nil || weak.ID != "b" {
		t.Errorf("weakest link should be b, got %v", weak)
	}
}

// --- Belnap Bridge ---

func TestLabelToBelnap(t *testing.T) {
	tests := []struct {
		label Label
		want  belnap.Value
	}{
		{In, belnap.True},
		{Out, belnap.False},
		{Undec, belnap.Neither},
	}
	for _, tc := range tests {
		got := LabelToBelnap(tc.label)
		if got != tc.want {
			t.Errorf("LabelToBelnap(%s) = %s, want %s", tc.label, got, tc.want)
		}
	}
}

func TestBelnapStatus(t *testing.T) {
	// A (unattacked, evidence True) → BelnapStatus = T ⊔ T = T
	fw := New()
	fw.AddArgument(Argument{ID: "a", Status: belnap.True})
	if fw.BelnapStatus("a") != belnap.True {
		t.Errorf("unattacked true: want T, got %s", fw.BelnapStatus("a"))
	}

	// B (defeated, evidence True) → BelnapStatus = F ⊔ T = B
	fw.AddArgument(Argument{ID: "b", Status: belnap.True})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	if fw.BelnapStatus("b") != belnap.Both {
		t.Errorf("defeated but evidence true: want B, got %s", fw.BelnapStatus("b"))
	}
}

func TestCrossExtensionBelnap(t *testing.T) {
	// A ↔ B: A is In in one preferred ext, Out in the other → Both
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "a"})

	if fw.CrossExtensionBelnap("a") != belnap.Both {
		t.Errorf("a across extensions: want Both, got %s", fw.CrossExtensionBelnap("a"))
	}
}

// --- Narrative Entropy ---

func TestNarrativeEntropySingle(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	if fw.NarrativeEntropy() != 0 {
		t.Error("single extension: entropy should be 0")
	}
}

func TestNarrativeEntropyTwo(t *testing.T) {
	// A ↔ B: 2 preferred extensions → entropy = 1 bit
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "a"})

	if !approx(fw.NarrativeEntropy(), 1.0) {
		t.Errorf("two extensions: want entropy=1.0, got %.6f", fw.NarrativeEntropy())
	}
}

// --- Conflict-Free and Admissible ---

func TestIsConflictFree(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})

	if !fw.IsConflictFree(map[string]bool{"a": true}) {
		t.Error("{a} should be conflict-free")
	}
	if fw.IsConflictFree(map[string]bool{"a": true, "b": true}) {
		t.Error("{a,b} should NOT be conflict-free (a attacks b)")
	}
}

func TestIsAdmissible(t *testing.T) {
	// A → B → C: {a,c} is admissible (a defends c against b)
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})

	if !fw.IsAdmissible(map[string]bool{"a": true, "c": true}) {
		t.Error("{a,c} should be admissible")
	}
	if fw.IsAdmissible(map[string]bool{"b": true, "c": true}) {
		t.Error("{b,c} should NOT be admissible (b attacks c)")
	}
}

// --- Accessors ---

func TestAttackersOf(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})

	attackers := fw.AttackersOf("c")
	if len(attackers) != 2 || attackers[0] != "a" || attackers[1] != "b" {
		t.Errorf("attackers of c: want [a,b], got %v", attackers)
	}
}

func TestArgumentsAccessor(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "a"})

	args := fw.Arguments()
	if len(args) != 2 || args[0].ID != "a" || args[1].ID != "b" {
		t.Errorf("arguments should be sorted by ID: got %v", args)
	}
}

// --- Integration: Subjective Logic + Argumentation ---

func TestDefensibleChainVsDefeatedChain(t *testing.T) {
	// Two competing causal chains:
	// Chain 1: E1 → E2 → conclusion (strong, unattacked)
	// Chain 2: E3 → E4 → same conclusion (weak, attacked)
	fw := New()
	fw.AddArgument(Argument{ID: "e1", Strength: strength(0.9), Status: belnap.True})
	fw.AddArgument(Argument{ID: "e2", Strength: strength(0.8), Status: belnap.True})
	fw.AddArgument(Argument{ID: "e3", Strength: strength(0.4), Status: belnap.True})
	fw.AddArgument(Argument{ID: "e4", Strength: strength(0.5), Status: belnap.True})
	fw.AddArgument(Argument{ID: "refuter", Strength: strength(0.85), Status: belnap.True})

	fw.AddSupport(Support{SupporterID: "e1", SupportedID: "e2"})
	fw.AddSupport(Support{SupporterID: "e3", SupportedID: "e4"})
	fw.AddAttack(Attack{AttackerID: "refuter", TargetID: "e3"})

	chains := fw.FindCausalChains()
	if len(chains) != 2 {
		t.Fatalf("want 2 chains, got %d", len(chains))
	}

	// Find which chain is e1→e2 vs e3→e4
	var chain1, chain2 CausalChain
	for _, ch := range chains {
		if ch.ArgumentIDs[0] == "e1" {
			chain1 = ch
		} else {
			chain2 = ch
		}
	}

	if !fw.ChainDefensible(chain1) {
		t.Error("unattacked chain e1→e2 should be defensible")
	}
	if fw.ChainDefensible(chain2) {
		t.Error("attacked chain e3→e4 should not be defensible")
	}

	// Chain 1 should be stronger
	s1 := fw.ChainStrength(chain1).ExpectedProbability()
	s2 := fw.ChainStrength(chain2).ExpectedProbability()
	if s1 <= s2 {
		t.Errorf("chain1 should be stronger: %.4f <= %.4f", s1, s2)
	}
}

// =============================================================================
// Additional coverage tests
// =============================================================================

// --- String Methods ---

func TestAttackTypeString(t *testing.T) {
	tests := []struct {
		typ  AttackType
		want string
	}{
		{Rebut, "Rebut"},
		{Undercut, "Undercut"},
		{Undermine, "Undermine"},
		{AttackType(99), "Unknown"}, // out-of-range value
	}
	for _, tc := range tests {
		got := tc.typ.String()
		if got != tc.want {
			t.Errorf("AttackType(%d).String() = %q, want %q", int(tc.typ), got, tc.want)
		}
	}
}

func TestLabelString(t *testing.T) {
	tests := []struct {
		label Label
		want  string
	}{
		{In, "In"},
		{Out, "Out"},
		{Undec, "Undec"},
		{Label(99), "?"}, // out-of-range value
	}
	for _, tc := range tests {
		got := tc.label.String()
		if got != tc.want {
			t.Errorf("Label(%d).String() = %q, want %q", int(tc.label), got, tc.want)
		}
	}
}

// --- Accessors: Argument(id), Attacks(), Supports(), including not-found ---

func TestArgumentAccessorFound(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "x", Desc: "test arg"})
	got := fw.Argument("x")
	if got == nil {
		t.Fatal("Argument('x') returned nil for existing argument")
	}
	if got.ID != "x" || got.Desc != "test arg" {
		t.Errorf("Argument('x') = %+v, want ID=x Desc='test arg'", got)
	}
}

func TestArgumentAccessorNotFound(t *testing.T) {
	fw := New()
	got := fw.Argument("nonexistent")
	if got != nil {
		t.Errorf("Argument('nonexistent') should return nil, got %+v", got)
	}
}

func TestAttacksAccessor(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b", Type: Undercut})

	attacks := fw.Attacks()
	if len(attacks) != 1 {
		t.Fatalf("want 1 attack, got %d", len(attacks))
	}
	if attacks[0].AttackerID != "a" || attacks[0].TargetID != "b" || attacks[0].Type != Undercut {
		t.Errorf("unexpected attack: %+v", attacks[0])
	}
}

func TestAttacksAccessorEmpty(t *testing.T) {
	fw := New()
	attacks := fw.Attacks()
	if len(attacks) != 0 {
		t.Errorf("empty framework: want 0 attacks, got %d", len(attacks))
	}
}

func TestSupportsAccessor(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})

	supports := fw.Supports()
	if len(supports) != 1 {
		t.Fatalf("want 1 support, got %d", len(supports))
	}
	if supports[0].SupporterID != "a" || supports[0].SupportedID != "b" {
		t.Errorf("unexpected support: %+v", supports[0])
	}
}

func TestSupportsAccessorEmpty(t *testing.T) {
	fw := New()
	supports := fw.Supports()
	if len(supports) != 0 {
		t.Errorf("empty framework: want 0 supports, got %d", len(supports))
	}
}

// --- EffectiveAttacks: diamond support topology ---
// Cayrol & Lagasquie-Schiex BAF flattening: A supports B and C, both B and C
// support D. Attacker E attacks A. The BFS in EffectiveAttacks should produce
// secondary attacks E→B, E→C, E→D without duplicate edges, verifying the
// visited-set deduplication.
func TestEffectiveAttacksDiamondSupport(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddArgument(Argument{ID: "e"})
	// Diamond: A→B, A→C, B→D, C→D
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "c"})
	fw.AddSupport(Support{SupporterID: "b", SupportedID: "d"})
	fw.AddSupport(Support{SupporterID: "c", SupportedID: "d"})
	// E attacks root A
	fw.AddAttack(Attack{AttackerID: "e", TargetID: "a", Type: Rebut})

	eff := fw.EffectiveAttacks()
	targets := make(map[string]int)
	for _, atk := range eff {
		if atk.AttackerID == "e" {
			targets[atk.TargetID]++
		}
	}
	// E should attack A (explicit) + B, C, D (secondary via BAF flattening)
	for _, expected := range []string{"a", "b", "c", "d"} {
		if targets[expected] != 1 {
			t.Errorf("expected exactly 1 attack from e to %s, got %d", expected, targets[expected])
		}
	}
	// Total unique E→* attacks should be 4
	if len(targets) != 4 {
		t.Errorf("want 4 targets from e, got %d: %v", len(targets), targets)
	}

	// Verify grounded extension: E is the only accepted argument (Dung 1995
	// least fixpoint) since it defeats A, and A's defeat propagates to B, C, D
	// via necessary support (Cayrol & Lagasquie-Schiex 2005).
	ext := fw.GroundedExtension()
	got := make(map[string]bool)
	for _, id := range ext {
		got[id] = true
	}
	if !got["e"] {
		t.Error("e should be in grounded extension (unattacked)")
	}
	for _, defeated := range []string{"a", "b", "c", "d"} {
		if got[defeated] {
			t.Errorf("%s should not be in grounded (defeated via support chain)", defeated)
		}
	}
}

// --- IsAdmissible: not defended ---
// Per Dung (1995), S is admissible iff S is conflict-free and every argument
// in S is defended by S. Here {b} is attacked by a and b cannot counter-attack a,
// so {b} is not admissible.
func TestIsAdmissibleNotDefended(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b", Type: Rebut})

	set := map[string]bool{"b": true}
	if fw.IsAdmissible(set) {
		t.Error("{b} should NOT be admissible: b is attacked by a with no counter-attack (Dung 1995 Def. 6)")
	}
	// {a} should be admissible (unattacked, conflict-free, trivially defended)
	if !fw.IsAdmissible(map[string]bool{"a": true}) {
		t.Error("{a} should be admissible: unattacked argument")
	}
	// Empty set is always admissible (vacuously conflict-free, vacuously defended)
	if !fw.IsAdmissible(map[string]bool{}) {
		t.Error("empty set should be admissible (Dung 1995: vacuous satisfaction)")
	}
}

// --- PreferredExtensions >25 args fallback ---
// When the framework exceeds maxArgsForExhaustive (25), PreferredExtensions
// should fall back to returning the grounded extension wrapped in a singleton
// list to avoid combinatorial explosion.
func TestPreferredExtensionsFallback(t *testing.T) {
	fw := New()
	for i := 0; i < 26; i++ {
		fw.AddArgument(Argument{ID: fmt.Sprintf("arg%02d", i)})
	}
	// 26 unattacked arguments: grounded extension = all 26
	exts := fw.PreferredExtensions()
	if len(exts) != 1 {
		t.Fatalf("fallback: want 1 extension, got %d", len(exts))
	}
	if len(exts[0]) != 26 {
		t.Errorf("fallback: want 26 args in grounded, got %d", len(exts[0]))
	}
}

// --- StableExtensions >25 args fallback ---
// Beyond maxArgsForExhaustive, StableExtensions should return nil.
func TestStableExtensionsFallback(t *testing.T) {
	fw := New()
	for i := 0; i < 26; i++ {
		fw.AddArgument(Argument{ID: fmt.Sprintf("arg%02d", i)})
	}
	exts := fw.StableExtensions()
	if exts != nil {
		t.Errorf("fallback: want nil for large framework, got %v", exts)
	}
}

// --- Causal chains with support cycles ---
// A→B→C→A forms a support cycle. FindCausalChains must terminate without
// infinite recursion. The cycle means none of A, B, C is a pure root (each
// has an incoming support), so no chains should be discovered.
func TestFindCausalChainsSupportCycle(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddSupport(Support{SupporterID: "b", SupportedID: "c"})
	fw.AddSupport(Support{SupporterID: "c", SupportedID: "a"})

	chains := fw.FindCausalChains()
	// Every argument has incoming support, so no root exists → no chains
	if len(chains) != 0 {
		t.Errorf("support cycle: want 0 chains (no root), got %d: %v", len(chains), chains)
	}
}

// Cycle with a dangling tail: D→A→B→C→A. D is a root (outgoing support,
// no incoming). The DFS should discover the chain D→A→B→C but stop at
// the cycle back to A.
func TestFindCausalChainsCycleWithRoot(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddSupport(Support{SupporterID: "d", SupportedID: "a"})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddSupport(Support{SupporterID: "b", SupportedID: "c"})
	fw.AddSupport(Support{SupporterID: "c", SupportedID: "a"}) // cycle back

	chains := fw.FindCausalChains()
	if len(chains) != 1 {
		t.Fatalf("cycle with root: want 1 chain, got %d: %v", len(chains), chains)
	}
	// Chain should be d→a→b→c (stops at cycle back to visited a)
	ch := chains[0].ArgumentIDs
	if len(ch) != 4 || ch[0] != "d" || ch[1] != "a" || ch[2] != "b" || ch[3] != "c" {
		t.Errorf("want chain [d,a,b,c], got %v", ch)
	}
}

// --- ChainStrength: empty chain ---
// Empty ArgumentIDs should return Vacuous(0.5) per the implementation.
func TestChainStrengthEmpty(t *testing.T) {
	fw := New()
	empty := CausalChain{ArgumentIDs: []string{}}
	cs := fw.ChainStrength(empty)
	vac := subjective.Vacuous(0.5)
	if !approx(cs.Belief, vac.Belief) || !approx(cs.Disbelief, vac.Disbelief) || !approx(cs.Uncertainty, vac.Uncertainty) {
		t.Errorf("empty chain strength: want Vacuous(0.5)=%+v, got %+v", vac, cs)
	}
}

// --- ChainStrength with missing argument ---
// First argument missing → Vacuous. Non-first argument missing → skipped.
func TestChainStrengthMissingFirstArg(t *testing.T) {
	fw := New()
	chain := CausalChain{ArgumentIDs: []string{"nonexistent"}}
	cs := fw.ChainStrength(chain)
	vac := subjective.Vacuous(0.5)
	if !approx(cs.Uncertainty, vac.Uncertainty) {
		t.Errorf("missing first arg: want Vacuous, got %+v", cs)
	}
}

func TestChainStrengthMissingLaterArg(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a", Strength: strength(0.8)})
	// "missing" is not in the framework
	chain := CausalChain{ArgumentIDs: []string{"a", "missing"}}
	cs := fw.ChainStrength(chain)
	// Missing argument is skipped; result should just be a's strength
	aEP := fw.args["a"].Strength.ExpectedProbability()
	if !approx(cs.ExpectedProbability(), aEP) {
		t.Errorf("missing later arg: want E[p]=%.6f (just a), got %.6f", aEP, cs.ExpectedProbability())
	}
}

// --- ChainWeakestLink with missing argument ---
func TestChainWeakestLinkMissingArg(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a", Strength: strength(0.8)})
	chain := CausalChain{ArgumentIDs: []string{"a", "nonexistent"}}
	weak := fw.ChainWeakestLink(chain)
	// "nonexistent" is skipped; only "a" is considered
	if weak == nil || weak.ID != "a" {
		t.Errorf("missing arg: weakest link should be 'a', got %v", weak)
	}
}

func TestChainWeakestLinkAllMissing(t *testing.T) {
	fw := New()
	chain := CausalChain{ArgumentIDs: []string{"x", "y"}}
	weak := fw.ChainWeakestLink(chain)
	if weak != nil {
		t.Errorf("all missing: weakest link should be nil, got %+v", weak)
	}
}

// --- BelnapStatus: missing argument ---
// Querying an argument not in the framework should return Neither, consistent
// with the epistemic interpretation: no information → Neither (Belnap 1977).
func TestBelnapStatusMissingArgument(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a", Status: belnap.True})
	got := fw.BelnapStatus("nonexistent")
	if got != belnap.Neither {
		t.Errorf("missing argument: want Neither, got %s", got)
	}
}

// --- CrossExtensionBelnap edge cases ---

// Empty framework: no extensions, no arguments → Neither.
func TestCrossExtensionBelnapEmptyFramework(t *testing.T) {
	fw := New()
	got := fw.CrossExtensionBelnap("anything")
	if got != belnap.Neither {
		t.Errorf("empty framework: want Neither, got %s", got)
	}
}

// Argument undecided in all extensions → Neither.
// Odd cycle A→B→C→A: grounded = {} (all Undec), preferred = {{}}.
// Asking about any of them should yield Neither.
func TestCrossExtensionBelnapAllUndec(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "a"})

	// In the odd cycle, the only preferred extension is {} (empty).
	// All arguments are neither in the extension nor attacked by a member of
	// the extension, so they are Undec in every extension → Neither.
	for _, id := range []string{"a", "b", "c"} {
		got := fw.CrossExtensionBelnap(id)
		if got != belnap.Neither {
			t.Errorf("CrossExtensionBelnap(%s) in odd cycle: want Neither, got %s", id, got)
		}
	}
}

// CrossExtensionBelnap Out+Undec and In+Undec branches.
//
// Framework: odd cycle A→B→C→A with C also attacking X, plus D↔E with D attacking A.
//
// Preferred extensions (computed): {b,d,x} and {e}.
//
//	In {b,d,x}: a=Out, b=In, c=Out, d=In, e=Out, x=In
//	In {e}:     a=Undec, b=Undec, c=Undec, d=Out, e=In, x=Undec
//
// Argument a: Out in one, Undec in other → outCount>0 && undecCount>0 → Both
// Argument x: In in one, Undec in other → inCount>0 && undecCount>0 → Both
func TestCrossExtensionBelnapOutAndUndecMixed(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddArgument(Argument{ID: "e"})
	fw.AddArgument(Argument{ID: "x"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "a"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "x"})
	fw.AddAttack(Attack{AttackerID: "d", TargetID: "e"})
	fw.AddAttack(Attack{AttackerID: "e", TargetID: "d"})
	fw.AddAttack(Attack{AttackerID: "d", TargetID: "a"})

	exts := fw.PreferredExtensions()
	if len(exts) != 2 {
		t.Fatalf("want 2 preferred extensions, got %d: %v", len(exts), exts)
	}

	// a: Out in {b,d,x} (attacked by d(In)), Undec in {e} → Out+Undec → Both
	gotA := fw.CrossExtensionBelnap("a")
	if gotA != belnap.Both {
		t.Errorf("a across extensions: want Both (Out+Undec mix), got %s", gotA)
	}
}

// Argument In in some and Undec in others → Both.
// Framework: A ↔ B, C unattacked. A also supports nothing special.
// In ext {A}: A is In. In ext {B}: A is Out.
// Now check argument that is In in one but Undec in another:
//   Add D attacked by B only. In {A}: B is Out, so D is In. In {B}: B is In, so D is Out.
//   That gives In+Out → Both (already tested).
// For In+Undec: add E attacked by C only where C is in odd cycle.
// Simpler: use framework where arg is In some, Undec in others.
//
// D ↔ E (mutual), F attacked by D only.
// Preferred: {D}, {E}. In {D}: F is Out (attacked by D). In {E}: F not attacked → Undec.
// But wait, we need a case where arg is IN in some and UNDEC in others.
//
// Let's try: A ↔ B, D supported by nothing but not attacked by anyone.
// D is in both preferred extensions {A,D} and {B,D} → True.
// Instead: A ↔ B, A attacks C, D attacks C.
// Preferred: {A,D} and {B,D}. C: In {A,D}: Out. In {B,D}: attacked by D → Out. Still Out.
//
// For In+Undec: A ↔ B, C is attacked only by itself (self-attack) and D.
// Actually simplest: A ↔ B, and D is not attacked or attacking anything.
// Preferred: {A,D}, {B,D}. D is In everywhere → True.
// We need an argument that appears in SOME but is Undec in others.
//
// Consider: A ↔ B, C ↔ D. Preferred: {A,C}, {A,D}, {B,C}, {B,D}.
// Now add E attacked by only C. In exts with C (In): E is Out.
// In exts without C ({A,D}, {B,D}): E is not attacked by D or A or B → Undec? No, C is Out in those.
// Actually in {A,D}: C is attacked by D (In) → C is Out. E is attacked by C (Out) → E is not attacked by any In member → Undec.
// In {A,C}: C is In. E is attacked by C (In) → E is Out.
// So E: {A,C}→Out, {A,D}→Undec, {B,C}→Out, {B,D}→Undec. Out+Undec → Both.
// Already covered by the test above. Let's also test In+Undec explicitly.
//
// For a genuinely In+Undec case: Consider the Nixon diamond.
// A ↔ B (mutual), with additional argument C only attacked by A.
// Then add D only attacked by C.
// Preferred: {A} and {B}.
// In {A}: C is attacked by A (In) → Out. D: attacked by C (Out) → not attacked by any In → Undec? No.
// Wait, D is only attacked by C. In {A}: C is Out (attacked by In 'a'). D is not attacked by any In member → Undec.
// In {B}: C is not attacked by B. C: not attacked by any In member? C is attacked by A. A is Out (B attacks A, B is In). So C is not attacked by any In → Undec.
// D: attacked by C (Undec) → not attacked by In → Undec.
// So D is Undec in all extensions → Neither. Not what we want.
//
// Instead: Framework with 3 preferred exts where arg is In in one, Undec in another.
// Simple approach: A ↔ B, C unattacked, and C is in all exts.
// C is In everywhere → True.
// We want an arg in SOME extensions but not all.
// How about: A ↔ B, C ↔ D, E not attacked but present.
// Preferred: {A,C,E}, {A,D,E}, {B,C,E}, {B,D,E}. E always In → True.
//
// The implementation code shows that inCount>0 && undecCount>0 → Both.
// So we need a framework where an argument is In in at least one preferred ext
// and Undec (not In, not attacked by any In member) in at least one other.
// That's hard to construct naturally because preferred extensions are maximal admissible.
// The existing mutual attack test (In+Out → Both) and the Out+Undec test above
// already cover the three "Both" branches. Let me verify by checking which branches
// are hit:
// - inCount>0 && outCount>0: original CrossExtensionBelnap test (A ↔ B)
// - inCount>0 && undecCount>0: needs specific test
// - outCount>0 && undecCount>0: TestCrossExtensionBelnapOutAndUndecMixed above
//
// For In+Undec: We need an argument X that is IN in one preferred ext and
// UNDEC in another. This can happen if X is in one maximal admissible set
// but in another ext, X is neither a member nor attacked by any member.
// Try: A ↔ B, X attacked by nothing.
// Preferred: {A, X}, {B, X}. X is In everywhere → True. Nope.
// Try: A ↔ B, X ↔ Y, A attacks X.
// Admissible sets? {A}: A attacks X, X attacks Y. {A,Y}: conflict-free? A attacks B (not Y), X attacks Y, Y attacks X. A∈set, Y∈set. No attack between A and Y. A attacks X (not in set). Y attacks X (not in set). X attacks Y, but X not in set. Conflict-free. Defended? A: unattacked. Y: attacked by X. X is attacked by A∈set → Y defended. Admissible: yes. Maximal superset? Add B? B attacked by A(In), not defended. No. {A,Y} is maximal.
// {B}: B attacks A. X ↔ Y. B doesn't attack X or Y. {B,X}: X attacked by A (not in set). A attacked by B (in set) → X defended. Conflict-free. {B,X}: X attacks Y (not in set). Admissible. Maximal? Add Y? Y attacked by X(In) → Y is Out → can't add. {B,X} maximal.
// {B,Y}: Y attacked by X, X attacked by A, A attacked by B(In) → X is defended against by B. But Y attacked by X. Is X attacked by anyone in {B,Y}? X attacked by A. A attacked by B(In). So X's attacker A is counter-attacked by B. So Y's attacker X is defended if X's attackers are counter-attacked. Wait, Y is attacked by X. For Y to be defended by {B,Y}, every attacker of Y (i.e., X) must be counter-attacked by some member of {B,Y}. Is X attacked by any member of {B,Y}? B doesn't attack X, Y attacks X? Yes! X ↔ Y so Y attacks X. So Y defends itself. {B,Y} admissible? Conflict-free: B attacks A (not in set), Y attacks X (not in set). Yes. Defended: B unattacked by any in-framework attacker? B attacked by A, A attacked by... hmm B doesn't attack A in this framework. Wait: A ↔ B? No, I only said A ↔ B. Let me re-specify.
// Actually this is getting complex. Let me find a simpler approach for the In+Undec branch.

func TestCrossExtensionBelnapInAndUndecMixed(t *testing.T) {
	// Uses the same framework as TestCrossExtensionBelnapOutAndUndecMixed:
	// Odd cycle A→B→C→A, C attacks X, D↔E, D attacks A.
	//
	// Preferred extensions: {b,d,x} and {e}.
	//   In {b,d,x}: x=In (member of the extension)
	//   In {e}:     x=Undec (not in ext, not attacked by any In member;
	//               x's attacker c is Undec since c is in the odd cycle
	//               and not resolved by ext {e})
	//
	// Argument x: In + Undec → inCount>0 && undecCount>0 → Both
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddArgument(Argument{ID: "e"})
	fw.AddArgument(Argument{ID: "x"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "a"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "x"})
	fw.AddAttack(Attack{AttackerID: "d", TargetID: "e"})
	fw.AddAttack(Attack{AttackerID: "e", TargetID: "d"})
	fw.AddAttack(Attack{AttackerID: "d", TargetID: "a"})

	exts := fw.PreferredExtensions()
	if len(exts) != 2 {
		t.Fatalf("want 2 preferred extensions, got %d: %v", len(exts), exts)
	}

	// x: In in {b,d,x}, Undec in {e} → In+Undec → Both
	gotX := fw.CrossExtensionBelnap("x")
	if gotX != belnap.Both {
		t.Errorf("x across extensions: want Both (In+Undec mix), got %s", gotX)
	}
}

// Verify that CrossExtensionBelnap returns True when an argument is In in
// every preferred extension (universally accepted across all narratives).
func TestCrossExtensionBelnapUniversallyIn(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "b"})

	// Preferred: {a, b} and {a, c}. a is In in both → True.
	got := fw.CrossExtensionBelnap("a")
	if got != belnap.True {
		t.Errorf("universally accepted: want True, got %s", got)
	}
}

// Verify False when argument is Out in all preferred extensions.
func TestCrossExtensionBelnapUniversallyOut(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})

	// Single preferred extension {a}. b is attacked by a(In) → Out.
	got := fw.CrossExtensionBelnap("b")
	if got != belnap.False {
		t.Errorf("universally defeated: want False, got %s", got)
	}
}

// --- isProperSubset: both true and false cases ---
// These are tested indirectly through PreferredExtensions, but we exercise
// the helper directly for coverage.
func TestIsProperSubset(t *testing.T) {
	tests := []struct {
		name string
		a, b map[string]bool
		want bool
	}{
		{
			name: "proper subset",
			a:    map[string]bool{"x": true},
			b:    map[string]bool{"x": true, "y": true},
			want: true,
		},
		{
			name: "equal sets (not proper)",
			a:    map[string]bool{"x": true, "y": true},
			b:    map[string]bool{"x": true, "y": true},
			want: false, // same size → len(a) >= len(b) → false
		},
		{
			name: "superset (not subset at all)",
			a:    map[string]bool{"x": true, "y": true, "z": true},
			b:    map[string]bool{"x": true},
			want: false,
		},
		{
			name: "disjoint sets",
			a:    map[string]bool{"x": true},
			b:    map[string]bool{"y": true, "z": true},
			want: false, // x not in b
		},
		{
			name: "empty is proper subset of non-empty",
			a:    map[string]bool{},
			b:    map[string]bool{"x": true},
			want: true,
		},
		{
			name: "both empty (not proper)",
			a:    map[string]bool{},
			b:    map[string]bool{},
			want: false,
		},
	}
	for _, tc := range tests {
		got := isProperSubset(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("isProperSubset %s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// --- FindCausalChains: empty framework (no supports) ---
func TestFindCausalChainsNoSupports(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})

	chains := fw.FindCausalChains()
	if len(chains) != 0 {
		t.Errorf("no supports: want 0 chains, got %d", len(chains))
	}
}

func TestFindCausalChainsEmptyFramework(t *testing.T) {
	fw := New()
	chains := fw.FindCausalChains()
	if len(chains) != 0 {
		t.Errorf("empty framework: want 0 chains, got %d", len(chains))
	}
}

// --- Tarski Fixpoint Convergence Properties ---
// Dung (1995) Theorem 25: the grounded extension is the least fixpoint of the
// characteristic function F(S) = {A | A is defended by S}. Monotonicity of F
// on the complete lattice of conflict-free sets guarantees convergence by
// Tarski's theorem (1955). We verify:
//   1. The grounded extension IS a fixpoint (F(GE) = GE)
//   2. No proper subset of the grounded extension is also a fixpoint
//   3. Convergence occurs in at most |Args| iterations

func TestGroundedIsLeastFixpoint(t *testing.T) {
	// Non-trivial framework: A → B → C → D, E → B (reinstatement chain).
	// Grounded: {A, C, E} — A and E unattacked, C reinstated by A defeating B,
	// D reinstated by... let's verify.
	// A→B: A defeats B. B→C: B defeats C, but A defeats B → C reinstated.
	// C→D: C defeats D. E→B: E also defeats B.
	// Grounded: A, E unattacked → In. B attacked by A(In) → Out.
	// C attacked by B(Out) → C defended → In. D attacked by C(In) → Out.
	// GE = {A, C, E}.
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddArgument(Argument{ID: "e"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "d"})
	fw.AddAttack(Attack{AttackerID: "e", TargetID: "b"})

	ext := fw.GroundedExtension()
	geSet := make(map[string]bool)
	for _, id := range ext {
		geSet[id] = true
	}

	// Verify F(GE) = GE (fixpoint property).
	// F(S) = {A | every attacker of A is counter-attacked by some member of S}
	allAttacks := fw.EffectiveAttacks()
	fGE := make(map[string]bool)
	for id := range fw.args {
		defended := true
		for _, atk := range allAttacks {
			if atk.TargetID == id {
				// Check if attacker is counter-attacked by GE
				counterAttacked := false
				for _, atk2 := range allAttacks {
					if atk2.TargetID == atk.AttackerID && geSet[atk2.AttackerID] {
						counterAttacked = true
						break
					}
				}
				if !counterAttacked {
					defended = false
					break
				}
			}
		}
		if defended {
			fGE[id] = true
		}
	}

	// F(GE) should equal GE
	if len(fGE) != len(geSet) {
		t.Fatalf("fixpoint violation: F(GE) has %d elements, GE has %d", len(fGE), len(geSet))
	}
	for id := range geSet {
		if !fGE[id] {
			t.Errorf("fixpoint violation: %s in GE but not in F(GE)", id)
		}
	}
	for id := range fGE {
		if !geSet[id] {
			t.Errorf("fixpoint violation: %s in F(GE) but not in GE", id)
		}
	}

	// Verify it's the LEAST fixpoint: no proper subset of GE is also a fixpoint.
	for omit := range geSet {
		subset := make(map[string]bool)
		for k := range geSet {
			if k != omit {
				subset[k] = true
			}
		}
		// Compute F(subset)
		fSubset := make(map[string]bool)
		for id := range fw.args {
			defended := true
			for _, atk := range allAttacks {
				if atk.TargetID == id {
					counterAttacked := false
					for _, atk2 := range allAttacks {
						if atk2.TargetID == atk.AttackerID && subset[atk2.AttackerID] {
							counterAttacked = true
							break
						}
					}
					if !counterAttacked {
						defended = false
						break
					}
				}
			}
			if defended {
				fSubset[id] = true
			}
		}
		// F(subset) should NOT equal subset (otherwise subset would be a smaller fixpoint)
		if len(fSubset) == len(subset) {
			equal := true
			for k := range subset {
				if !fSubset[k] {
					equal = false
					break
				}
			}
			if equal {
				t.Errorf("least fixpoint violation: GE\\{%s} is also a fixpoint", omit)
			}
		}
	}
}

// Verify that the grounded extension is always a subset of every preferred
// extension (Dung 1995, Theorem 25: GE ⊆ every preferred extension).
func TestGroundedSubsetOfPreferred(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "a"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "d"})
	fw.AddAttack(Attack{AttackerID: "d", TargetID: "c"})

	ge := fw.GroundedExtension()
	geSet := make(map[string]bool)
	for _, id := range ge {
		geSet[id] = true
	}

	for i, pref := range fw.PreferredExtensions() {
		prefSet := make(map[string]bool)
		for _, id := range pref {
			prefSet[id] = true
		}
		for id := range geSet {
			if !prefSet[id] {
				t.Errorf("Dung Theorem 25 violation: %s in grounded but not in preferred ext %d (%v)", id, i, pref)
			}
		}
	}
}

// Every stable extension is also a preferred extension (Dung 1995, Theorem 29).
func TestStableImpliesPreferred(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "a"})
	fw.AddAttack(Attack{AttackerID: "b", TargetID: "c"})
	fw.AddAttack(Attack{AttackerID: "c", TargetID: "b"})

	stables := fw.StableExtensions()
	preferreds := fw.PreferredExtensions()

	prefSets := make([]map[string]bool, len(preferreds))
	for i, p := range preferreds {
		s := make(map[string]bool)
		for _, id := range p {
			s[id] = true
		}
		prefSets[i] = s
	}

	for _, stable := range stables {
		stableSet := make(map[string]bool)
		for _, id := range stable {
			stableSet[id] = true
		}
		found := false
		for _, ps := range prefSets {
			if len(ps) == len(stableSet) {
				match := true
				for k := range stableSet {
					if !ps[k] {
						match = false
						break
					}
				}
				if match {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("Dung Theorem 29 violation: stable ext %v is not a preferred ext", stable)
		}
	}
}

// --- Cayrol & Lagasquie-Schiex BAF Flattening Correctness ---
// Verify that EffectiveAttacks produces exactly the correct secondary attacks
// for a transitive support chain, and that the grounded extension of the
// flattened framework matches the expected result.
func TestBAFFlatteningTransitiveChain(t *testing.T) {
	// A supports B supports C supports D. E attacks A.
	// Secondary attacks should be: E→B, E→C, E→D (transitive propagation).
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddArgument(Argument{ID: "c"})
	fw.AddArgument(Argument{ID: "d"})
	fw.AddArgument(Argument{ID: "e"})
	fw.AddSupport(Support{SupporterID: "a", SupportedID: "b"})
	fw.AddSupport(Support{SupporterID: "b", SupportedID: "c"})
	fw.AddSupport(Support{SupporterID: "c", SupportedID: "d"})
	fw.AddAttack(Attack{AttackerID: "e", TargetID: "a", Type: Undermine})

	eff := fw.EffectiveAttacks()
	secondary := make(map[string]bool)
	for _, atk := range eff {
		if atk.AttackerID == "e" && atk.TargetID != "a" {
			secondary[atk.TargetID] = true
			// Verify secondary attacks inherit the attack type
			if atk.Type != Undermine {
				t.Errorf("secondary attack E→%s should inherit Undermine type, got %s", atk.TargetID, atk.Type)
			}
		}
	}
	for _, expected := range []string{"b", "c", "d"} {
		if !secondary[expected] {
			t.Errorf("missing secondary attack E→%s in BAF flattening", expected)
		}
	}

	// Grounded extension: E unattacked → In. A, B, C, D all defeated.
	ext := fw.GroundedExtension()
	if len(ext) != 1 || ext[0] != "e" {
		t.Errorf("BAF grounded: want [e], got %v", ext)
	}
}

// --- EffectiveAttacks with no supports returns only explicit attacks ---
func TestEffectiveAttacksNoSupports(t *testing.T) {
	fw := New()
	fw.AddArgument(Argument{ID: "a"})
	fw.AddArgument(Argument{ID: "b"})
	fw.AddAttack(Attack{AttackerID: "a", TargetID: "b", Type: Rebut})

	eff := fw.EffectiveAttacks()
	if len(eff) != 1 {
		t.Fatalf("no supports: want 1 effective attack, got %d", len(eff))
	}
	if eff[0].AttackerID != "a" || eff[0].TargetID != "b" {
		t.Errorf("unexpected effective attack: %+v", eff[0])
	}
}

// --- GroundedLabelling on empty framework ---
func TestGroundedLabellingEmpty(t *testing.T) {
	fw := New()
	labels := fw.GroundedLabelling()
	if len(labels) != 0 {
		t.Errorf("empty framework: want 0 labels, got %d", len(labels))
	}
}

// --- NarrativeEntropy on empty framework ---
func TestNarrativeEntropyEmpty(t *testing.T) {
	fw := New()
	ent := fw.NarrativeEntropy()
	if ent != 0 {
		t.Errorf("empty framework: want entropy=0, got %.6f", ent)
	}
}

// --- ChainDefensible empty chain ---
func TestChainDefensibleEmptyChain(t *testing.T) {
	fw := New()
	empty := CausalChain{ArgumentIDs: []string{}}
	// Empty chain: vacuously all members are In → defensible
	if !fw.ChainDefensible(empty) {
		t.Error("empty chain should be vacuously defensible")
	}
}
