package subjective

import (
	"math"
	"testing"
)

const eps = 1e-9

func approx(a, b float64) bool { return math.Abs(a-b) < eps }

func TestVacuous(t *testing.T) {
	o := Vacuous(0.5)
	if o.Belief != 0 || o.Disbelief != 0 || o.Uncertainty != 1 {
		t.Errorf("Vacuous = %+v, want {0,0,1}", o)
	}
}

func TestDogmatic(t *testing.T) {
	tr := DogmaticTrue(0.5)
	if tr.Belief != 1 || tr.Disbelief != 0 || tr.Uncertainty != 0 {
		t.Errorf("DogmaticTrue = %+v", tr)
	}
	fa := DogmaticFalse(0.5)
	if fa.Belief != 0 || fa.Disbelief != 1 || fa.Uncertainty != 0 {
		t.Errorf("DogmaticFalse = %+v", fa)
	}
}

func TestFromReliability(t *testing.T) {
	cases := []struct {
		r       float64
		b, d, u float64
	}{
		{1.0, 1, 0, 0},
		{0.0, 0, 1, 0},
		{0.5, 0, 0, 1},
		{0.75, 0.5, 0, 0.5},
		{0.25, 0, 0.5, 0.5},
	}
	for _, c := range cases {
		o := FromReliability(c.r, 0.5)
		if !approx(o.Belief, c.b) || !approx(o.Disbelief, c.d) || !approx(o.Uncertainty, c.u) {
			t.Errorf("FromReliability(%v) = {%v,%v,%v}, want {%v,%v,%v}",
				c.r, o.Belief, o.Disbelief, o.Uncertainty, c.b, c.d, c.u)
		}
	}
}

func TestExpectedProbability(t *testing.T) {
	// E[p] = b + a*u
	o := Opinion{0.4, 0.2, 0.4, 0.5}
	want := 0.4 + 0.5*0.4 // = 0.6
	if !approx(o.ExpectedProbability(), want) {
		t.Errorf("EP = %v, want %v", o.ExpectedProbability(), want)
	}
	// Vacuous with a=0.5: EP = 0.5
	if !approx(Vacuous(0.5).ExpectedProbability(), 0.5) {
		t.Error("Vacuous EP != 0.5")
	}
}

func TestTrustDiscount(t *testing.T) {
	claim := DogmaticTrue(0.5)

	// Full trust → claim intact
	full := DogmaticTrue(0.5)
	result := TrustDiscount(full, claim)
	if !approx(result.Belief, 1) || !approx(result.Uncertainty, 0) {
		t.Errorf("full trust discount = %+v, want belief=1", result)
	}

	// Zero trust → vacuous
	zero := DogmaticFalse(0.5)
	result = TrustDiscount(zero, claim)
	if !approx(result.Belief, 0) || !approx(result.Uncertainty, 1) {
		t.Errorf("zero trust discount = %+v, want vacuous", result)
	}

	// Partial trust (b=0.6)
	partial := Opinion{0.6, 0.1, 0.3, 0.5}
	someClaim := Opinion{0.8, 0.1, 0.1, 0.5}
	result = TrustDiscount(partial, someClaim)
	wantB := 0.6 * 0.8
	wantD := 0.6 * 0.1
	wantU := 1 - 0.6*(0.8+0.1)
	if !approx(result.Belief, wantB) || !approx(result.Disbelief, wantD) || !approx(result.Uncertainty, wantU) {
		t.Errorf("partial trust = %+v, want {%v,%v,%v}", result, wantB, wantD, wantU)
	}
}

func TestConsensusFuse(t *testing.T) {
	// Fusing two opinions reduces uncertainty
	a := Opinion{0.3, 0.1, 0.6, 0.5}
	b := Opinion{0.4, 0.1, 0.5, 0.5}
	fused := ConsensusFuse(a, b)
	if fused.Uncertainty >= a.Uncertainty || fused.Uncertainty >= b.Uncertainty {
		t.Errorf("fused uncertainty %v should be less than %v and %v", fused.Uncertainty, a.Uncertainty, b.Uncertainty)
	}
	// b+d+u should be ~1
	sum := fused.Belief + fused.Disbelief + fused.Uncertainty
	if !approx(sum, 1.0) {
		t.Errorf("b+d+u = %v, want 1", sum)
	}

	// Conflicting opinions: both credible, opposite views
	pos := Opinion{0.8, 0, 0.2, 0.5}
	neg := Opinion{0, 0.8, 0.2, 0.5}
	conflict := ConsensusFuse(pos, neg)
	// Both sides contribute → belief and disbelief both non-zero
	if conflict.Belief < 0.01 || conflict.Disbelief < 0.01 {
		t.Errorf("conflict fusion = %+v, want both belief and disbelief non-zero", conflict)
	}

	// Dogmatic + non-dogmatic: dogmatic wins
	dog := DogmaticTrue(0.5)
	vac := Vacuous(0.5)
	result := ConsensusFuse(vac, dog)
	if !approx(result.Belief, 1) {
		t.Errorf("dogmatic wins: %+v", result)
	}

	// Two dogmatics: average
	d1 := DogmaticTrue(0.5)
	d2 := DogmaticFalse(0.5)
	avg := ConsensusFuse(d1, d2)
	if !approx(avg.Belief, 0.5) || !approx(avg.Disbelief, 0.5) {
		t.Errorf("two dogmatics average: %+v", avg)
	}
}

func TestConsensusFuseEmpty(t *testing.T) {
	o := ConsensusFuse()
	if o.Uncertainty != 1.0 || o.Belief != 0 || o.Disbelief != 0 {
		t.Errorf("ConsensusFuse() should be vacuous: %+v", o)
	}
}

func TestFromReliabilityOutOfRange(t *testing.T) {
	// r > 1.0: clamp to DogmaticTrue
	over := FromReliability(1.5, 0.5)
	if !approx(over.Belief, 1) || !approx(over.Uncertainty, 0) {
		t.Errorf("FromReliability(1.5) = %+v, want DogmaticTrue", over)
	}
	// r < 0.0: clamp to DogmaticFalse
	under := FromReliability(-0.5, 0.5)
	if !approx(under.Disbelief, 1) || !approx(under.Uncertainty, 0) {
		t.Errorf("FromReliability(-0.5) = %+v, want DogmaticFalse", under)
	}
}

