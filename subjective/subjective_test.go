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
