package subjective

import "math"

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

// TrustDiscount applies trust discounting: we trust the source's opinion
// proportionally to our trust in them. Full trust → their opinion intact;
// zero trust → vacuous.
func TrustDiscount(trust, claim Opinion) Opinion {
	// result.b = trust.b · claim.b
	// result.d = trust.b · claim.d
	// result.u = 1 − trust.b·(claim.b + claim.d)
	b := trust.Belief * claim.Belief
	d := trust.Belief * claim.Disbelief
	u := 1 - trust.Belief*(claim.Belief+claim.Disbelief)
	return Opinion{b, d, u, claim.BaseRate}
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

func fuseTwo(a, b Opinion) Opinion {
	// Both dogmatic: average
	if a.Uncertainty == 0 && b.Uncertainty == 0 {
		return Opinion{
			Belief:      (a.Belief + b.Belief) / 2,
			Disbelief:   (a.Disbelief + b.Disbelief) / 2,
			Uncertainty: 0,
			BaseRate:    (a.BaseRate + b.BaseRate) / 2,
		}
	}
	// One dogmatic: dogmatic wins
	if a.Uncertainty == 0 {
		return a
	}
	if b.Uncertainty == 0 {
		return b
	}
	// Normal case: CBF
	denom := a.Uncertainty + b.Uncertainty - a.Uncertainty*b.Uncertainty
	if math.Abs(denom) < 1e-12 {
		return Vacuous((a.BaseRate + b.BaseRate) / 2)
	}
	belief := (a.Belief*b.Uncertainty + b.Belief*a.Uncertainty) / denom
	disbelief := (a.Disbelief*b.Uncertainty + b.Disbelief*a.Uncertainty) / denom
	uncertainty := (a.Uncertainty * b.Uncertainty) / denom
	baseRate := (a.BaseRate + b.BaseRate) / 2
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