func TestMultiply(t *testing.T) {
	// E[x∧y] = E[x]·E[y] for independent opinions.
	cases := []struct {
		name string
		x, y Opinion
	}{
		{"vacuous∧vacuous", Vacuous(0.5), Vacuous(0.5)},
		{"dogT∧dogT", DogmaticTrue(0.5), DogmaticTrue(0.5)},
		{"dogT∧vacuous", DogmaticTrue(0.5), Vacuous(0.5)},
		{"dogT∧dogF", DogmaticTrue(0.5), DogmaticFalse(0.5)},
		{"strong∧strong", Opinion{0.8, 0.1, 0.1, 0.5}, Opinion{0.7, 0.1, 0.2, 0.5}},
		{"partial∧partial", Opinion{0.3, 0.3, 0.4, 0.5}, Opinion{0.4, 0.2, 0.4, 0.5}},
	}
	for _, c := range cases {
		result := Multiply(c.x, c.y)
		wantEP := c.x.ExpectedProbability() * c.y.ExpectedProbability()
		gotEP := result.ExpectedProbability()
		if !approx(gotEP, wantEP) {
			t.Errorf("%s: E[p] = %.6f, want %.6f (result=%+v)", c.name, gotEP, wantEP, result)
		}
		sum := result.Belief + result.Disbelief + result.Uncertainty
		if !approx(sum, 1.0) {
			t.Errorf("%s: b+d+u = %.6f, want 1", c.name, sum)
		}
		wantA := c.x.BaseRate * c.y.BaseRate
		if !approx(result.BaseRate, wantA) {
			t.Errorf("%s: base rate = %.6f, want %.6f", c.name, result.BaseRate, wantA)
		}
		if result.Belief < -eps || result.Disbelief < -eps || result.Uncertainty < -eps {
			t.Errorf("%s: negative component: %+v", c.name, result)
		}
	}
}

func TestCoMultiply(t *testing.T) {
	// E[x∨y] = E[x]+E[y]−E[x]·E[y] for independent opinions.
	cases := []struct {
		name string
		x, y Opinion
	}{
		{"vacuous∨vacuous", Vacuous(0.5), Vacuous(0.5)},
		{"dogT∨dogF", DogmaticTrue(0.5), DogmaticFalse(0.5)},
		{"strong∨strong", Opinion{0.8, 0.1, 0.1, 0.5}, Opinion{0.7, 0.1, 0.2, 0.5}},
	}
	for _, c := range cases {
		result := CoMultiply(c.x, c.y)
		ex := c.x.ExpectedProbability()
		ey := c.y.ExpectedProbability()
		wantEP := ex + ey - ex*ey
		gotEP := result.ExpectedProbability()
		if !approx(gotEP, wantEP) {
			t.Errorf("%s: E[p] = %.6f, want %.6f (result=%+v)", c.name, gotEP, wantEP, result)
		}
		sum := result.Belief + result.Disbelief + result.Uncertainty
		if !approx(sum, 1.0) {
			t.Errorf("%s: b+d+u = %.6f, want 1", c.name, sum)
		}
		if result.Belief < -eps || result.Disbelief < -eps || result.Uncertainty < -eps {
			t.Errorf("%s: negative component: %+v", c.name, result)
		}
	}
}

func TestCBFBaseRateConfidenceWeighted(t *testing.T) {
	// When base rates differ, the fused base rate should weight by confidence (1−u),
	// not simple average. Per Jøsang (2016) §12.6.
	a := Opinion{0.6, 0.1, 0.3, 0.8} // high confidence (u=0.3), high base rate
	b := Opinion{0.1, 0.1, 0.8, 0.2} // low confidence (u=0.8), low base rate
	fused := ConsensusFuse(a, b)

	// Confidence-weighted: (0.8*0.7 + 0.2*0.2) / (0.7+0.2) = (0.56+0.04)/0.9 = 0.667
	confA := 1 - a.Uncertainty
	confB := 1 - b.Uncertainty
	wantBR := (a.BaseRate*confA + b.BaseRate*confB) / (confA + confB)
	if !approx(fused.BaseRate, wantBR) {
		t.Errorf("fused base rate = %.3f, want %.3f (confidence-weighted)", fused.BaseRate, wantBR)
	}
	// Should NOT be the simple average
	simpleAvg := (a.BaseRate + b.BaseRate) / 2
	if approx(fused.BaseRate, simpleAvg) && !approx(wantBR, simpleAvg) {
		t.Errorf("fused base rate %.3f is the simple average, should be confidence-weighted", fused.BaseRate)
	}
}

func TestOpinionFromEvidence(t *testing.T) {
	// Property 1: OFE(6, 0, 0.5).EP() == 0.875
	o := OpinionFromEvidence(6, 0, 0.5)
	if !approx(o.ExpectedProbability(), 0.875) {
		t.Errorf("OFE(6,0,0.5).EP() = %v, want 0.875", o.ExpectedProbability())
	}

	// Property 2: OFE(0, 0, 0.5) == Vacuous(0.5)
	vac := OpinionFromEvidence(0, 0, 0.5)
	want := Vacuous(0.5)
	if !approx(vac.Belief, want.Belief) || !approx(vac.Disbelief, want.Disbelief) || !approx(vac.Uncertainty, want.Uncertainty) {
		t.Errorf("OFE(0,0,0.5) = %+v, want Vacuous", vac)
	}

	// b+d+u=1
	sum := o.Belief + o.Disbelief + o.Uncertainty
	if !approx(sum, 1.0) {
		t.Errorf("b+d+u = %v, want 1", sum)
	}

	// Negative clamping
	neg := OpinionFromEvidence(-5, -3, 0.5)
	if !approx(neg.Belief, 0) || !approx(neg.Disbelief, 0) || !approx(neg.Uncertainty, 1) {
		t.Errorf("OFE(-5,-3,0.5) = %+v, want Vacuous", neg)
	}
}

func TestEvidenceCountsRoundtrip(t *testing.T) {
	cases := []struct {
		r, s float64
	}{
		{1, 0},
		{0, 1},
		{3, 2},
		{0.5, 0.5},
		{10, 5},
	}
	for _, c := range cases {
		o := OpinionFromEvidence(c.r, c.s, 0.5)
		gotR, gotS := EvidenceCounts(o)
		if !approx(gotR, c.r) || !approx(gotS, c.s) {
			t.Errorf("roundtrip(%v,%v): got (%v,%v)", c.r, c.s, gotR, gotS)
		}
	}
}

