# uncertain_logic

An AGPL3 Go library for reasoning about claims made by sources that may lie, be ignorant, or contradict each other — without the reasoning system exploding.

The library composes four formal systems:

- **Belnap four-valued logic** — truth status under contradiction
- **Subjective logic (Jøsang)** — graded credibility with trust propagation
- **Allen interval algebra** — temporal reasoning over event intervals
- **Dung argumentation frameworks** — defensible reasoning under attack, extended with bipolar support (Cayrol & Lagasquie-Schiex 2005) for causal chain analysis

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
  argumentation/   Dung frameworks, Grounded/Preferred/Stable semantics, causal chains
  models/          Actor, Subject, Claim, Evidence, Proposition
  investigation/   Investigation builder, Assert/Query DSL, analysis
  viewer/          HTMX web viewer for argumentation frameworks
  cmd/wmd-demo/    Iraq WMD intelligence failure demonstration
```

---

## Quick start

```go
package main

import (
    "fmt"
    "time"

    "github.com/pnathan/uncertain_logic/investigation"
    "github.com/pnathan/uncertain_logic/models"
    "github.com/pnathan/uncertain_logic/temporal"
)

func main() {
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

    // The time period the claim covers
    start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
    end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
    iv := temporal.EventInterval{Start: &start, End: &end}

    // When the claim was made
    assertionTime := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

    claimID := inv.AssertClaim("senator",
        models.Proposition{Subject: "bill42", Predicate: "cost", Value: "5B"},
        assertionTime, iv)

    // Attach evidence
    inv.AddEvidence(claimID, "CBO score shows $4.8B", models.Supports,
        investigation.WithWeight(0.85))
    inv.AddEvidence(claimID, "Treasury model shows $7.2B", models.Refutes,
        investigation.WithWeight(0.75))

    // Analyze
    a, _ := inv.AnalyzeClaim(claimID)
    fmt.Println(a.FiveQuestions())
}
```

Output:

```
=== Five Questions ===
Q1 (What):  bill42 cost = 5B
Q2 (When claimed): Senator X claimed at 2024-02-01
Q3 (About):  Bill S.42 (bill42)
Q4 (Event interval): 2024-01-01 to 2024-12-31
Q5 (Credibility): Belnap=B, E[p]=0.645 (b=0.443 d=0.152 u=0.405)
   Evidence: 1 supporting, 1 refuting, 0 neutral (r=2.18, s=0.75)
   Actor reliability: 0.700 (base=0.700, after conflicts)
```

---

## Core concepts

### Claims and subjects

Every claim is a **triple**: `(subject, predicate, value)` — wrapped in a `models.Proposition`.

```go
prop := models.Proposition{Subject: "company_x", Predicate: "revenue", Value: "50M"}
```

Attach it to an actor assertion with `AssertClaim`, or record it as ground truth with `AssertFact`.

### Meta-claims (claims about claims)

A claim can target another claim rather than a subject entity. This is how you model attribution chains — "journalist says politician said X" — without asserting X is true.

```go
// Senator's direct claim
senClaimID := inv.AssertClaim("senator",
    models.Proposition{Subject: "bill42", Predicate: "cost", Value: "5B"}, t1, iv)

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

// Combine results with logical operations (package-level functions)
r1 := inv.Q("bill42", "cost", iv)[0]
r2 := inv.Q("bill42", "feasibility", iv)[0]
both := investigation.And(r1, r2)
either := investigation.Or(r1, r2)
negated := investigation.Not(r1)
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

## Argumentation frameworks

The `argumentation` package implements Dung abstract argumentation (Dung 1995) extended with bipolar support (Cayrol & Lagasquie-Schiex 2005). This is the fourth formal leg, layered on top of the first three.

### Building a framework from an investigation

```go
// Construct a Dung framework over all claims about a subject+predicate
fw := inv.BuildArgumentFramework("bill42", "cost", iv)
```

The framework automatically:
- Creates arguments from matching claims (carrying Belnap status and subjective opinion as strength)
- Generates mutual **rebut** attacks between contradictory claims with overlapping intervals
- Converts **refuting** meta-claims into rebut attacks
- Converts **supporting** meta-claims into support links (for causal chains)

### Semantics

```go
// Grounded extension: least fixpoint — the most conservative defensible set
ext := fw.GroundedExtension()

// Grounded labelling: three-valued In/Out/Undec per argument
labels := fw.GroundedLabelling()

// Preferred extensions: maximal admissible sets — alternative narrative scenarios
pref := fw.PreferredExtensions()

// Stable extensions: every non-member is attacked by a member
stable := fw.StableExtensions()

// Narrative entropy: 0 = one dominant narrative, higher = more contested
entropy := fw.NarrativeEntropy()
```

### Causal chains

Support links model causal dependencies. Attacking the root of a chain propagates defeat to all downstream arguments (BAF necessary support interpretation).

```go
chains := fw.FindCausalChains()
for _, chain := range chains {
    strength := fw.ChainStrength(chain)        // subjective multiplication across links
    weak := fw.ChainWeakestLink(chain)          // bottleneck argument
    defensible := fw.ChainDefensible(chain)     // is the full chain in grounded?
}
```

### Belnap-Dung bridge

Two complementary Belnap perspectives:
- **Evidence-level** (`BelnapStatus`): from `FromCounts(supporting, refuting)` within a single claim
- **Cross-extension** (`CrossExtensionBelnap`): `Both` when an argument appears in some preferred extensions but not others — genuine structural uncertainty

