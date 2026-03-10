# The Four Pillars of Logic

This document explains, in plain language, what the four formal systems in `uncertain_logic` are doing and why they matter together. No math background required — just a willingness to think carefully about messy information.

---

## The Problem

Imagine you're an investigative journalist, an intelligence analyst, or a researcher trying to figure out what's actually true about something important. You have sources. Some are reliable, some aren't. Some contradict each other. Some are talking about different time periods. Some have conflicts of interest. And some are making claims about *other people's claims* rather than about reality directly.

Traditional logic says a statement is true or false. Traditional probability gives you a number between 0 and 1. Neither handles the situation where two credible sources flatly contradict each other *and you don't know who's right*. Traditional approaches either explode (classical logic — one contradiction makes everything true) or paper over the conflict (probability — averaging away the disagreement signal).

`uncertain_logic` keeps the contradiction visible while still letting you reason about it. It does this by composing four formal systems, each handling a different aspect of the mess.

---

## Pillar 1: Belnap Four-Valued Logic — "What do we know?"

**The everyday version:** Instead of just "true" or "false," every claim gets one of four labels:

| Label | What it means | Example |
|-------|--------------|---------|
| **Neither** | Nobody has said anything about this | "Does the company have offices in Paraguay?" — no one's commented |
| **True** | Evidence supports it, nothing refutes it | Three independent sources confirm the CEO resigned |
| **False** | Evidence refutes it, nothing supports it | The document was proven to be a forgery |
| **Both** | Evidence supports it AND refutes it | CIA says they have WMDs, inspectors on the ground say they don't |

The key insight is **Both**. In classical logic, if something is both true and false, you can prove *anything* — the whole system breaks. Belnap's system doesn't break. "Both" is a stable answer that says: *we have genuine contradiction here, and that's information worth preserving*.

**When does "Both" apply?** Only when the contradicting claims are about the *same time period*. If an analyst said "the market is bullish" in 2020 and "the market is bearish" in 2023, that's not a contradiction — the market changed. Belnap `Both` only fires when claims overlap in time (that's where Pillar 3 comes in).

**What Belnap doesn't tell you:** How *much* to believe something. A claim with one weak supporting source and one strong refuting source both gets "Both" — same as a claim with ten strong sources on each side. For the "how much" question, you need Pillar 2.

---

## Pillar 2: Subjective Logic — "How much should we believe?"

**The everyday version:** Every claim also carries a *credibility score* that's more nuanced than a single probability. It's a triplet:

- **Belief** — how much evidence actively supports this
- **Disbelief** — how much evidence actively refutes this
- **Uncertainty** — how much we simply don't know

These three always add up to 1. A claim with (0.4, 0.1, 0.5) means: moderate support, a little counter-evidence, and we're still half in the dark. Compare that to (0.8, 0.2, 0.0) — strong support, some opposition, and we feel we've seen all the relevant evidence.

The crucial distinction from regular probability is **uncertainty is explicit**. A regular probability of 0.5 could mean "we've seen tons of evidence and it's perfectly split" or "we have no idea." Those are very different situations, and subjective logic distinguishes them.

**Trust discounting:** If a source has reliability 0.7 (on a 0–1 scale) and makes a confident claim, the system *discounts* that confidence proportionally. A perfectly reliable source (1.0) passes through unchanged. A completely unreliable source (0.0) produces pure uncertainty — the system acts as if they said nothing at all. This is why the library tracks *who* said something, not just *what* they said.

**Consensus fusion:** When multiple independent sources weigh in, their opinions are mathematically combined. More sources generally means less uncertainty — but if they disagree, the conflict shows up in the belief/disbelief balance.

**Conflict of interest handling:** An analyst with an undisclosed financial interest in the outcome gets their reliability halved (×0.50). Disclosed conflicts get a lighter penalty (×0.80). The idea: disclosed conflicts let the reader adjust for bias; undisclosed ones are worse because the reader didn't even know to adjust.

---

## Pillar 3: Allen Interval Algebra — "When did this happen?"

**The everyday version:** Claims aren't about instants — they're about time periods. "The company was profitable" could mean 2020–2022. "The company was losing money" could mean 2023–2024. These don't contradict each other because they're about different times.

Allen's interval algebra defines 13 possible relationships between two time periods: one could come before the other, they could overlap, one could contain the other, they could start or end together, etc. The library uses these to answer one critical question: **do these two claims overlap in time?**

If they overlap, contradictory claims can produce a Belnap "Both." If they don't overlap, contradictory values are fine — the subject simply changed over time.

**Open intervals:** Sometimes a source says "around the time of the IPO" without giving exact dates. The system handles this conservatively — an open interval is treated as potentially overlapping with everything. This means open-dated claims are more likely to trigger contradiction detection, which is the safe default when you're uncertain about timing.