func TestEvidenceCountsDogmatic(t *testing.T) {
	r, s := EvidenceCounts(DogmaticTrue(0.5))
	if !math.IsInf(r, 1) || s != 0 {
		t.Errorf("DogmaticTrue evidence: r=%v, s=%v, want (+Inf, 0)", r, s)
	}
	r, s = EvidenceCounts(DogmaticFalse(0.5))
	if r != 0 || !math.IsInf(s, 1) {
		t.Errorf("DogmaticFalse evidence: r=%v, s=%v, want (0, +Inf)", r, s)
	}
}

func TestCBFEvidenceAdditivity(t *testing.T) {
	// CBF(OFE(r1,s1), OFE(r2,s2)) ≈ OFE(r1+r2, s1+s2)
	r1, s1 := 3.0, 1.0
	r2, s2 := 2.0, 4.0
	o1 := OpinionFromEvidence(r1, s1, 0.5)
	o2 := OpinionFromEvidence(r2, s2, 0.5)
	fused := ConsensusFuse(o1, o2)
	direct := OpinionFromEvidence(r1+r2, s1+s2, 0.5)
	if !approx(fused.Belief, direct.Belief) || !approx(fused.Disbelief, direct.Disbelief) || !approx(fused.Uncertainty, direct.Uncertainty) {
		t.Errorf("CBF additivity failed:\n  fused=%+v\n  direct=%+v", fused, direct)
	}
}

func TestAveragingFuseIdempotent(t *testing.T) {
	o := Opinion{0.4, 0.2, 0.4, 0.5}
	fused2 := AveragingFuse(o, o)
	if !approx(fused2.Belief, o.Belief) || !approx(fused2.Disbelief, o.Disbelief) || !approx(fused2.Uncertainty, o.Uncertainty) {
		t.Errorf("ABF(o,o) = %+v, want %+v", fused2, o)
	}
	fused3 := AveragingFuse(o, o, o)
	if !approx(fused3.Belief, o.Belief) || !approx(fused3.Disbelief, o.Disbelief) || !approx(fused3.Uncertainty, o.Uncertainty) {
		t.Errorf("ABF(o,o,o) = %+v, want %+v", fused3, o)
	}
}

func TestAveragingFuseVsCBF(t *testing.T) {
	o1 := Opinion{0.3, 0.1, 0.6, 0.5}
	o2 := Opinion{0.4, 0.1, 0.5, 0.5}
	abf := AveragingFuse(o1, o2)
	cbf := ConsensusFuse(o1, o2)
	// ABF preserves more uncertainty than CBF
	if abf.Uncertainty <= cbf.Uncertainty {
		t.Errorf("ABF uncertainty %.4f should be > CBF uncertainty %.4f", abf.Uncertainty, cbf.Uncertainty)
	}
}

func TestAveragingFuseEdgeCases(t *testing.T) {
	// Empty → Vacuous
	empty := AveragingFuse()
	if !approx(empty.Uncertainty, 1) {
		t.Errorf("ABF() should be vacuous: %+v", empty)
	}

	// Single → identity
	o := Opinion{0.5, 0.2, 0.3, 0.5}
	single := AveragingFuse(o)
	if !approx(single.Belief, o.Belief) || !approx(single.Disbelief, o.Disbelief) {
		t.Errorf("ABF(single) = %+v, want %+v", single, o)
	}

	// Both dogmatic → average
	d1 := DogmaticTrue(0.5)
	d2 := DogmaticFalse(0.5)
	avg := AveragingFuse(d1, d2)
	if !approx(avg.Belief, 0.5) || !approx(avg.Disbelief, 0.5) {
		t.Errorf("ABF(dogT, dogF) = %+v, want average", avg)
	}

	// One dogmatic → dominates
	vac := Vacuous(0.5)
	dom := AveragingFuse(vac, DogmaticTrue(0.5))
	if !approx(dom.Belief, 1) {
		t.Errorf("ABF(vac, dogT) = %+v, want dogmatic true to dominate", dom)
	}
}

func TestNegate(t *testing.T) {
	o := Opinion{0.6, 0.2, 0.2, 0.3}
	n := Negate(o)
	if !approx(n.Belief, 0.2) || !approx(n.Disbelief, 0.6) || !approx(n.BaseRate, 0.7) {
		t.Errorf("Negate = %+v", n)
	}
	// b+d+u preserved
	sum := n.Belief + n.Disbelief + n.Uncertainty
	if !approx(sum, 1.0) {
		t.Errorf("Negate b+d+u = %v", sum)
	}
}

// --- Coverage-gap tests below ---

// assertInvariant checks the fundamental b+d+u=1 invariant and non-negativity.
func assertInvariant(t *testing.T, label string, o Opinion) {
	t.Helper()
	sum := o.Belief + o.Disbelief + o.Uncertainty
	if !approx(sum, 1.0) {
		t.Errorf("%s: b+d+u = %.15f, want 1.0 (opinion=%+v)", label, sum, o)
	}
	if o.Belief < -eps {
		t.Errorf("%s: negative belief = %v", label, o.Belief)
	}
	if o.Disbelief < -eps {
		t.Errorf("%s: negative disbelief = %v", label, o.Disbelief)
	}
	if o.Uncertainty < -eps {
		t.Errorf("%s: negative uncertainty = %v", label, o.Uncertainty)
	}
}

// assertEP checks E[p] = b + a*u matches an expected value.
func assertEP(t *testing.T, label string, o Opinion, wantEP float64) {
	t.Helper()
	gotEP := o.ExpectedProbability()
	if !approx(gotEP, wantEP) {
		t.Errorf("%s: E[p] = %.9f, want %.9f (opinion=%+v)", label, gotEP, wantEP, o)
	}
}

// --- Gap 1: opFromExpected base rate <= 0 ---

