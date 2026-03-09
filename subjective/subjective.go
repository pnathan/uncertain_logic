package subjective

import "math"

// W is the non-informative prior weight corresponding to a Beta(1,1)
// uniform prior. Per Josang (2016) Ch. 3, W=2 maps evidence counts
// to/from opinions via the beta-binomial bijection.
const W = 2.0

// Opinion represents a subjective logic opinion triple with base rate.
// Invariant: Belief + Disbelief + Uncertainty = 1.0
type Opinion struct {
	Belief      float64 // b: degree of positive evidence
	Disbelief   float64 // d: degree of negative evidence
	Uncertainty float64 // u: lack of evidence; b+d+u=1
	BaseRate    float64 // a: prior probability
}

// Vacuous returns a pure uncertainty opinion (no evidence).
func Vacuous(baseRate float64) Opinion {
	return Opinion{0, 0, 1, baseRate}
}

// DogmaticTrue returns a certain belief opinion.
func DogmaticTrue(baseRate float64) Opinion {
	return Opinion{1, 0, 0, baseRate}
}

// DogmaticFalse returns a certain disbelief opinion.
func DogmaticFalse(baseRate float64) Opinion {
	return Opinion{0, 1, 0, baseRate}
}

// FromReliability maps r∈[0,1] to an opinion.
// r=0.5 → vacuous (0,0,1); r=1.0 → dogmatic true (1,0,0); r=0.0 → dogmatic false (0,1,0).
func FromReliability(r, baseRate float64) Opinion {
	r = clamp(r, 0, 1)
	switch {
	case r >= 1.0:
		return DogmaticTrue(baseRate)
	case r <= 0.0:
		return DogmaticFalse(baseRate)
	case r == 0.5:
		return Vacuous(baseRate)
	case r > 0.5:
		// Map [0.5, 1.0] → b in [0, 1], u decreasing
		b := (r - 0.5) * 2
		return Opinion{b, 0, 1 - b, baseRate}
	default:
		// Map [0.0, 0.5] → d in [1, 0], u decreasing
		d := (0.5 - r) * 2
		return Opinion{0, d, 1 - d, baseRate}
	}
}

// ExpectedProbability computes the projected probability: E[p] = b + a·u
func (o Opinion) ExpectedProbability() float64 {
	return o.Belief + o.BaseRate*o.Uncertainty
}

// TrustDiscount applies Type I trust discounting: we trust the source's opinion
// proportionally to our trust in them. Full trust → their opinion intact;
// zero trust → vacuous.
//
// Reference: Jøsang, "Subjective Logic" (Springer 2016), §14.2 Definition 14.2.
//
// Note: Type I discounting uses only trust.Belief. Sources with reliability
// ≤ 0.5 (where trust.Belief == 0 via FromReliability) produce vacuous results
// regardless of trust.Disbelief. This is a known limitation of Type I
// discounting — it models trust (positive confidence) but not distrust
// (active belief the source is lying). See Jøsang §14.2.2 for Type II
// discounting which addresses this.
func TrustDiscount(trust, claim Opinion) Opinion {
	// result.b = trust.b · claim.b
	// result.d = trust.b · claim.d
	// result.u = 1 − trust.b·(claim.b + claim.d)
	b := trust.Belief * claim.Belief
	d := trust.Belief * claim.Disbelief
	u := 1 - trust.Belief*(claim.Belief+claim.Disbelief)
	return Opinion{b, d, u, claim.BaseRate}
}

// Multiply computes the conjunction of two independent opinions: P(x AND y).
// The resulting opinion satisfies E[x∧y] = E[x]·E[y] with base rate
// a_{x∧y} = a_x·a_y, and maximises uncertainty subject to those constraints
// (maximum entropy / least commitment principle).
//
// Reference: Jøsang, "Subjective Logic" (Springer 2016), §14.3 Definition 14.1.
//
// This operator is for combining DIFFERENT propositions. To combine
// independent assessments of the SAME proposition, use ConsensusFuse.
func Multiply(x, y Opinion) Opinion {
	return opFromExpected(
		x.ExpectedProbability()*y.ExpectedProbability(),
		x.BaseRate*y.BaseRate,
	)
}

