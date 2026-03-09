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

func TestActorClone(t *testing.T) {
	orig := Actor{
		ID: "a1", Name: "Analyst", SourceType: Analyst, BaseReliability: 0.8,
		Conflicts: []ConflictOfInterest{
			{Description: "long position", Direction: Long, Disclosed: true, SubjectID: "s1"},
		},
		Notes: "original",
	}
	clone := orig.Clone()

	// Mutate clone — original must be unaffected
	clone.Name = "Mutated"
	clone.BaseReliability = 0.1
	clone.Conflicts[0].Description = "mutated"
	clone.Conflicts = append(clone.Conflicts, ConflictOfInterest{Description: "new"})
	clone.Notes = "cloned"

	if orig.Name != "Analyst" {
		t.Errorf("clone mutated original Name")
	}
	if orig.BaseReliability != 0.8 {
		t.Errorf("clone mutated original BaseReliability")
	}
	if orig.Conflicts[0].Description != "long position" {
		t.Errorf("clone mutated original Conflicts[0]")
	}
	if len(orig.Conflicts) != 1 {
		t.Errorf("clone append affected original Conflicts len: got %d", len(orig.Conflicts))
	}
	if orig.Notes != "original" {
		t.Errorf("clone mutated original Notes")
	}

	// Actor with nil Conflicts
	bare := Actor{ID: "bare", BaseReliability: 0.5}
	bareClone := bare.Clone()
	if bareClone.ID != "bare" || bareClone.BaseReliability != 0.5 {
		t.Errorf("bare clone mismatch: %+v", bareClone)
	}
}

func TestSubjectClone(t *testing.T) {
	orig := Subject{ID: "s1", Name: "Corp", SubjectType: "company", Notes: "original"}
	clone := orig.Clone()
	clone.Name = "Mutated"
	if orig.Name != "Corp" {
		t.Errorf("clone mutated original Name")
	}
}

func TestEvidenceClone(t *testing.T) {
	w := 0.9
	orig := Evidence{
		ID: "e1", ClaimID: "c1", Content: "doc", Valence: Supports,
		Weight: &w, Notes: "original",
	}
	clone := orig.Clone()

	// Mutate clone weight pointer — original must be unaffected
	*clone.Weight = 0.1
	clone.Notes = "cloned"

	if *orig.Weight != 0.9 {
		t.Errorf("clone mutated original Weight: got %f", *orig.Weight)
	}
	if orig.Notes != "original" {
		t.Errorf("clone mutated original Notes")
	}

	// Evidence with nil Weight
	noWeight := Evidence{ID: "e2", Valence: Neutral}
	nwClone := noWeight.Clone()
	if nwClone.Weight != nil {
		t.Error("clone of nil-weight evidence should have nil Weight")
	}
}

func TestClaimClone(t *testing.T) {
	orig := Claim{
		ID: "c1", ActorID: "a1", SubjectID: "s1",
		Predicate: "revenue", Value: "100M", Content: "claim text",
		EvidenceIDs: []string{"e1", "e2"},
		Notes:       "original",
	}
	clone := orig.Clone()

	// Mutate clone — original must be unaffected
	clone.EvidenceIDs[0] = "mutated"
	clone.EvidenceIDs = append(clone.EvidenceIDs, "e3")
	clone.Notes = "cloned"

	if orig.EvidenceIDs[0] != "e1" {
		t.Errorf("clone mutated original EvidenceIDs[0]")
	}
	if len(orig.EvidenceIDs) != 2 {
		t.Errorf("clone append affected original EvidenceIDs len: got %d", len(orig.EvidenceIDs))
	}
	if orig.Notes != "original" {
		t.Errorf("clone mutated original Notes")
	}

	// Claim with nil EvidenceIDs
	bare := Claim{ID: "c2", Predicate: "p"}
	bareClone := bare.Clone()
	if bareClone.ID != "c2" || bareClone.Predicate != "p" {
		t.Errorf("bare clone mismatch: %+v", bareClone)
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
