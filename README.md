# uncertain_logic

An AGPL3 Go library for reasoning about claims made by sources that may lie, be ignorant, or contradict each other — without the reasoning system exploding.

The library composes three formal systems:

- **Belnap four-valued logic** — truth status under contradiction
- **Subjective logic (Jøsang)** — graded credibility with trust propagation
- **Allen interval algebra** — temporal reasoning over event intervals

Together they answer the five investigative questions about any claim:

1. **What** does someone claim?
2. **When** did they claim it?
3. **What** are they claiming it about?
4. **When** did they claim it happened?
5. **How much** do we believe them, and why?

---

## Package layout

```
uncertain_logic/
  belnap/          Four-valued logic values and operations
  subjective/      Opinion arithmetic and trust discounting
  temporal/        EventInterval and Allen's 13 relations
  models/          Actor, Subject, Claim, Evidence, Proposition
  investigation/   Investigation builder, Assert/Query DSL, analysis
```

---

## Quick start

```go
import (
    "uncertain_logic/investigation"
    "uncertain_logic/models"
    "uncertain_logic/temporal"
)

inv := investigation.New("Is the $5B cost estimate credible?")

inv.AddActor("senator", "Senator X", models.Expert,
    investigation.WithReliability(0.7))
inv.AddActor("cbo", "CBO", models.Institutional,
    investigation.WithReliability(0.95))
inv.AddActor("lobbyist", "Industry Lobbyist", models.Analyst,
    investigation.WithReliability(0.6),
    investigation.WithConflict(models.ConflictOfInterest{
        Description: "Client benefits from bill passage",
        Direction:   models.Long,
        Disclosed:   true,
    }),
)
inv.AddSubject("bill42", "Bill S.42", "legislation")

iv := temporal.EventInterval{...} // the period the claim is about

claimID := inv.AssertClaim("senator",
    investigation.Prop("bill42", "cost", "5B"),
    assertionTime, iv)

// Attach evidence
inv.AddEvidence(claimID, "CBO score shows $4.8B", models.Supports,
    investigation.WithWeight(0.85))
inv.AddEvidence(claimID, "Treasury model shows $7.2B", models.Refutes,
    investigation.WithWeight(0.75))

// Analyze
a, _ := inv.AnalyzeClaim(claimID)
fmt.Println(a.FiveQuestions())
// Q1 (What):  bill42 cost = 5B
// Q2 (When claimed): Senator X claimed at 2024-02-01
// Q3 (About):  bill42
// Q4 (Event interval): 2024-01-01 to 2024-12-31
// Q5 (Credibility): Belnap=B, E[p]=0.512 (b=0.340 d=0.380 u=0.280)
//    Evidence: 1 supporting, 1 refuting, 0 neutral
//    Actor reliability: 0.700 (base=0.700, after conflicts)
```

---

## Core concepts

### Claims and subjects

Every claim is a **triple**: `(subject, predicate, value)` — wrapped in a `Proposition`.

```go
prop := investigation.Prop("company_x", "revenue", "50M")
```

Attach it to an actor assertion with `AssertClaim`, or record it as ground truth with `AssertFact`.

### Meta-claims (claims about claims)

A claim can target another claim rather than a subject entity. This is how you model attribution chains — "journalist says politician said X" — without asserting X is true.

```go
// Senator's direct claim
senClaimID := inv.AssertClaim("senator", investigation.Prop("bill42", "cost", "5B"), t1, iv)

// Journalist attributes it (supports the claim as real, not the cost figure)
inv.AssertMetaClaim("journalist", senClaimID, "authorship", "stated", t2,
    investigation.WithValence(models.Supports))

// Fact-checker disputes accuracy
inv.AssertMetaClaim("factchecker", senClaimID, "accuracy", "false", t3,
    investigation.WithValence(models.Refutes))
```

Meta-claims feed into Dung-style attack/support: the fact-checker's refutation adds refuting evidence to the senator's claim when credibility is computed. Recursion is depth-limited (default: 3) so cycles in political discourse terminate cleanly.

### Belnap four-valued logic

Claim status is one of four values:

| Value | Meaning |
|-------|---------|
| `N` (Neither) | No evidence either way |
| `T` (True) | Supported, not refuted |
| `F` (False) | Refuted, not supported |
| `B` (Both) | Contradicted — evidence for AND against |

`B` is stable under contradiction. The system does not explode when two sources disagree.

**Critical**: `B` only applies to claims whose **event intervals overlap**. A writer who calls GWB "moderate" (1985–1992) and "conservative" (1993–2001) is not contradicting themselves — those intervals don't overlap, so `Overlapping()` returns false and each claim can independently be `T`.