```go
combined := fw.BelnapStatus(argID)           // structural ⊔ evidence
crossExt := fw.CrossExtensionBelnap(argID)   // Both if in some extensions, not others
```

---

## Viewer

The `viewer` package provides an HTMX-based web interface for exploring argumentation frameworks interactively.

```go
import "github.com/pnathan/uncertain_logic/viewer"

// Blocks, serving the investigation at http://localhost:8080
viewer.Serve(inv, ":8080")
```

Features:
- **Static hierarchical graph layout** — rows = argumentation status (In/Out/Undec), columns = subject
- **Click any node** to see its full analysis: Belnap status, opinion, attackers, evidence, extension membership
- **Extensions panel** — grounded, preferred, and stable extensions with narrative entropy
- **Causal chains panel** — chain paths, strength, weakest link, defensibility
- Works with any `Investigation` object — generic, not demo-specific

---

## Iraq WMD demo

`cmd/wmd-demo/` models the Iraq WMD intelligence failure (2002–2004) as a case study. It exercises all four formal systems on real-world intelligence analysis, demonstrating how the library handles:

- 14 actors (CIA, DIA, INR, DOE, MI6, BND, Curveball, INC, UNMOVIC, IAEA, Powell, Wilson, ISG)
- 7 subjects (aluminum tubes, mobile bio-labs, Niger uranium, nuclear program, chemical weapons, biological program, long-range missiles)
- ~66 claims with supporting/refuting evidence
- Post-invasion ground truth (ISG findings) contradicting pre-invasion assessments

```bash
go run ./cmd/wmd-demo/
# Prints Q() results, then opens viewer at http://localhost:8080
```

---

## Known limitations

The four pillars cover epistemological reasoning — determining what's true and how credible it is. Four known gaps remain outside the current architecture:

### 1. Spatial reasoning

Allen interval algebra handles *when* but not *where*. Alibi reasoning ("could the suspect travel from A to B in the available time?"), geospatial intelligence, and supply chain tracking all require spatial relations that the library does not model. A region calculus or spatial constraint system would be a fifth pillar alongside temporal.

### 2. Source independence / correlation

`ConsensusFuse` assumes the opinions being fused are **independent**. When sources share an upstream origin — two journalists quoting the same leaker, or multiple intelligence agencies routing a single fabricator's reporting — fusing them as independent double-counts evidence and artificially reduces uncertainty.

The Iraq WMD case is the canonical example: CIA, DIA, and MI6 all reported mobile bio-labs, appearing to independently corroborate. All three traced back to a single source (Curveball) routed through BND. Independence-aware fusion would have treated this as one low-reliability opinion, not three corroborating ones.

Addressing this requires a **source dependency graph** and a modified fusion operator (Jøsang describes dependent-source fusion in his 2016 monograph). The meta-claim architecture can partially surface shared sourcing, but the fusion math does not currently discount for it.

### 3. Dynamic reliability

`Actor.BaseReliability` is a single static scalar. In practice, reliability varies along two dimensions:

- **Temporal**: a source becomes more or less reliable over time (witness contamination after coaching, institutional capture, analyst burnout). A `ReliabilityAt(time.Time)` function would capture this.
- **Topic-specific**: a nuclear physicist is expert-grade on enrichment but analyst-grade on biological agents. `ConflictOfInterest` is subject-scoped, which partially handles directional bias, but domain expertise boundaries are a different concept. A `ReliabilityFor(subjectID)` or topic-tagged reliability would be needed.

### 4. Denial and deception (D&D)

The hardest gap. The system models sources as noisy channels with fixed reliability. Real adversaries are **strategic agents** who build credibility (truthful leaks) then spend it (disinformation at a critical moment). Their behavior depends on what they believe the analyst believes — a game-theoretic interaction that none of the four pillars model.

The `Troll` source type and low reliability settings approximate known bad actors, but do not capture an adversary who is *sometimes truthful by design*. Addressing D&D would require modeling sources as agents with objectives, beliefs, and strategies — a fundamentally different paradigm from the current signal-processing approach.

| # | Limitation | Difficulty | Architectural impact |
|---|-----------|------------|---------------------|
| 1 | Spatial reasoning | Medium | New pillar alongside temporal |
| 2 | Source independence | Medium | Dependency graph + modified fusion operator |
| 3 | Dynamic reliability | Medium | `ReliabilityAt(time, subject)` replacing scalar |
| 4 | D&D / adversarial sources | Hard | Game-theoretic agent modeling |

Items 1–3 are extensions within the current paradigm. Item 4 is a paradigm shift.

---

## Design notes

**Why not Bayesian P(A|B)?** Bayesian reasoning degrades on genuinely rare, high-stakes events (lottery paradox). Belnap's `B` value is stable — contradicted claims stay `B` rather than collapsing to a probability that loses the contradiction signal.

**Why not just use databases?** Logical evaluation (Belnap aggregation, subjective fusion, meta-claim recursion) happens in Go, not SQL. Storage is the `map[string]*Claim` in `Investigation`; the interface is stable if you swap it for SQLite later.

**Composability**: `belnap`, `subjective`, `temporal`, and `argumentation` are pure logic packages with minimal dependencies. `argumentation` depends on `belnap` and `subjective` (for Belnap bridge and chain strength). `models` depends only on `temporal`. `investigation` is the only package that wires all four together. `viewer` depends on `investigation` and `argumentation`.