// CoMultiply computes the disjunction of two independent opinions: P(x OR y).
// The resulting opinion satisfies E[x∨y] = E[x]+E[y]−E[x]·E[y] with base
// rate a_{x∨y} = a_x+a_y−a_x·a_y, and maximises uncertainty.
//
// Reference: Jøsang, "Subjective Logic" (Springer 2016), §14.3 Definition 14.2.
func CoMultiply(x, y Opinion) Opinion {
	ex := x.ExpectedProbability()
	ey := y.ExpectedProbability()
	return opFromExpected(
		ex+ey-ex*ey,
		x.BaseRate+y.BaseRate-x.BaseRate*y.BaseRate,
	)
}

// opFromExpected constructs the maximum-uncertainty opinion consistent with a
// target expected probability and base rate.
func opFromExpected(ep, baseRate float64) Opinion {
	ep = clamp(ep, 0, 1)
	baseRate = clamp(baseRate, 0, 1)

	// Edge cases where base rate is at a boundary.
	if baseRate <= 0 {
		// E[p] = b, u unconstrained by base rate; maximise u → b = ep, u = 1-ep.
		return Opinion{ep, 1 - ep, 0, 0}
	}
	if baseRate >= 1 {
		return Opinion{0, 1 - ep, 0, 1}
	}

	// Maximise u subject to:
	//   b = ep − baseRate·u  ≥ 0  →  u ≤ ep/baseRate
	//   d = 1 − b − u        ≥ 0  →  u ≤ (1−ep)/(1−baseRate)
	uMax := ep / baseRate
	if alt := (1 - ep) / (1 - baseRate); alt < uMax {
		uMax = alt
	}
	if uMax > 1 {
		uMax = 1
	}

	b := ep - baseRate*uMax
	d := 1 - b - uMax
	// Clamp to absorb floating-point noise.
	return Opinion{clamp(b, 0, 1), clamp(d, 0, 1), clamp(uMax, 0, 1), baseRate}
}

// ConsensusFuse combines independent opinions using Cumulative Belief Fusion.
// Handles dogmatic edge cases by averaging.
func ConsensusFuse(opinions ...Opinion) Opinion {
	if len(opinions) == 0 {
		return Vacuous(0.5)
	}
	if len(opinions) == 1 {
		return opinions[0]
	}
	result := opinions[0]
	for i := 1; i < len(opinions); i++ {
		result = fuseTwo(result, opinions[i])
	}
	return result
}

// fuseTwo implements pairwise Cumulative Belief Fusion (CBF).
//
// Reference: Jøsang, Diaz & Rifqi, "Cumulative and Averaging Fusion of
// Beliefs", Information Fusion 11(2), 2010, Theorem 1 (Eq. 14–15).
// Base rate: confidence-weighted per Jøsang (2016) §12.6; when all opinions
// share the same base rate (the common case in this codebase) the formula
// degenerates to the shared value.
func fuseTwo(a, b Opinion) Opinion {
	// Both dogmatic (u=0): weighted average with γ_A=γ_B=0.5 (default).
	// Per Jøsang (2010) Eq. 15.
	if a.Uncertainty == 0 && b.Uncertainty == 0 {
		return Opinion{
			Belief:      (a.Belief + b.Belief) / 2,
			Disbelief:   (a.Disbelief + b.Disbelief) / 2,
			Uncertainty: 0,
			BaseRate:    (a.BaseRate + b.BaseRate) / 2,
		}
	}
	// One dogmatic: dogmatic opinion represents infinite evidence, dominates.
	if a.Uncertainty == 0 {
		return a
	}
	if b.Uncertainty == 0 {
		return b
	}
	// Normal case: CBF (Eq. 14).
	denom := a.Uncertainty + b.Uncertainty - a.Uncertainty*b.Uncertainty
	if math.Abs(denom) < 1e-12 {
		return Vacuous((a.BaseRate + b.BaseRate) / 2)
	}
	belief := (a.Belief*b.Uncertainty + b.Belief*a.Uncertainty) / denom
	disbelief := (a.Disbelief*b.Uncertainty + b.Disbelief*a.Uncertainty) / denom
	uncertainty := (a.Uncertainty * b.Uncertainty) / denom
	// Confidence-weighted base rate: each source's base rate is weighted by
	// its confidence (1−u). Per Jøsang (2016) §12.6.
	confA := 1 - a.Uncertainty
	confB := 1 - b.Uncertainty
	confSum := confA + confB
	var baseRate float64
	if confSum < 1e-12 {
		baseRate = (a.BaseRate + b.BaseRate) / 2
	} else {
		baseRate = (a.BaseRate*confA + b.BaseRate*confB) / confSum
	}
	return Opinion{belief, disbelief, uncertainty, baseRate}
}