```go
// Truth operations (for compound claims)
belnap.And(belnap.True, belnap.Both)   // Both
belnap.Or(belnap.Neither, belnap.Both) // True (N∨B = T in truth lattice)
belnap.Not(belnap.True)                // False

// Knowledge accumulation (for gathering evidence)
belnap.EvidenceJoin(belnap.True, belnap.False) // Both — contradiction
belnap.EvidenceJoin(belnap.Neither, belnap.True) // True — new info
```

### Subjective logic opinions

A credibility opinion is `(belief, disbelief, uncertainty, base_rate)` with `b+d+u=1`.

```go
// Construct
op := subjective.FromReliability(0.75, 0.5) // r=0.75 → b=0.5, d=0, u=0.5
op.ExpectedProbability()                      // b + a*u = 0.5 + 0.5*0.5 = 0.75

// Trust discounting: discount a claim opinion by our trust in the source
trusted   := subjective.FromReliability(0.9, 0.5)
claimOp   := subjective.DogmaticTrue(0.5)
discounted := subjective.TrustDiscount(trusted, claimOp)
// High trust → opinion mostly intact; zero trust → vacuous (0,0,1)

// Fuse independent opinions (reduces uncertainty)
fused := subjective.ConsensusFuse(op1, op2, op3)

// Negate (swap belief↔disbelief)
negated := subjective.Negate(op)
```

### Temporal reasoning

Claims are about **intervals**, not instants. Allen's 13 relations determine whether two claim intervals overlap.

```go
a := temporal.EventInterval{Start: &t1, End: &t2}
b := temporal.EventInterval{Start: &t3, End: &t4}

temporal.Relate(a, b)      // AllenRelation (Precedes, Overlaps, Contains, ...)
temporal.Overlapping(a, b) // true unless Precedes or PrecededBy

// Open (unspecified) intervals are conservative: overlap with everything
open := temporal.Open("around the IPO")
temporal.Overlapping(open, a) // always true
```

### Actors and conflicts of interest

```go
inv.AddActor("analyst", "Bull Analyst", models.Analyst,
    investigation.WithReliability(0.7),
    investigation.WithConflict(models.ConflictOfInterest{
        Description: "Long position",
        Direction:   models.Long,
        Disclosed:   false,           // undisclosed → 0.50 penalty
        SubjectID:   "stockx",        // scoped to this subject
    }),
)
// AdjustedReliability("stockx") = 0.7 * 0.50 = 0.35
```

Conflict penalties: disclosed → ×0.80, undisclosed → ×0.50. Multiple conflicts multiply.

### Querying

```go
// Q returns one QueryResult per temporal cluster of claims
results := inv.Q("bill42", "cost", iv)
for _, r := range results {
    fmt.Println(r.Belnap, r.Opinion.ExpectedProbability())
}

// Combine results with logical operations
r1 := inv.Q("bill42", "cost", iv)[0]
r2 := inv.Q("bill42", "feasibility", iv)[0]
both := investigation.And(r1, r2)
```

### Timeline analysis

```go
// SubjectTimeline: how has the narrative evolved over time?
// Claims in the same TimeSlice have overlapping EventIntervals → Belnap candidates.
// Claims in different slices signal a fluent change, not a contradiction.
timeline := inv.SubjectTimeline("gwb")
for _, slice := range timeline {
    fmt.Printf("Period %v–%v: %d claims\n",
        slice.Interval.Start, slice.Interval.End, len(slice.Claims))
}

// ActorBeliefHistory: did an actor revise their position?
claims, revisions := inv.ActorBeliefHistory("writer", "gwb")
for _, rev := range revisions {
    if rev.TemporallyConsistent {
        // Actor updated view as subject changed — not a contradiction
    } else {
        // Actor reversed position about the same period — genuine reversal
    }
}
```

---

## Actors reference

| SourceType | Default use |
|-----------|-------------|
| `Analyst` | Financial/market analysts |
| `Journalist` | Reporters; use meta-claims for attribution |
| `Expert` | Domain specialists |
| `Insider` | Employees, whistleblowers |
| `Regulator` | Government agencies (SEC, FDA…) |
| `Institutional` | Companies, standards bodies |
| `Anonymous` | Unknown sourcing |
| `SocialMedia` | Reddit, Twitter, forums |
| `Troll` | Known bad actors |

Default reliability when not specified: **0.6**. `_system` actor (used by `AssertFact`) has reliability **1.0**.

---

## Design notes

**Why not Bayesian P(A|B)?** Bayesian reasoning degrades on genuinely rare, high-stakes events (lottery paradox). Belnap's `B` value is stable — contradicted claims stay `B` rather than collapsing to a probability that loses the contradiction signal.

**Why not just use databases?** Logical evaluation (Belnap aggregation, subjective fusion, meta-claim recursion) happens in Go, not SQL. Storage is the `map[string]*Claim` in `Investigation`; the interface is stable if you swap it for SQLite later.

**Composability**: `belnap`, `subjective`, and `temporal` are pure logic packages with no dependencies on each other. `models` depends only on `temporal`. `investigation` is the only package that wires them together.