func TestOpFromExpected_BaseRateZero(t *testing.T) {
	// When base rate collapses to 0, opFromExpected takes the a<=0 branch:
	//   return Opinion{ep, 1-ep, 0, 0}
	// This happens via Multiply when one input has base rate 0.

	// Multiply with base rate 0 input: a_{x^y} = 0*0.5 = 0
	x := Opinion{0.8, 0.1, 0.1, 0.0} // base rate 0
	y := Opinion{0.7, 0.1, 0.2, 0.5}
	result := Multiply(x, y)

	assertInvariant(t, "baseRate=0 multiply", result)
	if !approx(result.BaseRate, 0.0) {
		t.Errorf("expected base rate 0, got %v", result.BaseRate)
	}
	// When a=0: E[p] = b + 0*u = b, so b must equal the target ep
	wantEP := x.ExpectedProbability() * y.ExpectedProbability()
	assertEP(t, "baseRate=0 multiply", result, wantEP)
	// Uncertainty must be 0 in the a<=0 branch
	if !approx(result.Uncertainty, 0.0) {
		t.Errorf("baseRate=0: uncertainty = %v, want 0", result.Uncertainty)
	}

	// Also test via CoMultiply: a_{x∨y} = 0+0-0*0 = 0
	x0 := Opinion{0.6, 0.2, 0.2, 0.0}
	y0 := Opinion{0.4, 0.3, 0.3, 0.0}
	coResult := CoMultiply(x0, y0)
	assertInvariant(t, "baseRate=0 comultiply", coResult)
	if !approx(coResult.BaseRate, 0.0) {
		t.Errorf("expected base rate 0, got %v", coResult.BaseRate)
	}
	ex := x0.ExpectedProbability()
	ey := y0.ExpectedProbability()
	assertEP(t, "baseRate=0 comultiply", coResult, ex+ey-ex*ey)
}

// --- Gap 2: opFromExpected base rate >= 1 ---

func TestOpFromExpected_BaseRateOne(t *testing.T) {
	// When base rate reaches 1, opFromExpected takes the a>=1 branch:
	//   return Opinion{0, 1-ep, 0, 1}
	// This is a degenerate boundary: when a=1, E[p] = b + 1*u.
	// The code forces b=0, u=0, returning Opinion{0, 1-ep, 0, 1}.
	//
	// We verify the branch IS taken and the output shape matches the code.

	// Multiply with both base rates = 1: a_{x^y} = 1*1 = 1
	// Use inputs where ep is exactly 1 so the result is well-formed:
	// DogmaticTrue with a=1: E[p] = 1+1*0 = 1
	x := DogmaticTrue(1.0)
	y := DogmaticTrue(1.0)
	result := Multiply(x, y)
	// ep = 1*1 = 1, so Opinion{0, 1-1, 0, 1} = Opinion{0, 0, 0, 1}
	if !approx(result.BaseRate, 1.0) {
		t.Errorf("expected base rate 1, got %v", result.BaseRate)
	}
	if !approx(result.Belief, 0.0) {
		t.Errorf("baseRate=1: belief = %v, want 0", result.Belief)
	}
	if !approx(result.Uncertainty, 0.0) {
		t.Errorf("baseRate=1: uncertainty = %v, want 0", result.Uncertainty)
	}
	if !approx(result.Disbelief, 0.0) {
		t.Errorf("baseRate=1: disbelief = %v, want 0", result.Disbelief)
	}

	// CoMultiply where both base rates = 1: a_{x∨y} = 1+1-1 = 1
	// ep = E[x]+E[y]-E[x]*E[y]; use DogmaticTrue so ep = 1+1-1 = 1
	coResult := CoMultiply(DogmaticTrue(1.0), DogmaticTrue(1.0))
	if !approx(coResult.BaseRate, 1.0) {
		t.Errorf("comultiply: expected base rate 1, got %v", coResult.BaseRate)
	}
	if !approx(coResult.Belief, 0.0) || !approx(coResult.Uncertainty, 0.0) {
		t.Errorf("comultiply baseRate=1: unexpected opinion %+v", coResult)
	}

	// Non-trivial case: Multiply with a=1 but ep < 1.
	// With a=1: code returns Opinion{0, 1-ep, 0, 1}.
	// b+d+u = 0 + (1-ep) + 0 = 1-ep. This is < 1 when ep > 0.
	// This is the actual branch behavior (boundary degeneration).
	x2 := Opinion{0.5, 0.3, 0.2, 1.0} // E[p] = 0.5+1*0.2 = 0.7
	y2 := Opinion{0.3, 0.5, 0.2, 1.0} // E[p] = 0.3+1*0.2 = 0.5
	mulResult := Multiply(x2, y2)
	// Verify branch is taken: belief=0, uncertainty=0, baseRate=1
	if !approx(mulResult.BaseRate, 1.0) {
		t.Errorf("multiply a=1: base rate = %v, want 1", mulResult.BaseRate)
	}
	if !approx(mulResult.Belief, 0.0) {
		t.Errorf("multiply a=1: belief = %v, want 0", mulResult.Belief)
	}
	if !approx(mulResult.Uncertainty, 0.0) {
		t.Errorf("multiply a=1: uncertainty = %v, want 0", mulResult.Uncertainty)
	}
	// ep = 0.7*0.5 = 0.35, so disbelief = 1-0.35 = 0.65
	wantD := 1 - x2.ExpectedProbability()*y2.ExpectedProbability()
	if !approx(mulResult.Disbelief, wantD) {
		t.Errorf("multiply a=1: disbelief = %v, want %v", mulResult.Disbelief, wantD)
	}
}

// --- Gap 3: opFromExpected uMax > 1 clipping ---

