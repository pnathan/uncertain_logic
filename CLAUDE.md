# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go test ./...              # Run all tests
go test ./... -v           # Verbose test output
go test ./... -cover       # With coverage metrics
go test ./belnap/...       # Run a single package's tests
go test -run TestName ./investigation/...  # Run a specific test
```

No external dependencies — pure stdlib. Go 1.22 required.

## Architecture

`uncertain_logic` is a Go library for reasoning about claims from sources that may lie, be ignorant, or contradict each other. It composes four formal systems:

### Packages

**`belnap/`** — Four-valued logic (N/T/F/B: Neither, True, False, Both-contradicted). `EvidenceJoin` accumulates evidence (T⊔F→B); `And`/`Or` are standard lattice ops. `FromCounts(supporting, refuting)` derives a value from evidence counts.

**`subjective/`** — Jøsang subjective logic: opinions as `(belief, disbelief, uncertainty, base_rate)` with invariant `b+d+u=1`. Key ops: `TrustDiscount(trust, claim)` applies source reliability; `ConsensusFuse(opinions...)` combines independent sources; `AveragingFuse(opinions...)` combines dependent sources (idempotent, n-ary direct formula per Josang 2010 Eq.16).

**`temporal/`** — Allen's 13 interval relations. `Relate(a,b)` returns the relation; `Overlapping(a,b)` gates whether claims temporally conflict. `Open(desc)` creates a conservative interval that overlaps everything.

**`argumentation/`** — Dung abstract argumentation frameworks extended with bipolar support (Cayrol & Lagasquie-Schiex 2005) for causal chain reasoning. Core types: `Argument`, `Attack` (Rebut/Undercut/Undermine), `Support`, `Framework`. Semantics: `GroundedExtension()` (least fixpoint), `PreferredExtensions()` (maximal admissible), `StableExtensions()`. `GroundedLabelling()` returns 3-valued In/Out/Undec. Support chains propagate defeat via `EffectiveAttacks()` (BAF flattening). `FindCausalChains()` discovers causal paths; `ChainStrength()` uses subjective multiplication; `ChainWeakestLink()` finds bottlenecks. Bridges: `LabelToBelnap()`, `CrossExtensionBelnap()` (Both when in some extensions but not others), `NarrativeEntropy()` (Shannon entropy over extensions).

**`models/`** — Domain entities: `Actor` (source with reliability), `Subject`, `Claim`, `Evidence`, `Proposition`, `ConflictOfInterest`. Nine source types (`Analyst`, `Journalist`, `Expert`, `Insider`, `Regulator`, `Institutional`, `Anonymous`, `SocialMedia`, `Troll`) all default to reliability 0.6. `ConflictOfInterest` discounts reliability (disclosed ×0.80, undisclosed ×0.50).

**`investigation/`** — Orchestrates the other four packages via a builder/DSL. This is the primary user-facing API.

### Investigation API (main entry point)

```go
inv := investigation.New("research question")
inv.AddActor(id, name, sourceType, opts...)   // WithReliability(float64)
inv.AddSubject(id, name, type)
claimID := inv.AssertClaim(actorID, prop, assertionTime, interval, opts...)
inv.AssertMetaClaim(actorID, targetClaimID, predicate, value, time, opts...)
inv.AssertFact(prop, interval, opts...)       // ground truth, reliability=1.0
inv.AddEvidence(claimID, content, valence, opts...)  // WithWeight(float64)

// Source dependency declarations (echo-chamber correction)
inv.DeclareSourceDependency(actorID, upstreamID)    // actor depends on upstream source
inv.SourceDependencies()                             // read accessor (defensive copy)
inv.LoadSourceDependencies(deps)                     // bulk import for persistence

// Query & analyze
results := inv.Q(subjectID, predicate, interval)
analysis, err := inv.AnalyzeClaim(claimID)
fmt.Println(analysis.FiveQuestions())        // structured five-part answer

// Logical ops on QueryResults (package-level functions, not methods)
combined := investigation.And(result1, result2)
combined := investigation.Or(result1, result2)
negated := investigation.Not(result)

// Timeline views
inv.SubjectTimeline(subjectID)               // claims grouped by temporal overlap
inv.ActorBeliefHistory(actorID, subjectID)   // detect belief revisions

// Argumentation framework (Dung semantics over claims)
fw := inv.BuildArgumentFramework(subjectID, predicate, interval)
ext := fw.GroundedExtension()                // defensible arguments (least fixpoint)
labels := fw.GroundedLabelling()             // In/Out/Undec per argument
pref := fw.PreferredExtensions()             // alternative narrative sets
chains := fw.FindCausalChains()              // causal reasoning paths
fw.ChainDefensible(chain)                    // is the full chain defensible?
fw.ChainStrength(chain)                      // conjunction of link credibilities
fw.ChainWeakestLink(chain)                   // bottleneck argument
fw.NarrativeEntropy()                        // 0=resolved, high=contested
fw.CrossExtensionBelnap(argID)               // Both if in some extensions but not others
```

### Key design invariants

- **Temporal overlap gates contradiction**: non-overlapping claim intervals never produce a Belnap `B` (Both). Temporal evolution ≠ logical contradiction.
- **Meta-claims enable reification**: `AssertMetaClaim` models attribution chains (journalist claims expert said X) without asserting the base claim is true.
- **Depth-limited recursion** (default 3 levels): prevents infinite cycles when tracing meta-claim chains.
- **`ClaimAnalysis`** holds both a Belnap value (categorical) and a subjective Opinion (probabilistic), exposing complementary views.
- **Causal chain defeat propagation**: support links in the argumentation framework model causal dependencies; attacking the root of a support chain propagates defeat to all downstream arguments (BAF necessary support interpretation).
- **Grounded extension = least fixpoint**: the most conservative defensible narrative. The gap between grounded (lower) and union of preferred extensions (upper) is the genuinely contested territory.
- **Dependency-aware fusion**: `Q()` uses three-level fusion — ABF within each actor, ABF within each dependency group (connected component), CBF across independent groups. `DeclareSourceDependency` prevents echo-chamber amplification when multiple agencies cite the same upstream source. With no dependencies declared, behavior is backward-compatible (all singletons → pure CBF). Note: dependencies are **topic-agnostic** — declaring a dependency groups actors for all queries, not just the topic that motivated the dependency. This is an actor-level simplification; Josang 2016 Ch. 12 defines the theoretically complete claim-level model.