**Why this matters in practice:** The Iraq WMD case is a good example. Pre-invasion intelligence assessments (2001–2003) claimed active WMD programs. Post-invasion ground truth (2003–2004) found no stockpiles. These time periods overlap, so the system correctly identifies the contradiction. But if an analyst had assessed the 1991 Gulf War period separately, claims about 1991 stockpiles wouldn't contradict 2003 findings — different periods, different assessments.

---

## Pillar 4: Dung Argumentation — "Who wins the argument?"

**The everyday version:** Once you have a collection of claims that contradict each other, you need a way to figure out which sets of claims can coexist — which "stories" hold together internally. That's what argumentation frameworks do.

Think of it like a debate with rules:
- Each claim is an **argument** in the framework
- When two claims contradict each other (same subject, same time period, different values), they **attack** each other
- When a meta-claim refutes a base claim, that's also an attack
- When a meta-claim supports a base claim, that's a **support link**

From these attack and support relationships, the system computes several things:

**Grounded extension — the conservative answer:** Which arguments survive all attacks and are unconditionally defensible? This is the "what can we say for sure" set. In a highly contested situation, this can be empty — meaning nothing is beyond dispute.

**Preferred extensions — the alternative scenarios:** What are the maximal internally consistent sets of arguments? Each preferred extension is a coherent narrative. If there are two preferred extensions, there are two viable stories about what happened. If there's one, the evidence points one way.

**Narrative entropy:** A single number measuring how contested the situation is. 0 bits means one dominant narrative. 1 bit means two equally plausible narratives. Higher means more fragmentation. This directly quantifies "how much do the sources disagree?"

**Causal chains:** Support links create dependency chains. If argument A supports argument B, and someone successfully attacks A, then B loses its support. The system traces these chains, measures their strength, and identifies the weakest link — the most vulnerable point in a chain of reasoning.

---

## How the Four Pillars Work Together

None of these systems is sufficient on its own. Here's how they compose:

1. **Temporal reasoning (Pillar 3) gates contradiction detection.** Two claims only contradict if they overlap in time. This prevents false contradictions from temporal evolution.

2. **Belnap logic (Pillar 1) categorizes the evidence situation.** After temporal gating, evidence for and against a claim is accumulated using the Belnap join: supporting evidence pushes toward True, refuting evidence pushes toward False, both together produces Both.

3. **Subjective logic (Pillar 2) quantifies credibility.** Each claim gets a trust-discounted, consensus-fused opinion that captures not just "how likely" but "how uncertain." Source reliability, conflicts of interest, and evidence weight all feed into this.

4. **Argumentation (Pillar 4) determines defensibility.** The Dung framework takes all the claims with their Belnap statuses and credibility opinions, builds the attack/support graph, and computes which narratives survive scrutiny. The grounded extension is the conservative answer; preferred extensions map out the contested territory.

The result is that any claim can be examined from two complementary perspectives:
- **Categorical**: Its Belnap value tells you whether it's supported, refuted, contradicted, or unknown
- **Probabilistic**: Its subjective opinion tells you how strongly, with explicit uncertainty

And the argumentation layer adds a structural dimension:
- **Defensible**: Is this claim part of the conservative consensus (grounded), or only viable under certain assumptions (preferred)?
- **Narrative context**: How many alternative stories exist, and how does this claim fit within them?

---

## A Concrete Example

Consider the Iraq WMD intelligence failure (the library includes this as `cmd/wmd-demo`):

- **Temporal**: All analytical assessments cover 2001–2003. Post-invasion findings cover 2003–2004. The overlap means pre-invasion claims and ground truth can contradict.

- **Belnap**: The CIA's claim "Iraq has active nuclear program" gets evidence from aluminum tube analysis (supporting) and IAEA inspection results (refuting). Result: **Both** — genuine contradiction in the evidence base.

- **Subjective**: CIA (reliability 0.75) is trust-discounted differently from Curveball (0.15, a known fabricator). When their opinions are fused, Curveball's confident claims barely move the needle because his trust discount reduces them to near-pure uncertainty.

- **Argumentation**: The grounded extension (conservative defensible set) ends up containing mostly the post-invasion ground truth — the ISG findings that no stockpiles existed. Pre-invasion intelligence assessments are attacked by this ground truth and by dissenting agencies (INR on nuclear, DOE on tubes). Narrative entropy is low because the post-invasion evidence overwhelms: there's essentially one surviving narrative.

The Five Questions that `FiveQuestions()` answers for any claim — What is claimed? When was it claimed? What is it about? When did it happen? How much do we believe it? — are each grounded in these four pillars working together.

---

## What the Four Pillars Don't Cover

The four pillars handle the epistemological core — figuring out what's true and how confident you should be. But there are four known gaps where the library reaches its limits. These matter for different domains (intelligence analysis, police investigation, journalism, research) in different ways.

### 1. Space

The library reasons about *when* things happen but not *where*. Allen's interval algebra gives you 13 temporal relationships, but there's no spatial equivalent.

Why it matters: In police work, alibi reasoning depends on spatial constraints — "could someone drive from the crime scene to the alibi location in 20 minutes?" In intelligence, geospatial analysis is a primary collection discipline. In supply chain investigations, physical routing matters.