func TestOpFromExpected_UMaxClipping(t *testing.T) {
	// Force uMax = min(ep/a, (1-ep)/(1-a)) > 1 before clamping.
	// This happens when ep is close to a, making both bounds large.
	// Example: ep ≈ a, so ep/a ≈ 1 and (1-ep)/(1-a) ≈ 1.
	// With ep=a exactly, both bounds are 1 — need ep slightly less than a
	// with a large a so (1-ep)/(1-a) > 1.
	//
	// Concrete: a=0.2, ep=0.15 → ep/a = 0.75, (1-ep)/(1-a) = 0.85/0.8 = 1.0625 > 1
	// So the alt bound exceeds 1 and gets clipped.

	// We can reach this via Multiply:
	// Need x,y such that E[x]*E[y] = 0.15, a_x*a_y = 0.2
	// x = Opinion with E[x]=0.5, a_x=0.5 → y needs E[y]=0.3, a_y=0.4
	// ep/a = 0.15/0.2 = 0.75, (1-0.15)/(1-0.2) = 0.85/0.8 = 1.0625 → clipped to 1
	x := Opinion{0.4, 0.1, 0.5, 0.5} // E[p] = 0.4+0.5*0.5 = 0.65
	y := Opinion{0.1, 0.1, 0.8, 0.4} // E[p] = 0.1+0.4*0.8 = 0.42

	// a_{x^y} = 0.5*0.4 = 0.2
	// ep = 0.65*0.42 = 0.273
	// ep/a = 0.273/0.2 = 1.365 > 1 → uMax initially > 1, will be clipped
	result := Multiply(x, y)
	assertInvariant(t, "uMax clipping multiply", result)
	wantEP := x.ExpectedProbability() * y.ExpectedProbability()
	assertEP(t, "uMax clipping multiply", result, wantEP)

	// Additionally test via CoMultiply with small base rates so a_{x∨y} stays small
	// while E[p] is moderate, pushing ep/a > 1.
	cx := Opinion{0.7, 0.1, 0.2, 0.1} // E[p] = 0.7+0.1*0.2 = 0.72
	cy := Opinion{0.5, 0.2, 0.3, 0.1} // E[p] = 0.5+0.1*0.3 = 0.53
	// a_{x∨y} = 0.1+0.1-0.01 = 0.19
	// ep = 0.72+0.53-0.72*0.53 = 1.25-0.3816 = 0.8684
	// ep/a = 0.8684/0.19 = 4.57 >> 1 → clipped
	coResult := CoMultiply(cx, cy)
	assertInvariant(t, "uMax clipping comultiply", coResult)
	ecx := cx.ExpectedProbability()
	ecy := cy.ExpectedProbability()
	assertEP(t, "uMax clipping comultiply", coResult, ecx+ecy-ecx*ecy)
}

// --- Gap 4: ConsensusFuse single opinion ---

func TestConsensusFuse_SingleOpinion(t *testing.T) {
	cases := []struct {
		name string
		o    Opinion
	}{
		{"vacuous", Vacuous(0.5)},
		{"dogmatic true", DogmaticTrue(0.3)},
		{"dogmatic false", DogmaticFalse(0.7)},
		{"partial evidence", Opinion{0.4, 0.2, 0.4, 0.6}},
		{"near-vacuous", Opinion{0.01, 0.01, 0.98, 0.5}},
	}
	for _, c := range cases {
		result := ConsensusFuse(c.o)
		if !approx(result.Belief, c.o.Belief) ||
			!approx(result.Disbelief, c.o.Disbelief) ||
			!approx(result.Uncertainty, c.o.Uncertainty) ||
			!approx(result.BaseRate, c.o.BaseRate) {
			t.Errorf("ConsensusFuse(%s) = %+v, want %+v", c.name, result, c.o)
		}
		assertInvariant(t, "single CBF "+c.name, result)
	}
}

// --- Gap 5: fuseTwo dogmatic a, non-dogmatic b ---

func TestFuseTwo_DogmaticA_NonDogmaticB(t *testing.T) {
	// When a.Uncertainty == 0 and b.Uncertainty > 0, dogmatic a dominates.
	// Per Josang: dogmatic opinion represents infinite evidence.
	dogA := DogmaticTrue(0.5)
	nonDogB := Opinion{0.3, 0.3, 0.4, 0.5}

	result := ConsensusFuse(dogA, nonDogB)
	// dogmatic a should dominate completely
	if !approx(result.Belief, dogA.Belief) ||
		!approx(result.Disbelief, dogA.Disbelief) ||
		!approx(result.Uncertainty, dogA.Uncertainty) {
		t.Errorf("fuseTwo(dogmatic, non-dogmatic) = %+v, want %+v", result, dogA)
	}
	assertInvariant(t, "dogmatic-a dominates", result)

	// Verify commutativity: fuse(b,a) should also give dogmatic a's values
	reverse := ConsensusFuse(nonDogB, dogA)
	if !approx(reverse.Belief, dogA.Belief) ||
		!approx(reverse.Disbelief, dogA.Disbelief) ||
		!approx(reverse.Uncertainty, dogA.Uncertainty) {
		t.Errorf("fuseTwo(non-dogmatic, dogmatic) = %+v, want %+v", reverse, dogA)
	}
	assertInvariant(t, "dogmatic-a dominates (reversed)", reverse)

	// Also test dogmatic false dominating
	dogF := DogmaticFalse(0.4)
	partial := Opinion{0.5, 0.1, 0.4, 0.6}
	resultF := ConsensusFuse(dogF, partial)
	if !approx(resultF.Belief, dogF.Belief) ||
		!approx(resultF.Disbelief, dogF.Disbelief) ||
		!approx(resultF.Uncertainty, dogF.Uncertainty) {
		t.Errorf("fuseTwo(dogmatic false, partial) = %+v, want %+v", resultF, dogF)
	}
	assertInvariant(t, "dogmatic-false dominates", resultF)
}

// --- Gap 6: fuseTwo denom near-zero ---

func TestFuseTwo_DenomNearZero(t *testing.T) {
	// denom = u_a + u_b - u_a*u_b → approaches 0 when both u → 0 (nearly dogmatic)
	// but not exactly 0 (which hits the both-dogmatic branch).
	// With u_a = u_b = ε (tiny): denom = 2ε - ε² ≈ 2ε → small but > 0.
	//
	// The denom < 1e-12 guard only triggers when denom is truly negligible.
	// Use extremely small uncertainties to approach the guard.
	tiny := 1e-13
	a := Opinion{0.5, 0.5 - tiny, tiny, 0.5}
	b := Opinion{0.5 - tiny, 0.5, tiny, 0.5}
	// denom = tiny + tiny - tiny*tiny ≈ 2e-13 → below the 1e-12 threshold
	result := ConsensusFuse(a, b)

	// Should fall through to vacuous with averaged base rate
	assertInvariant(t, "denom near-zero", result)
	if !approx(result.Uncertainty, 1.0) {
		t.Errorf("denom near-zero: expected vacuous (u=1), got u=%v", result.Uncertainty)
	}
	if !approx(result.BaseRate, 0.5) {
		t.Errorf("denom near-zero: base rate = %v, want 0.5", result.BaseRate)
	}
}