// OpinionFromEvidence constructs an opinion from positive (r) and negative (s)
// evidence counts using the beta-binomial bijection.
// total = r + s + W; b = r/total, d = s/total, u = W/total.
// Negative r or s values are clamped to 0.
//
// Reference: Josang (2016) Ch. 3, Definition 3.2.
func OpinionFromEvidence(r, s, baseRate float64) Opinion {
	if r < 0 {
		r = 0
	}
	if s < 0 {
		s = 0
	}
	total := r + s + W
	return Opinion{
		Belief:      r / total,
		Disbelief:   s / total,
		Uncertainty: W / total,
		BaseRate:    baseRate,
	}
}

// EvidenceCounts returns the positive (r) and negative (s) evidence counts
// implied by an opinion, inverting the beta-binomial bijection.
// r = W * b / u, s = W * d / u.
// For dogmatic opinions (u=0), returns (+Inf, 0) or (0, +Inf) depending
// on whether belief or disbelief dominates.
//
// Reference: Josang (2016) Ch. 3, inverse bijection.
func EvidenceCounts(o Opinion) (r, s float64) {
	if o.Uncertainty == 0 {
		if o.Belief >= o.Disbelief {
			return math.Inf(1), 0
		}
		return 0, math.Inf(1)
	}
	return W * o.Belief / o.Uncertainty, W * o.Disbelief / o.Uncertainty
}

// AveragingFuse combines dependent opinions using Averaging Belief Fusion (ABF).
// ABF is idempotent: fusing an opinion with itself returns the same opinion.
// Use ABF for dependent sources (e.g. same actor repeating a claim).
//
// Reference: Josang, Diaz & Rifqi (2010) §4, Eq. 16.
func AveragingFuse(opinions ...Opinion) Opinion {
	if len(opinions) == 0 {
		return Vacuous(0.5)
	}
	if len(opinions) == 1 {
		return opinions[0]
	}
	result := opinions[0]
	for i := 1; i < len(opinions); i++ {
		result = abfTwo(result, opinions[i])
	}
	return result
}

// abfTwo implements pairwise Averaging Belief Fusion.
//
// Reference: Josang, Diaz & Rifqi (2010) §4.
func abfTwo(a, b Opinion) Opinion {
	// Both dogmatic: weighted average with equal weights.
	if a.Uncertainty == 0 && b.Uncertainty == 0 {
		return Opinion{
			Belief:      (a.Belief + b.Belief) / 2,
			Disbelief:   (a.Disbelief + b.Disbelief) / 2,
			Uncertainty: 0,
			BaseRate:    (a.BaseRate + b.BaseRate) / 2,
		}
	}
	// One dogmatic: dominates (infinite evidence).
	if a.Uncertainty == 0 {
		return a
	}
	if b.Uncertainty == 0 {
		return b
	}

	// Normal case: ABF (Eq. 16).
	// K_a = u_b, K_b = u_a (for n=2, K_k = product of u_j for j != k)
	// denom = u_a + u_b
	denom := a.Uncertainty + b.Uncertainty
	if math.Abs(denom) < 1e-12 {
		return Vacuous((a.BaseRate + b.BaseRate) / 2)
	}

	belief := (a.Belief*b.Uncertainty + b.Belief*a.Uncertainty) / denom
	disbelief := (a.Disbelief*b.Uncertainty + b.Disbelief*a.Uncertainty) / denom
	uncertainty := 2 * a.Uncertainty * b.Uncertainty / denom

	// Confidence-weighted base rate.
	confA := 1 - a.Uncertainty
	confB := 1 - b.Uncertainty
	confSum := confA + confB
	var baseRate float64
	if confSum < 1e-12 {
		baseRate = (a.BaseRate + b.BaseRate) / 2
	} else {
		baseRate = (a.BaseRate*confA + b.BaseRate*confB) / confSum
	}
	return Opinion{belief, disbelief, uncertainty, baseRate}
}

// Negate swaps belief↔disbelief and flips the base rate.
func Negate(op Opinion) Opinion {
	return Opinion{
		Belief:      op.Disbelief,
		Disbelief:   op.Belief,
		Uncertainty: op.Uncertainty,
		BaseRate:    1 - op.BaseRate,
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