The temporal pillar would need a spatial counterpart — something like a region calculus that can express relationships like "overlaps," "contains," or "adjacent to" for physical locations, the same way Allen intervals do for time periods.

### 2. Source Independence

When the library combines multiple sources' opinions (using consensus fusion), it assumes the sources are **independent** — that each one arrived at their view separately, based on their own information.

In reality, sources are often correlated:
- Two journalists may have talked to the same leaker
- Multiple intelligence agencies may be reading the same intercepted communication
- Two research papers may draw on the same underlying dataset
- Witnesses may have talked to each other before giving statements

When correlated sources are fused as if independent, the system **double-counts evidence** and becomes more confident than it should be. Two sources that look like independent corroboration but actually trace back to one origin should carry roughly the same weight as that one origin.

The Iraq WMD case makes this concrete: CIA, DIA, and MI6 all reported that Iraq had mobile biological weapons labs. This *looked* like three independent agencies corroborating the same finding. In reality, all three were reporting information that originated from a single source — an Iraqi defector codenamed Curveball, routed through German intelligence (BND). The "corroboration" was an echo, not independent confirmation.

The library's meta-claim architecture can *surface* shared sourcing (you can trace "agency X's claim is based on source Y"), but the fusion math doesn't currently adjust for it. Fixing this would require tracking which sources share upstream origins and discounting the fusion accordingly.

### 3. Dynamic Reliability

The library assigns each source a single reliability number that stays constant. In practice, reliability shifts along two axes:

**Over time:** A witness who was reliable immediately after an event may become less reliable after being coached, threatened, or simply as memory fades. An institutional source that was trustworthy under one administration may become politically captured under another. An analyst experiencing burnout produces lower-quality work over time.

**By topic:** A nuclear physicist who is a world expert on uranium enrichment may have no special insight into biological weapons. A financial analyst who is brilliant at valuing tech companies may be mediocre at commodity markets. Currently, the library treats reliability as a single number regardless of what the source is opining about.

The conflict-of-interest system partially addresses topic specificity — you can flag that an analyst has a financial interest in a specific subject, which reduces their reliability *for that subject*. But "domain expertise boundaries" are a different concept from conflicts of interest. A source can be perfectly honest and unconflicted but simply *not expert enough* in a particular area.

### 4. Denial and Deception (D&D)

This is the hardest gap, and it's fundamentally different from the other three.

The library models sources as **noisy channels** — they have some fixed probability of transmitting accurate information, like a radio signal with static. A reliability of 0.8 means roughly "80% of the time, what this source says reflects reality." The static doesn't change based on what you're listening for.

Real adversaries are not noisy channels. They are **strategic agents** who:
- Deliberately provide true information to build credibility
- Wait for a critical moment, then use that credibility to inject disinformation
- Adjust their behavior based on what they think *you* believe
- May sacrifice short-term goals to achieve long-term deception objectives

A classic example: during World War II, the British ran double agents who fed mostly true intelligence to Germany, building trust over months. At a critical moment (D-Day), they used that trust to feed false information about the invasion location. The "reliability" of these agents wasn't low — it was *strategically variable*, high when it didn't matter and low when it did.

The library's `Troll` source type and low reliability settings can flag known bad actors, but they can't capture an adversary who is *truthful by design* most of the time. Addressing this would require modeling sources not as channels with static but as *agents with their own beliefs, goals, and strategies* — a game-theoretic framework that is fundamentally different from the signal-processing approach the four pillars take.

### Summary

| Gap | One-line description | How hard to fix |
|-----|---------------------|-----------------|
| Spatial reasoning | No *where*, only *when* | Medium — add a fifth pillar |
| Source independence | Fusion assumes uncorrelated sources | Medium — dependency graph + modified math |
| Dynamic reliability | Fixed reliability, doesn't vary by time or topic | Medium — extend the Actor model |
| Denial & deception | Sources modeled as channels, not strategic agents | Hard — requires a paradigm shift |

The first three are extensions within the current architecture. The fourth would require a fundamentally different kind of reasoning — game theory rather than signal processing — and is an open problem even in purpose-built intelligence systems.

---

## Further Reading

The formal systems implemented here come from:

- **Belnap**: Nuel Belnap, "A Useful Four-Valued Logic" (1977). How to reason when your information might be contradictory.
- **Subjective logic**: Audun Jøsang, "Subjective Logic" (Springer, 2016). A calculus for reasoning under uncertainty with explicit trust.
- **Allen intervals**: James F. Allen, "Maintaining Knowledge about Temporal Intervals" (1983). The definitive framework for reasoning about time periods.
- **Dung argumentation**: Phan Minh Dung, "On the Acceptability of Arguments" (1995). The foundation for formal argumentation theory.
- **Bipolar argumentation**: Claudette Cayrol & Marie-Christine Lagasquie-Schiex, "On the Acceptability of Arguments in Bipolar Argumentation Frameworks" (2005). Extends Dung with support relations for causal reasoning.