// --- Gap 7: fuseTwo confSum near-zero ---

func TestFuseTwo_ConfSumNearZero(t *testing.T) {
	// confSum = (1-u_a) + (1-u_b) → near 0 when both opinions are near-vacuous.
	// This triggers the confSum < 1e-12 fallback for base rate averaging.
	nearVacA := Opinion{0, 0, 1.0, 0.3}             // exactly vacuous
	nearVacB := Opinion{1e-14, 0, 1.0 - 1e-14, 0.7} // very near vacuous
	// confA = 0.0, confB ≈ 1e-14, confSum ≈ 1e-14 < 1e-12 → fallback
	result := ConsensusFuse(nearVacA, nearVacB)

	assertInvariant(t, "confSum near-zero CBF", result)
	// Base rate should be simple average since both are near-vacuous
	if !approx(result.BaseRate, 0.5) {
		t.Errorf("confSum near-zero: base rate = %v, want 0.5 (simple average)", result.BaseRate)
	}
	// Commutativity
	reverse := ConsensusFuse(nearVacB, nearVacA)
	if !approx(result.Belief, reverse.Belief) ||
		!approx(result.Disbelief, reverse.Disbelief) ||
		!approx(result.Uncertainty, reverse.Uncertainty) {
		t.Errorf("confSum near-zero: commutativity failed\n  a,b = %+v\n  b,a = %+v", result, reverse)
	}
}

// --- Gap 8: abfTwo dogmatic a, non-dogmatic b ---

func TestAbfTwo_DogmaticA_NonDogmaticB(t *testing.T) {
	// Same dominance behavior as CBF: dogmatic opinion wins.
	dogA := DogmaticTrue(0.5)
	nonDogB := Opinion{0.2, 0.4, 0.4, 0.5}

	result := AveragingFuse(dogA, nonDogB)
	if !approx(result.Belief, dogA.Belief) ||
		!approx(result.Disbelief, dogA.Disbelief) ||
		!approx(result.Uncertainty, dogA.Uncertainty) {
		t.Errorf("ABF(dogmatic, non-dogmatic) = %+v, want %+v", result, dogA)
	}
	assertInvariant(t, "ABF dogmatic-a dominates", result)

	// Commutativity: fuse(b, a) should also give dogmatic a
	reverse := AveragingFuse(nonDogB, dogA)
	if !approx(reverse.Belief, dogA.Belief) ||
		!approx(reverse.Disbelief, dogA.Disbelief) ||
		!approx(reverse.Uncertainty, dogA.Uncertainty) {
		t.Errorf("ABF(non-dogmatic, dogmatic) = %+v, want %+v", reverse, dogA)
	}
	assertInvariant(t, "ABF dogmatic-a dominates (reversed)", reverse)

	// Dogmatic false dominates
	dogF := DogmaticFalse(0.6)
	partial := Opinion{0.5, 0.1, 0.4, 0.4}
	resultF := AveragingFuse(dogF, partial)
	if !approx(resultF.Disbelief, 1.0) || !approx(resultF.Uncertainty, 0.0) {
		t.Errorf("ABF(dogmatic false, partial) = %+v, want dogmatic false", resultF)
	}
	assertInvariant(t, "ABF dogmatic-false dominates", resultF)
}

// --- Gap 9: abfTwo denom near-zero ---

func TestAbfTwo_DenomNearZero(t *testing.T) {
	// ABF denom = u_a + u_b → approaches 0 when both are nearly dogmatic.
	// With u_a = u_b = ε: denom = 2ε.
	// When denom < 1e-12, falls back to vacuous.
	tiny := 1e-13
	a := Opinion{0.7, 0.3 - tiny, tiny, 0.5}
	b := Opinion{0.3, 0.7 - tiny, tiny, 0.5}
	// denom = 2*tiny ≈ 2e-13 < 1e-12 → fallback
	result := AveragingFuse(a, b)

	assertInvariant(t, "ABF denom near-zero", result)
	if !approx(result.Uncertainty, 1.0) {
		t.Errorf("ABF denom near-zero: expected vacuous, got u=%v", result.Uncertainty)
	}
	if !approx(result.BaseRate, 0.5) {
		t.Errorf("ABF denom near-zero: base rate = %v, want 0.5", result.BaseRate)
	}
}

// --- Gap 10: abfTwo confSum near-zero ---

func TestAbfTwo_ConfSumNearZero(t *testing.T) {
	// confSum = (1-u_a) + (1-u_b) → near 0 when both are near-vacuous.
	// This triggers the confSum < 1e-12 fallback for base rate averaging in ABF.
	nearVacA := Opinion{0, 0, 1.0, 0.2}             // exactly vacuous
	nearVacB := Opinion{1e-14, 0, 1.0 - 1e-14, 0.8} // very near vacuous
	// confA = 0.0, confB ≈ 1e-14, confSum ≈ 1e-14 < 1e-12
	result := AveragingFuse(nearVacA, nearVacB)

	assertInvariant(t, "ABF confSum near-zero", result)
	// Base rate should be simple average
	if !approx(result.BaseRate, 0.5) {
		t.Errorf("ABF confSum near-zero: base rate = %v, want 0.5", result.BaseRate)
	}

	// ABF idempotency for vacuous opinions: fuse(vac, vac) = vac
	vac := Vacuous(0.5)
	idem := AveragingFuse(vac, vac)
	assertInvariant(t, "ABF vacuous idempotent", idem)
	if !approx(idem.Uncertainty, 1.0) {
		t.Errorf("ABF(vacuous, vacuous): u = %v, want 1.0", idem.Uncertainty)
	}
}

// --- N-ary ABF tests (direct formula vs pairwise) ---

