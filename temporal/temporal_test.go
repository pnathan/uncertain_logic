package temporal

import (
	"testing"
	"time"
)

func tp(year, month, day int) *time.Time {
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &t
}

func interval(y1, m1, d1, y2, m2, d2 int) EventInterval {
	return EventInterval{Start: tp(y1, m1, d1), End: tp(y2, m2, d2)}
}

func TestRelate(t *testing.T) {
	a := interval(2020, 1, 1, 2020, 6, 30)
	b := interval(2020, 7, 1, 2020, 12, 31)

	cases := []struct {
		name string
		a, b EventInterval
		want AllenRelation
	}{
		{"precedes", a, b, Precedes},
		{"preceded by", b, a, PrecededBy},
		{"meets", interval(2020, 1, 1, 2020, 6, 30), interval(2020, 6, 30, 2020, 12, 31), Meets},
		{"met by", interval(2020, 6, 30, 2020, 12, 31), interval(2020, 1, 1, 2020, 6, 30), MetBy},
		{"overlaps", interval(2020, 1, 1, 2020, 8, 1), interval(2020, 6, 1, 2020, 12, 31), Overlaps},
		{"overlapped by", interval(2020, 6, 1, 2020, 12, 31), interval(2020, 1, 1, 2020, 8, 1), OverlappedBy},
		{"equals", interval(2020, 1, 1, 2020, 12, 31), interval(2020, 1, 1, 2020, 12, 31), Equals},
		{"during", interval(2020, 3, 1, 2020, 9, 1), interval(2020, 1, 1, 2020, 12, 31), During},
		{"contains", interval(2020, 1, 1, 2020, 12, 31), interval(2020, 3, 1, 2020, 9, 1), Contains},
		{"starts", interval(2020, 1, 1, 2020, 6, 1), interval(2020, 1, 1, 2020, 12, 31), Starts},
		{"started by", interval(2020, 1, 1, 2020, 12, 31), interval(2020, 1, 1, 2020, 6, 1), StartedBy},
		{"finishes", interval(2020, 6, 1, 2020, 12, 31), interval(2020, 1, 1, 2020, 12, 31), Finishes},
		{"finished by", interval(2020, 1, 1, 2020, 12, 31), interval(2020, 6, 1, 2020, 12, 31), FinishedBy},
	}
	for _, c := range cases {
		got := Relate(c.a, c.b)
		if got != c.want {
			t.Errorf("%s: Relate = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOverlapping(t *testing.T) {
	cases := []struct {
		name string
		a, b EventInterval
		want bool
	}{
		{"disjoint past/future", interval(2020, 1, 1, 2020, 6, 30), interval(2020, 7, 1, 2020, 12, 31), false},
		{"disjoint future/past", interval(2020, 7, 1, 2020, 12, 31), interval(2020, 1, 1, 2020, 6, 30), false},
		{"overlapping", interval(2020, 1, 1, 2020, 8, 1), interval(2020, 6, 1, 2020, 12, 31), true},
		{"equal", interval(2020, 1, 1, 2020, 12, 31), interval(2020, 1, 1, 2020, 12, 31), true},
		{"contains", interval(2020, 1, 1, 2020, 12, 31), interval(2020, 3, 1, 2020, 9, 1), true},
		{"meets", interval(2020, 1, 1, 2020, 6, 30), interval(2020, 6, 30, 2020, 12, 31), true},
		{"met by", interval(2020, 6, 30, 2020, 12, 31), interval(2020, 1, 1, 2020, 6, 30), true},
	}
	for _, c := range cases {
		got := Overlapping(c.a, c.b)
		if got != c.want {
			t.Errorf("%s: Overlapping = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOpenIntervalConservative(t *testing.T) {
	open := Open("around the IPO")
	specific := interval(2020, 1, 1, 2020, 12, 31)
	// Open intervals should be treated as overlapping (conservative)
	if !Overlapping(open, specific) {
		t.Error("open interval should overlap with specific interval")
	}
	if !Overlapping(specific, open) {
		t.Error("specific interval should overlap with open interval")
	}
	if !Overlapping(open, open) {
		t.Error("two open intervals should overlap")
	}
}

func TestPointInTime(t *testing.T) {
	pt := PointInTime(time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC))
	if pt.Start == nil || pt.End == nil {
		t.Error("PointInTime should set both Start and End")
	}
	if !pt.Start.Equal(*pt.End) {
		t.Error("PointInTime Start should equal End")
	}
}

// TestPointIntervalEquality: two identical point intervals must classify as Equals,
// not Meets. Allen's framework assumes non-degenerate intervals (Start < End);
// Relate now checks Equals before boundary-contact to handle the degenerate case.
func TestPointIntervalEquality(t *testing.T) {
	pt := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)
	a := PointInTime(pt)
	b := PointInTime(pt)
	if got := Relate(a, b); got != Equals {
		t.Errorf("Relate(point, same point) = %v, want Equals", got)
	}
}

// TestPointIntervalVsDisjoint: a point interval before a proper interval → Precedes.
func TestPointIntervalVsDisjoint(t *testing.T) {
	pt := PointInTime(time.Date(2019, 6, 15, 0, 0, 0, 0, time.UTC))
	iv := interval(2020, 1, 1, 2020, 12, 31)
	if got := Relate(pt, iv); got != Precedes {
		t.Errorf("Relate(point-before, interval) = %v, want Precedes", got)
	}
	if Overlapping(pt, iv) {
		t.Error("point before interval should not overlap")
	}
}

func TestClone(t *testing.T) {
	// Concrete interval: clone must be independent
	orig := interval(2020, 1, 1, 2020, 12, 31)
	orig.Desc = "original"
	clone := orig.Clone()

	// Mutate clone's pointers — original must be unaffected
	*clone.Start = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
	*clone.End = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	clone.Desc = "cloned"

	if orig.Start.Year() != 2020 {
		t.Errorf("clone mutated original Start: got year %d", orig.Start.Year())
	}
	if orig.End.Year() != 2020 {
		t.Errorf("clone mutated original End: got year %d", orig.End.Year())
	}
	if orig.Desc != "original" {
		t.Errorf("clone mutated original Desc: got %q", orig.Desc)
	}

	// Open interval (nil pointers): clone must not panic
	open := Open("fuzzy")
	openClone := open.Clone()
	if openClone.Start != nil || openClone.End != nil {
		t.Error("clone of open interval should have nil Start/End")
	}
	if openClone.Desc != "fuzzy" {
		t.Errorf("clone Desc = %q, want fuzzy", openClone.Desc)
	}

	// Half-open: only Start set
	halfStart := EventInterval{Start: tp(2020, 6, 1)}
	hsClone := halfStart.Clone()
	*hsClone.Start = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
	if halfStart.Start.Year() != 2020 {
		t.Errorf("half-open clone mutated original Start")
	}
	if hsClone.End != nil {
		t.Error("half-open clone should have nil End")
	}
}

func TestGWBScenario(t *testing.T) {
	// GWB scenario: two editorials about different presidential terms
	term1 := interval(1985, 1, 1, 1993, 1, 1) // pre-presidency / first term window
	term2 := interval(1993, 1, 1, 2001, 1, 1) // second term window

	// These are sequential terms that share a boundary (meets/met by)
	// They should be Overlapping = true (boundary touch), but let's check:
	// Actually Meets means aE == bS, which returns Meets (overlapping)
	// For the GWB test we want clearly non-overlapping: use different dates
	term1b := interval(1985, 1, 1, 1992, 12, 31)
	term2b := interval(1993, 1, 1, 2001, 1, 1)

	if Overlapping(term1b, term2b) {
		t.Error("GWB scenario: sequential non-overlapping terms should NOT overlap")
	}
	_ = term1
	_ = term2
}
