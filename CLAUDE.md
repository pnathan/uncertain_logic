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

`uncertain_logic` is a Go library for reasoning about claims from sources that may lie, be ignorant, or contradict each other. It composes three formal systems:

### Packages

**`belnap/`** — Four-valued logic (N/T/F/B: Neither, True, False, Both-contradicted). `EvidenceJoin` accumulates evidence (T⊔F→B); `And`/`Or` are standard lattice ops. `FromCounts(supporting, refuting)` derives a value from evidence counts.

**`subjective/`** — Jøsang subjective logic: opinions as `(belief, disbelief, uncertainty, base_rate)` with invariant `b+d+u=1`. Key ops: `TrustDiscount(trust, claim)` applies source reliability; `ConsensusFuse(opinions...)` combines independent sources.

**`temporal/`** — Allen's 13 interval relations. `Relate(a,b)` returns the relation; `Overlapping(a,b)` gates whether claims temporally conflict. `Open(desc)` creates a conservative interval that overlaps everything.

**`models/`** — Domain entities: `Actor` (source with reliability), `Subject`, `Claim`, `Evidence`, `Proposition`, `ConflictOfInterest`. Nine source types (`Analyst`, `Journalist`, `Expert`, `Insider`, `Regulator`, `Institutional`, `Anonymous`, `SocialMedia`, `Troll`) all default to reliability 0.6. `ConflictOfInterest` discounts reliability (disclosed ×0.80, undisclosed ×0.50).

**`investigation/`** — Orchestrates the other three packages via a builder/DSL. This is the primary user-facing API.

### Investigation API (main entry point)

```go
inv := investigation.New("research question")
inv.AddActor(id, name, sourceType, opts...)   // WithReliability(float64)
inv.AddSubject(id, name, type)
claimID := inv.AssertClaim(actorID, prop, assertionTime, interval, opts...)
inv.AssertMetaClaim(actorID, targetClaimID, predicate, value, time, opts...)
inv.AssertFact(prop, interval, opts...)       // ground truth, reliability=1.0
inv.AddEvidence(claimID, content, valence, opts...)  // WithWeight(float64)

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
```

### Key design invariants

- **Temporal overlap gates contradiction**: non-overlapping claim intervals never produce a Belnap `B` (Both). Temporal evolution ≠ logical contradiction.
- **Meta-claims enable reification**: `AssertMetaClaim` models attribution chains (journalist claims expert said X) without asserting the base claim is true.
- **Depth-limited recursion** (default 3 levels): prevents infinite cycles when tracing meta-claim chains.
- **`ClaimAnalysis`** holds both a Belnap value (categorical) and a subjective Opinion (probabilistic), exposing complementary views.