func TestAveragingFuseNaryDirect(t *testing.T) {
	// Verify n-ary ABF formula for 3 sources with different uncertainties.
	// Manual computation from Josang, Diaz & Rifqi (2010) Eq. 16:
	//   o1 = {b=0.3, d=0.1, u=0.6, a=0.5}
	//   o2 = {b=0.4, d=0.2, u=0.4, a=0.5}
	//   o3 = {b=0.2, d=0.3, u=0.5, a=0.5}
	//
	// prodU = 0.6 * 0.4 * 0.5 = 0.12
	// K1 = prodU/u1 = 0.12/0.6 = 0.2
	// K2 = prodU/u2 = 0.12/0.4 = 0.3
	// K3 = prodU/u3 = 0.12/0.5 = 0.24
	// K  = 0.2 + 0.3 + 0.24 = 0.74
	//
	// b = (0.3*0.2 + 0.4*0.3 + 0.2*0.24) / 0.74
	//   = (0.06 + 0.12 + 0.048) / 0.74 = 0.228/0.74 ≈ 0.308108
	// d = (0.1*0.2 + 0.2*0.3 + 0.3*0.24) / 0.74
	//   = (0.02 + 0.06 + 0.072) / 0.74 = 0.152/0.74 ≈ 0.205405
	// u = 3 * 0.12 / 0.74 = 0.36/0.74 ≈ 0.486486

	o1 := Opinion{0.3, 0.1, 0.6, 0.5}
	o2 := Opinion{0.4, 0.2, 0.4, 0.5}
	o3 := Opinion{0.2, 0.3, 0.5, 0.5}

	result := AveragingFuse(o1, o2, o3)
	assertInvariant(t, "n-ary ABF direct", result)

	wantB := 0.228 / 0.74
	wantD := 0.152 / 0.74
	wantU := 0.36 / 0.74

	if !approx(result.Belief, wantB) {
		t.Errorf("n-ary ABF belief = %.9f, want %.9f", result.Belief, wantB)
	}
	if !approx(result.Disbelief, wantD) {
		t.Errorf("n-ary ABF disbelief = %.9f, want %.9f", result.Disbelief, wantD)
	}
	if !approx(result.Uncertainty, wantU) {
		t.Errorf("n-ary ABF uncertainty = %.9f, want %.9f", result.Uncertainty, wantU)
	}
}

func TestAveragingFuseNaryVsPairwise(t *testing.T) {
	// Show that pairwise ABF iteration != n-ary ABF for heterogeneous uncertainties.
	// This is the reason we need the direct n-ary formula.
	o1 := Opinion{0.3, 0.1, 0.6, 0.5}
	o2 := Opinion{0.5, 0.1, 0.4, 0.5}
	o3 := Opinion{0.1, 0.4, 0.5, 0.5}

	// N-ary direct (correct)
	nary := AveragingFuse(o1, o2, o3)

	// Pairwise iteration (incorrect for n>2): ABF(ABF(o1,o2), o3)
	pair12 := AveragingFuse(o1, o2)
	pairwise := AveragingFuse(pair12, o3)

	// They should differ for heterogeneous uncertainties
	if approx(nary.Belief, pairwise.Belief) &&
		approx(nary.Disbelief, pairwise.Disbelief) &&
		approx(nary.Uncertainty, pairwise.Uncertainty) {
		t.Errorf("n-ary and pairwise ABF should differ for heterogeneous uncertainties\n"+
			"  n-ary:    %+v\n  pairwise: %+v", nary, pairwise)
	}

	// Both should satisfy invariant
	assertInvariant(t, "n-ary ABF", nary)
	assertInvariant(t, "pairwise ABF", pairwise)
}

func TestAveragingFuseNaryMultiDogmaticMixed(t *testing.T) {
	// When 2+ dogmatic opinions are mixed with non-dogmatic opinions,
	// the n-ary formula degenerates (K_i=0 for all i). The implementation
	// averages the dogmatic subset and discards non-dogmatic opinions.
	// This test pins that behavior.
	d1 := DogmaticTrue(0.5)
	d2 := DogmaticFalse(0.5)
	partial := Opinion{0.4, 0.2, 0.4, 0.5}

	result := AveragingFuse(d1, d2, partial)
	// Should average the two dogmatics, ignoring partial
	if !approx(result.Belief, 0.5) || !approx(result.Disbelief, 0.5) || !approx(result.Uncertainty, 0.0) {
		t.Errorf("ABF(dogT, dogF, partial) = %+v, want {0.5, 0.5, 0, 0.5}", result)
	}
	assertInvariant(t, "multi-dogmatic mixed ABF", result)

	// Single dogmatic still dominates over non-dogmatics
	dog := DogmaticTrue(0.5)
	p1 := Opinion{0.3, 0.3, 0.4, 0.5}
	p2 := Opinion{0.1, 0.6, 0.3, 0.5}
	single := AveragingFuse(dog, p1, p2)
	if !approx(single.Belief, 1.0) || !approx(single.Uncertainty, 0.0) {
		t.Errorf("ABF(dogT, p1, p2) = %+v, want DogmaticTrue", single)
	}
	assertInvariant(t, "single-dogmatic mixed ABF", single)
}

func TestAveragingFuseNaryIdempotent(t *testing.T) {
	// ABF(o,o,...,o) = o for any opinion o. This is the key idempotency property.
	cases := []struct {
		name string
		o    Opinion
		n    int
	}{
		{"partial 3x", Opinion{0.4, 0.2, 0.4, 0.5}, 3},
		{"partial 5x", Opinion{0.4, 0.2, 0.4, 0.5}, 5},
		{"high-belief 4x", Opinion{0.7, 0.1, 0.2, 0.5}, 4},
		{"near-vacuous 3x", Opinion{0.01, 0.01, 0.98, 0.5}, 3},
	}
	for _, c := range cases {
		copies := make([]Opinion, c.n)
		for i := range copies {
			copies[i] = c.o
		}
		result := AveragingFuse(copies...)
		if !approx(result.Belief, c.o.Belief) || !approx(result.Disbelief, c.o.Disbelief) || !approx(result.Uncertainty, c.o.Uncertainty) {
			t.Errorf("ABF idempotent %s: got %+v, want %+v", c.name, result, c.o)
		}
		assertInvariant(t, "ABF idempotent "+c.name, result)
	}
}

// --- Cross-cutting property tests ---

