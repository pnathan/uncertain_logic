package belnap

import (
	"testing"
)

func TestString(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{Neither, "N"},
		{True, "T"},
		{False, "F"},
		{Both, "B"},
		{Value(99), "?"},
	}
	for _, c := range cases {
		if got := c.v.String(); got != c.want {
			t.Errorf("Value(%d).String() = %q, want %q", int(c.v), got, c.want)
		}
	}
}

func TestNot(t *testing.T) {
	cases := []struct{ in, want Value }{
		{True, False},
		{False, True},
		{Both, Both},
		{Neither, Neither},
	}
	for _, c := range cases {
		if got := Not(c.in); got != c.want {
			t.Errorf("Not(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAnd(t *testing.T) {
	// Truth lattice: F ≤ {N,B} ≤ T; N and B incomparable → N∧B = F
	cases := []struct{ a, b, want Value }{
		{True, True, True},
		{True, False, False},
		{True, Neither, Neither},
		{True, Both, Both},
		{False, True, False},
		{False, False, False},
		{False, Neither, False},
		{False, Both, False},
		{Neither, True, Neither},
		{Neither, False, False},
		{Neither, Neither, Neither},
		{Neither, Both, False},
		{Both, True, Both},
		{Both, False, False},
		{Both, Neither, False},
		{Both, Both, Both},
	}
	for _, c := range cases {
		if got := And(c.a, c.b); got != c.want {
			t.Errorf("And(%v,%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestOr(t *testing.T) {
	// Truth lattice: F ≤ {N,B} ≤ T; N and B incomparable → N∨B = T
	cases := []struct{ a, b, want Value }{
		{True, True, True},
		{True, False, True},
		{True, Neither, True},
		{True, Both, True},
		{False, True, True},
		{False, False, False},
		{False, Neither, Neither},
		{False, Both, Both},
		{Neither, True, True},
		{Neither, False, Neither},
		{Neither, Neither, Neither},
		{Neither, Both, True},
		{Both, True, True},
		{Both, False, Both},
		{Both, Neither, True},
		{Both, Both, Both},
	}
	for _, c := range cases {
		if got := Or(c.a, c.b); got != c.want {
			t.Errorf("Or(%v,%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestEvidenceJoin(t *testing.T) {
	// Knowledge lattice: N ≤ {T,F} ≤ B
	cases := []struct{ a, b, want Value }{
		{Neither, Neither, Neither},
		{Neither, True, True},
		{Neither, False, False},
		{Neither, Both, Both},
		{True, Neither, True},
		{True, True, True},
		{True, False, Both},
		{True, Both, Both},
		{False, Neither, False},
		{False, True, Both},
		{False, False, False},
		{False, Both, Both},
		{Both, Neither, Both},
		{Both, True, Both},
		{Both, False, Both},
		{Both, Both, Both},
	}
	for _, c := range cases {
		if got := EvidenceJoin(c.a, c.b); got != c.want {
			t.Errorf("EvidenceJoin(%v,%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestFromCounts(t *testing.T) {
	cases := []struct {
		s, r int
		want Value
	}{
		{0, 0, Neither},
		{1, 0, True},
		{0, 1, False},
		{1, 1, Both},
		{3, 0, True},
		{0, 3, False},
		{2, 2, Both},
	}
	for _, c := range cases {
		if got := FromCounts(c.s, c.r); got != c.want {
			t.Errorf("FromCounts(%d,%d) = %v, want %v", c.s, c.r, got, c.want)
		}
	}
}
