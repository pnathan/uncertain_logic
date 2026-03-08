package belnap

// Value represents a Belnap four-valued logic value.
// The four values form two lattices:
//
//	Truth lattice:     F ≤ {N,B} ≤ T  (N and B incomparable)
//	Knowledge lattice: N ≤ {T,F} ≤ B
type Value int

const (
	Neither Value = iota // N: no evidence either way
	True                 // T: supported, not refuted
	False                // F: refuted, not supported
	Both                 // B: contradicted — evidence for AND against
)

func (v Value) String() string {
	switch v {
	case Neither:
		return "N"
	case True:
		return "T"
	case False:
		return "F"
	case Both:
		return "B"
	default:
		return "?"
	}
}

// Not swaps T↔F, preserves B and N.
func Not(v Value) Value {
	switch v {
	case True:
		return False
	case False:
		return True
	default:
		return v
	}
}

// And is the meet in the truth lattice.
// Truth lattice order: F ≤ N, F ≤ B, N ≤ T, B ≤ T, N and B incomparable.
// So N∧B = F (greatest lower bound of two incomparable elements above F).
func And(a, b Value) Value {
	if a == b {
		return a
	}
	// F dominates in And
	if a == False || b == False {
		return False
	}
	// T is identity for And
	if a == True {
		return b
	}
	if b == True {
		return a
	}
	// N and B are incomparable; their meet is F
	return False
}

// Or is the join in the truth lattice.
// N∨B = T (least upper bound of two incomparable elements below T).
func Or(a, b Value) Value {
	if a == b {
		return a
	}
	// T dominates in Or
	if a == True || b == True {
		return True
	}
	// F is identity for Or
	if a == False {
		return b
	}
	if b == False {
		return a
	}
	// N and B are incomparable; their join is T
	return True
}

// EvidenceJoin is the join in the knowledge lattice.
// Knowledge lattice order: N ≤ T, N ≤ F, T ≤ B, F ≤ B.
// Accumulating contradictory evidence (T⊔F = B) yields Both.
func EvidenceJoin(a, b Value) Value {
	if a == b {
		return a
	}
	// B absorbs all
	if a == Both || b == Both {
		return Both
	}
	// N is identity in knowledge lattice (no info)
	if a == Neither {
		return b
	}
	if b == Neither {
		return a
	}
	// T and F together → Both (contradiction)
	return Both
}

// FromCounts derives a Belnap value from raw evidence counts.
func FromCounts(supporting, refuting int) Value {
	switch {
	case supporting > 0 && refuting > 0:
		return Both
	case supporting > 0:
		return True
	case refuting > 0:
		return False
	default:
		return Neither
	}
}