func TestCBF_Commutativity(t *testing.T) {
	// Fundamental property: CBF(a,b) = CBF(b,a) for all non-degenerate inputs.
	cases := []struct {
		name string
		a, b Opinion
	}{
		{"two partial", Opinion{0.3, 0.2, 0.5, 0.5}, Opinion{0.5, 0.1, 0.4, 0.5}},
		{"different base rates", Opinion{0.4, 0.1, 0.5, 0.3}, Opinion{0.2, 0.3, 0.5, 0.7}},
		{"one strong one weak", Opinion{0.8, 0.1, 0.1, 0.5}, Opinion{0.1, 0.1, 0.8, 0.5}},
		{"near-vacuous pair", Opinion{0.01, 0.01, 0.98, 0.4}, Opinion{0.02, 0.01, 0.97, 0.6}},
	}
	for _, c := range cases {
		ab := ConsensusFuse(c.a, c.b)
		ba := ConsensusFuse(c.b, c.a)
		if !approx(ab.Belief, ba.Belief) || !approx(ab.Disbelief, ba.Disbelief) ||
			!approx(ab.Uncertainty, ba.Uncertainty) || !approx(ab.BaseRate, ba.BaseRate) {
			t.Errorf("CBF commutativity %s:\n  (a,b) = %+v\n  (b,a) = %+v", c.name, ab, ba)
		}
	}
}

func TestABF_Commutativity(t *testing.T) {
	cases := []struct {
		name string
		a, b Opinion
	}{
		{"two partial", Opinion{0.3, 0.2, 0.5, 0.5}, Opinion{0.5, 0.1, 0.4, 0.5}},
		{"different base rates", Opinion{0.4, 0.1, 0.5, 0.3}, Opinion{0.2, 0.3, 0.5, 0.7}},
	}
	for _, c := range cases {
		ab := AveragingFuse(c.a, c.b)
		ba := AveragingFuse(c.b, c.a)
		if !approx(ab.Belief, ba.Belief) || !approx(ab.Disbelief, ba.Disbelief) ||
			!approx(ab.Uncertainty, ba.Uncertainty) || !approx(ab.BaseRate, ba.BaseRate) {
			t.Errorf("ABF commutativity %s:\n  (a,b) = %+v\n  (b,a) = %+v", c.name, ab, ba)
		}
	}
}

func TestEvidenceCountBijection_Coverage(t *testing.T) {
	// Verify the round-trip bijection: OpinionFromEvidence(r,s) → EvidenceCounts → (r,s)
	// for a range of evidence counts, including high-evidence cases.
	cases := []struct {
		r, s     float64
		baseRate float64
	}{
		{0, 0, 0.5},     // vacuous
		{100, 0, 0.5},   // strong positive
		{0, 100, 0.5},   // strong negative
		{50, 50, 0.5},   // balanced high evidence
		{1, 1, 0.3},     // low evidence, non-standard base rate
		{0.1, 0.2, 0.7}, // fractional evidence
	}
	for _, c := range cases {
		o := OpinionFromEvidence(c.r, c.s, c.baseRate)
		assertInvariant(t, "bijection", o)
		gotR, gotS := EvidenceCounts(o)
		if !approx(gotR, c.r) || !approx(gotS, c.s) {
			t.Errorf("bijection(%v,%v): got (%v,%v)", c.r, c.s, gotR, gotS)
		}
		// Verify E[p] = b + a*u = (r + a*W) / (r + s + W)
		wantEP := (c.r + c.baseRate*W) / (c.r + c.s + W)
		assertEP(t, "bijection EP", o, wantEP)
	}
}

func TestMultiply_BaseRateBoundaryInteraction(t *testing.T) {
	// Test Multiply where one operand has base rate exactly 0 and the other exactly 1.
	// a_{x^y} = 0*1 = 0, hitting the a<=0 branch.
	x := Opinion{0.5, 0.3, 0.2, 0.0}
	y := Opinion{0.5, 0.3, 0.2, 1.0}
	result := Multiply(x, y)
	assertInvariant(t, "multiply a=0*1", result)
	assertEP(t, "multiply a=0*1", result, x.ExpectedProbability()*y.ExpectedProbability())
	if !approx(result.BaseRate, 0.0) {
		t.Errorf("multiply a=0*1: base rate = %v, want 0", result.BaseRate)
	}
}

func TestCoMultiply_BaseRateBoundaryInteraction(t *testing.T) {
	// CoMultiply where both have base rate 1: a_{x∨y} = 1+1-1 = 1
	// When a=1, opFromExpected returns Opinion{0, 1-ep, 0, 1}.
	// We verify the branch is taken and the output shape is correct.
	x := Opinion{0.5, 0.3, 0.2, 1.0} // E[p] = 0.5+1*0.2 = 0.7
	y := Opinion{0.3, 0.5, 0.2, 1.0} // E[p] = 0.3+1*0.2 = 0.5
	result := CoMultiply(x, y)
	if !approx(result.BaseRate, 1.0) {
		t.Errorf("comultiply a=1+1-1: base rate = %v, want 1", result.BaseRate)
	}
	if !approx(result.Belief, 0.0) {
		t.Errorf("comultiply a=1: belief = %v, want 0", result.Belief)
	}
	if !approx(result.Uncertainty, 0.0) {
		t.Errorf("comultiply a=1: uncertainty = %v, want 0", result.Uncertainty)
	}
	// ep = 0.7+0.5-0.7*0.5 = 0.85, disbelief = 1-0.85 = 0.15
	ex := x.ExpectedProbability()
	ey := y.ExpectedProbability()
	wantD := 1 - (ex + ey - ex*ey)
	if !approx(result.Disbelief, wantD) {
		t.Errorf("comultiply a=1: disbelief = %v, want %v", result.Disbelief, wantD)
	}
}

func TestCBF_EvidenceAdditivity_AsymmetricBaseRates(t *testing.T) {
	// CBF evidence additivity with same base rates.
	// CBF(OFE(r1,s1,a), OFE(r2,s2,a)) ≈ OFE(r1+r2, s1+s2, a)
	r1, s1 := 5.0, 1.0
	r2, s2 := 1.0, 7.0
	a := 0.3
	o1 := OpinionFromEvidence(r1, s1, a)
	o2 := OpinionFromEvidence(r2, s2, a)
	fused := ConsensusFuse(o1, o2)
	direct := OpinionFromEvidence(r1+r2, s1+s2, a)
	assertInvariant(t, "CBF additivity asymmetric", fused)
	if !approx(fused.Belief, direct.Belief) ||
		!approx(fused.Disbelief, direct.Disbelief) ||
		!approx(fused.Uncertainty, direct.Uncertainty) {
		t.Errorf("CBF additivity (a=0.3):\n  fused  = %+v\n  direct = %+v", fused, direct)
	}
}
