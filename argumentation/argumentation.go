// Package argumentation implements Dung's abstract argumentation frameworks
// extended with bipolar support relations for causal chain reasoning.
//
// This is the fourth formal leg of the uncertain_logic library, integrating with:
//   - belnap: Argument labelling maps to four-valued logic (In→T, Out→F, Undec→N, cross-extension conflict→B)
//   - subjective: Argument strength as Jøsang opinions; chain strength via multiplication
//   - temporal: Attack generation is gated by temporal overlap (non-overlapping ≠ contradiction)
//
// References:
//   - Dung, "On the Acceptability of Arguments" (1995) — grounded, preferred, stable semantics
//   - Cayrol & Lagasquie-Schiex, "Bipolar Argumentation Frameworks" (2005) — support + attack
//   - Prakken, "ASPIC+" (2010) — rebut/undercut/undermine attack taxonomy
//   - Baroni, Caminada & Giacomin, "Argumentation Semantics" (2011) — labelling approach
package argumentation

import (
	"fmt"
	"math"
	"sort"

	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/subjective"
)

// AttackType classifies how one argument defeats another.
type AttackType int

const (
	Rebut     AttackType = iota // attacks the conclusion directly
	Undercut                    // attacks the inference step
	Undermine                   // attacks a premise
)

func (t AttackType) String() string {
	switch t {
	case Rebut:
		return "Rebut"
	case Undercut:
		return "Undercut"
	case Undermine:
		return "Undermine"
	default:
		return "Unknown"
	}
}

// Label represents the acceptability status of an argument.
type Label int

const (
	In    Label = iota // accepted: all attackers are Out
	Out                // defeated: at least one attacker is In
	Undec              // undecided: neither provably accepted nor defeated
)

func (l Label) String() string {
	switch l {
	case In:
		return "In"
	case Out:
		return "Out"
	case Undec:
		return "Undec"
	default:
		return "?"
	}
}

// Argument is a node in the argumentation framework.
type Argument struct {
	ID       string
	ClaimID  string             // back-reference to investigation claim
	Strength subjective.Opinion // credibility from subjective logic
	Status   belnap.Value       // evidence status from Belnap logic
	Desc     string
}

// Clone returns a deep copy.
func (a Argument) Clone() Argument { return a }

// Attack is a directed defeat edge.
type Attack struct {
	AttackerID string
	TargetID   string
	Type       AttackType
	Desc       string
}

// Support is a directed causal/evidential link. In a causal chain A→B→C,
// defeating A propagates defeat to B and C (necessary support interpretation).
type Support struct {
	SupporterID string
	SupportedID string
	Desc        string
}

// CausalChain is an ordered sequence of causally linked argument IDs.
type CausalChain struct {
	ArgumentIDs []string
}

// maxArgsForExhaustive caps exhaustive extension search. Beyond this,
// PreferredExtensions and StableExtensions fall back to the grounded extension.
const maxArgsForExhaustive = 25

// Framework is a Bipolar Argumentation Framework (BAF).
type Framework struct {
	args     map[string]*Argument
	attacks  []Attack
	supports []Support
}

// New creates an empty framework.
func New() *Framework {
	return &Framework{args: make(map[string]*Argument)}
}

// --- Mutators ---

// AddArgument registers an argument.
func (fw *Framework) AddArgument(arg Argument) {
	a := arg.Clone()
	fw.args[arg.ID] = &a
}

// AddAttack registers a directed attack.
func (fw *Framework) AddAttack(atk Attack) {
	fw.attacks = append(fw.attacks, atk)
}

// AddSupport registers a directed causal support link.
func (fw *Framework) AddSupport(sup Support) {
	fw.supports = append(fw.supports, sup)
}

// --- Accessors ---

// Argument returns a copy of the argument with the given ID, or nil.
func (fw *Framework) Argument(id string) *Argument {
	a, ok := fw.args[id]
	if !ok {
		return nil
	}
	clone := a.Clone()
	return &clone
}

