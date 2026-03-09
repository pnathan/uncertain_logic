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
