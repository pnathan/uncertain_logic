package models

import "testing"

func TestSourceTypeString(t *testing.T) {
	cases := []struct {
		v    SourceType
		want string
	}{
		{Analyst, "Analyst"},
		{Journalist, "Journalist"},
		{Expert, "Expert"},
		{Insider, "Insider"},
		{Regulator, "Regulator"},
		{Institutional, "Institutional"},
		{Anonymous, "Anonymous"},
		{SocialMedia, "SocialMedia"},
		{Troll, "Troll"},
		{SourceType(99), "Unknown"},
	}
	for _, c := range cases {
		if got := c.v.String(); got != c.want {
			t.Errorf("SourceType(%d).String() = %q, want %q", int(c.v), got, c.want)
		}
	}
}

func TestClaimTypeString(t *testing.T) {
	cases := []struct {
		v    ClaimType
		want string
	}{
		{Factual, "Factual"},
		{Predictive, "Predictive"},
		{Evaluative, "Evaluative"},
		{Causal, "Causal"},
		{Attribution, "Attribution"},
		{ClaimType(99), "Unknown"},
	}
	for _, c := range cases {
		if got := c.v.String(); got != c.want {
			t.Errorf("ClaimType(%d).String() = %q, want %q", int(c.v), got, c.want)
		}
	}
}

func TestValenceString(t *testing.T) {
	cases := []struct {
		v    Valence
		want string
	}{
		{Supports, "Supports"},
		{Refutes, "Refutes"},
		{Neutral, "Neutral"},
		{Valence(99), "Unknown"},
	}
	for _, c := range cases {
		if got := c.v.String(); got != c.want {
			t.Errorf("Valence(%d).String() = %q, want %q", int(c.v), got, c.want)
		}
	}
}

func TestTrustPenalty(t *testing.T) {
	if p := (ConflictOfInterest{Disclosed: true}).TrustPenalty(); p != 0.80 {
		t.Errorf("disclosed penalty = %v, want 0.80", p)
	}
	if p := (ConflictOfInterest{Disclosed: false}).TrustPenalty(); p != 0.50 {
		t.Errorf("undisclosed penalty = %v, want 0.50", p)
	}
}

func TestAdjustedReliability(t *testing.T) {
	// r > 1 clamps to 1
	high := &Actor{BaseReliability: 2.0}
	if r := high.AdjustedReliability("x"); r != 1.0 {
		t.Errorf("r>1 should clamp to 1, got %v", r)
	}

	// r < 0 clamps to 0
	neg := &Actor{BaseReliability: -1.0}
	if r := neg.AdjustedReliability("x"); r != 0.0 {
		t.Errorf("r<0 should clamp to 0, got %v", r)
	}

	// Conflict scoped to a different subject: penalty does not apply
	a := &Actor{
		BaseReliability: 0.8,
		Conflicts: []ConflictOfInterest{
			{SubjectID: "other_subject", Disclosed: false},
		},
	}
	if r := a.AdjustedReliability("my_subject"); r != 0.8 {
		t.Errorf("unscoped conflict should not reduce reliability, got %v", r)
	}

	// Conflict scoped to matching subject: penalty applies
	a2 := &Actor{
		BaseReliability: 0.8,
		Conflicts: []ConflictOfInterest{
			{SubjectID: "my_subject", Disclosed: false},
		},
	}
	if r := a2.AdjustedReliability("my_subject"); r >= 0.8 {
		t.Errorf("scoped conflict should reduce reliability, got %v", r)
	}
}

func TestToProposition(t *testing.T) {
	// SubjectID set
	c := &Claim{SubjectID: "company", Predicate: "revenue", Value: "100M"}
	p := c.ToProposition()
	if p.Subject != "company" || p.Predicate != "revenue" || p.Value != "100M" {
		t.Errorf("ToProposition = %+v", p)
	}

	// TargetClaimID set (meta-claim): Subject should be the target claim ID
	meta := &Claim{TargetClaimID: "claim_x", Predicate: "accuracy", Value: "false"}
	mp := meta.ToProposition()
	if mp.Subject != "claim_x" {
		t.Errorf("meta-claim ToProposition.Subject = %q, want claim_x", mp.Subject)
	}
}

func TestEffectiveWeight(t *testing.T) {
	// nil weight → default 0.6
	e := &Evidence{}
	if w := e.EffectiveWeight(); w != 0.6 {
		t.Errorf("nil weight should default to 0.6, got %v", w)
	}

	// explicit weight
	w := 0.9
	e2 := &Evidence{Weight: &w}
	if got := e2.EffectiveWeight(); got != 0.9 {
		t.Errorf("explicit weight should be 0.9, got %v", got)
	}
}