// Arguments returns all arguments sorted by ID.
func (fw *Framework) Arguments() []Argument {
	out := make([]Argument, 0, len(fw.args))
	for _, a := range fw.args {
		out = append(out, a.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Attacks returns all explicit attack edges.
func (fw *Framework) Attacks() []Attack {
	out := make([]Attack, len(fw.attacks))
	copy(out, fw.attacks)
	return out
}

// Supports returns all support edges.
func (fw *Framework) Supports() []Support {
	out := make([]Support, len(fw.supports))
	copy(out, fw.supports)
	return out
}

// AttackersOf returns argument IDs that attack the given argument
// (effective attacks including support-chain propagation).
func (fw *Framework) AttackersOf(id string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, atk := range fw.EffectiveAttacks() {
		if atk.TargetID == id && !seen[atk.AttackerID] {
			seen[atk.AttackerID] = true
			result = append(result, atk.AttackerID)
		}
	}
	sort.Strings(result)
	return result
}

// --- BAF Flattening ---

// EffectiveAttacks returns all attacks including secondary attacks derived
// from support chains. Under the necessary support interpretation: if A
// supports B and C attacks A, then C secondarily attacks B.
//
// This implements secondary attacks only (not mediated attacks). A mediated
// attack would occur when C supports A and A attacks B — making C an indirect
// attacker of B. Under necessary support, support propagates defeat downward
// but does not propagate attack capability upward. See Cayrol &
// Lagasquie-Schiex (2013) for the distinction between secondary and mediated.
func (fw *Framework) EffectiveAttacks() []Attack {
	result := make([]Attack, len(fw.attacks))
	copy(result, fw.attacks)

	supportAdj := make(map[string][]string)
	for _, s := range fw.supports {
		supportAdj[s.SupporterID] = append(supportAdj[s.SupporterID], s.SupportedID)
	}
	if len(supportAdj) == 0 {
		return result
	}

	type edge struct{ from, to string }
	seen := make(map[edge]bool)
	for _, atk := range fw.attacks {
		seen[edge{atk.AttackerID, atk.TargetID}] = true
	}

	// For each explicit attack (C → A), BFS from A through outgoing
	// support edges, adding secondary attacks from C to each reachable node.
	for _, atk := range fw.attacks {
		queue := supportAdj[atk.TargetID]
		visited := map[string]bool{atk.TargetID: true}
		for len(queue) > 0 {
			next := queue[0]
			queue = queue[1:]
			if visited[next] {
				continue
			}
			visited[next] = true
			e := edge{atk.AttackerID, next}
			if !seen[e] {
				seen[e] = true
				result = append(result, Attack{
					AttackerID: atk.AttackerID,
					TargetID:   next,
					Type:       atk.Type,
					Desc:       fmt.Sprintf("secondary via %s: %s", atk.TargetID, atk.Desc),
				})
			}
			queue = append(queue, supportAdj[next]...)
		}
	}
	return result
}

// --- Dung Semantics ---

// GroundedExtension computes the unique grounded extension — the least fixpoint
// of the characteristic function F(S) = {A | every attacker of A is attacked by S}.
//
// This is the most conservative defensible set: the investigator's "ground truth."
// Connection to Tarski (1955): Dung's grounded semantics IS the least fixpoint.
func (fw *Framework) GroundedExtension() []string {
	allAttacks := fw.EffectiveAttacks()
	in := make(map[string]bool)
	changed := true
	for changed {
		changed = false
		for id := range fw.args {
			if in[id] {
				continue
			}
			if fw.defendedBy(id, in, allAttacks) {
				in[id] = true
				changed = true
			}
		}
	}
	return sortedKeys(in)
}

// defendedBy returns true if every attacker of argID is counter-attacked by set.
func (fw *Framework) defendedBy(argID string, set map[string]bool, attacks []Attack) bool {
	for _, atk := range attacks {
		if atk.TargetID == argID {
			if !fw.attackedBySet(atk.AttackerID, set, attacks) {
				return false
			}
		}
	}
	return true
}

// attackedBySet returns true if argID is attacked by any member of set.
func (fw *Framework) attackedBySet(argID string, set map[string]bool, attacks []Attack) bool {
	for _, atk := range attacks {
		if atk.TargetID == argID && set[atk.AttackerID] {
			return true
		}
	}
	return false
}

// GroundedLabelling computes the 3-valued labelling from the grounded extension.
//
//	In:    in the grounded extension (certainly accepted)
//	Out:   attacked by an In argument (certainly defeated)
//	Undec: genuinely contested territory (the "boundary region")
func (fw *Framework) GroundedLabelling() map[string]Label {
	allAttacks := fw.EffectiveAttacks()
	labels := make(map[string]Label, len(fw.args))
	for id := range fw.args {
		labels[id] = Undec
	}

	changed := true
	for changed {
		changed = false
		for id := range fw.args {
			if labels[id] != Undec {
				continue
			}
			allOut := true
			anyIn := false
			for _, atk := range allAttacks {
				if atk.TargetID != id {
					continue
				}
				if labels[atk.AttackerID] == In {
					anyIn = true
					break
				}
				if labels[atk.AttackerID] != Out {
					allOut = false
				}
			}
			if anyIn {
				labels[id] = Out
				changed = true
			} else if allOut {
				labels[id] = In
				changed = true
			}
		}
	}
	return labels
}

// IsConflictFree returns true if no argument in set attacks another in set.
func (fw *Framework) IsConflictFree(set map[string]bool) bool {
	allAttacks := fw.EffectiveAttacks()
	for _, atk := range allAttacks {
		if set[atk.AttackerID] && set[atk.TargetID] {
			return false
		}
	}
	return true
}

// IsAdmissible returns true if set is conflict-free and defends all its members.
func (fw *Framework) IsAdmissible(set map[string]bool) bool {
	allAttacks := fw.EffectiveAttacks()
	if !fw.IsConflictFree(set) {
		return false
	}
	for id := range set {
		if !fw.defendedBy(id, set, allAttacks) {
			return false
		}
	}
	return true
}

// PreferredExtensions computes all preferred extensions — maximal admissible sets.
// Falls back to grounded extension only for frameworks exceeding maxArgsForExhaustive.
func (fw *Framework) PreferredExtensions() [][]string {
	if len(fw.args) > maxArgsForExhaustive {
		return [][]string{fw.GroundedExtension()}
	}
	allAttacks := fw.EffectiveAttacks()
	ids := fw.sortedIDs()

	var admissible []map[string]bool
	fw.enumAdmissible(ids, 0, make(map[string]bool), allAttacks, &admissible)

	var result [][]string
	for i, s := range admissible {
		maximal := true
		for j, t := range admissible {
			if i != j && isProperSubset(s, t) {
				maximal = false
				break
			}
		}
		if maximal {
			result = append(result, sortedKeys(s))
		}
	}
	if len(result) == 0 {
		result = append(result, []string{})
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
	return result
}

func (fw *Framework) enumAdmissible(ids []string, idx int, cur map[string]bool, attacks []Attack, out *[]map[string]bool) {
	if fw.isAdmissibleWith(cur, attacks) {
		cp := make(map[string]bool, len(cur))
		for k := range cur {
			cp[k] = true
		}
		*out = append(*out, cp)
	}
	for i := idx; i < len(ids); i++ {
		cur[ids[i]] = true
		if fw.isConflictFreeWith(cur, attacks) {
			fw.enumAdmissible(ids, i+1, cur, attacks, out)
		}
		delete(cur, ids[i])
	}
}

func (fw *Framework) isConflictFreeWith(set map[string]bool, attacks []Attack) bool {
	for _, atk := range attacks {
		if set[atk.AttackerID] && set[atk.TargetID] {
			return false
		}
	}
	return true
}

func (fw *Framework) isAdmissibleWith(set map[string]bool, attacks []Attack) bool {
	if !fw.isConflictFreeWith(set, attacks) {
		return false
	}
	for id := range set {
		if !fw.defendedBy(id, set, attacks) {
			return false
		}
	}
	return true
}

// StableExtensions computes all stable extensions — conflict-free sets
// that attack every argument outside the set.
func (fw *Framework) StableExtensions() [][]string {
	if len(fw.args) > maxArgsForExhaustive {
		return nil
	}
	allAttacks := fw.EffectiveAttacks()
	ids := fw.sortedIDs()

	var result [][]string
	fw.enumStable(ids, 0, make(map[string]bool), allAttacks, &result)

	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
	return result
}

func (fw *Framework) enumStable(ids []string, idx int, cur map[string]bool, attacks []Attack, out *[][]string) {
	if idx == len(ids) {
		if !fw.isConflictFreeWith(cur, attacks) {
			return
		}
		for _, id := range ids {
			if cur[id] {
				continue
			}
			if !fw.attackedBySet(id, cur, attacks) {
				return
			}
		}
		*out = append(*out, sortedKeys(cur))
		return
	}
	cur[ids[idx]] = true
	fw.enumStable(ids, idx+1, cur, attacks, out)
	delete(cur, ids[idx])
	fw.enumStable(ids, idx+1, cur, attacks, out)
}

// --- Causal Chain Analysis ---

// FindCausalChains discovers all maximal directed paths through support edges.
// Each chain represents a causal reasoning sequence: defeating any link
// propagates defeat downstream via the BAF secondary attack mechanism.
func (fw *Framework) FindCausalChains() []CausalChain {
	supportAdj := make(map[string][]string)
	supported := make(map[string]bool)
	for _, s := range fw.supports {
		supportAdj[s.SupporterID] = append(supportAdj[s.SupporterID], s.SupportedID)
		supported[s.SupportedID] = true
	}

	// Roots: arguments with outgoing support edges but no incoming ones
	var roots []string
	for id := range fw.args {
		if _, hasSup := supportAdj[id]; hasSup && !supported[id] {
			roots = append(roots, id)
		}
	}
	sort.Strings(roots)

	var chains []CausalChain
	for _, root := range roots {
		fw.dfsChains(root, []string{root}, supportAdj, map[string]bool{root: true}, &chains)
	}
	sort.Slice(chains, func(i, j int) bool {
		a, b := chains[i].ArgumentIDs, chains[j].ArgumentIDs
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
	return chains
}

func (fw *Framework) dfsChains(cur string, path []string, adj map[string][]string, visited map[string]bool, out *[]CausalChain) {
	nexts := adj[cur]
	isLeaf := true
	for _, next := range nexts {
		if visited[next] {
			continue
		}
		isLeaf = false
		visited[next] = true
		newPath := make([]string, len(path)+1)
		copy(newPath, path)
		newPath[len(path)] = next
		fw.dfsChains(next, newPath, adj, visited, out)
		delete(visited, next)
	}
	if isLeaf && len(path) > 1 {
		cp := make([]string, len(path))
		copy(cp, path)
		*out = append(*out, CausalChain{ArgumentIDs: cp})
	}
}

// ChainDefensible returns true if every argument in the chain is In
// in the grounded extension.
func (fw *Framework) ChainDefensible(chain CausalChain) bool {
	labelling := fw.GroundedLabelling()
	for _, id := range chain.ArgumentIDs {
		if labelling[id] != In {
			return false
		}
	}
	return true
}

// ChainStrength computes overall chain credibility via subjective multiplication.
// Since all links must hold, this is conjunction: E[chain] = ∏ E[link_i].
func (fw *Framework) ChainStrength(chain CausalChain) subjective.Opinion {
	if len(chain.ArgumentIDs) == 0 {
		return subjective.Vacuous(0.5)
	}
	first := fw.args[chain.ArgumentIDs[0]]
	if first == nil {
		return subjective.Vacuous(0.5)
	}
	result := first.Strength
	for i := 1; i < len(chain.ArgumentIDs); i++ {
		arg := fw.args[chain.ArgumentIDs[i]]
		if arg == nil {
			continue
		}
		result = subjective.Multiply(result, arg.Strength)
	}
	return result
}

// ChainWeakestLink returns the argument in the chain with the lowest
// expected probability — the bottleneck in causal reasoning.
func (fw *Framework) ChainWeakestLink(chain CausalChain) *Argument {
	var weakest *Argument
	weakestEP := math.MaxFloat64
	for _, id := range chain.ArgumentIDs {
		arg := fw.args[id]
		if arg == nil {
			continue
		}
		ep := arg.Strength.ExpectedProbability()
		if ep < weakestEP {
			weakestEP = ep
			clone := arg.Clone()
			weakest = &clone
		}
	}
	return weakest
}

// --- Belnap Bridge ---

// LabelToBelnap maps a Dung labelling to Belnap four-valued logic.
//
//	In    → True    (accepted)
//	Out   → False   (defeated)
//	Undec → Neither (insufficient to decide)
func LabelToBelnap(l Label) belnap.Value {
	switch l {
	case In:
		return belnap.True
	case Out:
		return belnap.False
	default:
		return belnap.Neither
	}
}

// BelnapStatus returns the Belnap value for an argument by combining its
// grounded labelling (structural defensibility) with its internal evidence
// status via EvidenceJoin.
func (fw *Framework) BelnapStatus(argID string) belnap.Value {
	labelling := fw.GroundedLabelling()
	label, ok := labelling[argID]
	if !ok {
		return belnap.Neither
	}
	structural := LabelToBelnap(label)
	arg := fw.args[argID]
	if arg == nil {
		return structural
	}
	return belnap.EvidenceJoin(structural, arg.Status)
}

// CrossExtensionBelnap computes the Belnap value for an argument across
// preferred extensions using per-extension labellings.
//
//	In all    → True
//	Out all   → False
//	Mixed     → Both  (genuinely contradicted across narrative alternatives)
//	Undec all → Neither
//
// Unlike simple set-membership checks, this computes the full labelling
// for each preferred extension, correctly distinguishing Undec (genuinely
// undecided) from Out (defeated by an accepted argument).
func (fw *Framework) CrossExtensionBelnap(argID string) belnap.Value {
	exts := fw.PreferredExtensions()
	if len(exts) == 0 {
		return belnap.Neither
	}
	allAttacks := fw.EffectiveAttacks()
	inCount, outCount, undecCount := 0, 0, 0
	for _, ext := range exts {
		// Compute the labelling induced by this preferred extension:
		// In = in the extension, Out = attacked by an In member, Undec = neither.
		extSet := make(map[string]bool, len(ext))
		for _, id := range ext {
			extSet[id] = true
		}
		if extSet[argID] {
			inCount++
		} else if fw.attackedBySet(argID, extSet, allAttacks) {
			outCount++
		} else {
			undecCount++
		}
	}
	switch {
	case inCount > 0 && outCount > 0:
		return belnap.Both
	case inCount > 0 && undecCount > 0:
		return belnap.Both // accepted in some, undecided in others
	case inCount > 0:
		return belnap.True
	case outCount > 0 && undecCount > 0:
		return belnap.Both // defeated in some, undecided in others
	case outCount > 0:
		return belnap.False
	default:
		return belnap.Neither // undecided in all extensions
	}
}

// --- Information Theory ---

// NarrativeEntropy computes Shannon entropy over preferred extensions
// under a uniform distribution. Zero = one dominant narrative; maximum =
// all narratives equally supported. The investigator's goal is entropy
// reduction through evidence acquisition.
//
// Note: this uses a uniform distribution over extensions (each extension
// equally likely). Hunter & Thimm (2017) develop a probabilistic approach
// where extensions are weighted by argument strengths; this simpler model
// measures structural narrative uncertainty independent of strength.
//
// Reference: Hunter & Thimm, "Probabilistic Reasoning with Abstract
// Argumentation Frameworks" (2017).
func (fw *Framework) NarrativeEntropy() float64 {
	exts := fw.PreferredExtensions()
	n := len(exts)
	if n <= 1 {
		return 0
	}
	// Uniform distribution over extensions
	p := 1.0 / float64(n)
	entropy := 0.0
	for i := 0; i < n; i++ {
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// --- Helpers ---

func (fw *Framework) sortedIDs() []string {
	ids := make([]string, 0, len(fw.args))
	for id := range fw.args {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isProperSubset(a, b map[string]bool) bool {
	if len(a) >= len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
