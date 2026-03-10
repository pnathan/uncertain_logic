// Package viewer provides an optional HTMX-based web interface for
// visualizing investigation argumentation frameworks.
//
// Usage:
//
//	inv := investigation.New("research question")
//	// ... populate investigation ...
//	viewer.Serve(inv, ":8080") // blocks
package viewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"text/template"

	"github.com/pnathan/uncertain_logic/argumentation"
	"github.com/pnathan/uncertain_logic/belnap"
	"github.com/pnathan/uncertain_logic/investigation"
	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/temporal"
)

// --- Graph Data Types ---

type graphNode struct {
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	Actor   string  `json:"actor"`
	Subject string  `json:"subject"`
	Belnap  string  `json:"belnap"`
	Status  string  `json:"status"`
	EP      float64 `json:"ep"`
	IsMeta  bool    `json:"is_meta"`
	Group   string  `json:"group"`
}

type graphLink struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	LinkType   string `json:"link_type"`
	AttackType string `json:"attack_type,omitempty"`
	Desc       string `json:"desc"`
}

type graphData struct {
	Nodes    []graphNode `json:"nodes"`
	Links    []graphLink `json:"links"`
	Grounded []string    `json:"grounded"`
	Entropy  float64     `json:"entropy"`
}

// --- Server ---

type server struct {
	inv *investigation.Investigation
	mu  sync.RWMutex
	// Cached per page-load
	fw       *argumentation.Framework
	labels   map[string]argumentation.Label
	claims   map[string]models.Claim
	actors   map[string]models.Actor
	evidence map[string]models.Evidence
	chains   []argumentation.CausalChain
}

// Serve starts the investigation viewer at the given address (e.g. ":8080").
func Serve(inv *investigation.Investigation, addr string) error {
	s := &server{inv: inv}
	s.rebuild()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /viewer.css", handleCSS)
	mux.HandleFunc("GET /viewer.js", handleJS)
	mux.HandleFunc("GET /htmx/overview", s.handleOverview)
	mux.HandleFunc("GET /htmx/node/{id}", s.handleNode)
	mux.HandleFunc("GET /htmx/extensions", s.handleExtensions)
	mux.HandleFunc("GET /htmx/chains", s.handleChains)

	fmt.Printf("Investigation viewer: http://%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *server) rebuild() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Index actors, claims, evidence
	s.actors = make(map[string]models.Actor)
	for _, a := range s.inv.Actors() {
		s.actors[a.ID] = a
	}
	s.claims = make(map[string]models.Claim)
	for _, c := range s.inv.Claims() {
		s.claims[c.ID] = c
	}
	s.evidence = make(map[string]models.Evidence)
	for _, e := range s.inv.Evidence() {
		s.evidence[e.ID] = e
	}

	// Build full argumentation framework
	s.fw = s.buildFullFramework()
	s.labels = s.fw.GroundedLabelling()
	s.chains = s.fw.FindCausalChains()
}

func (s *server) buildFullFramework() *argumentation.Framework {
	fw := argumentation.New()
	claimsList := make([]models.Claim, 0, len(s.claims))

	for _, c := range s.claims {
		analysis, err := s.inv.AnalyzeClaim(c.ID)
		if err != nil {
			continue
		}
		actorName := ""
		if a, ok := s.actors[c.ActorID]; ok {
			actorName = a.Name
		}
		desc := c.Content
		if desc == "" {
			desc = fmt.Sprintf("%s %s=%s", actorName, c.Predicate, c.Value)
		}
		fw.AddArgument(argumentation.Argument{
			ID:       c.ID,
			ClaimID:  c.ID,
			Strength: analysis.Credibility,
			Status:   analysis.BelnapStatus,
			Desc:     desc,
		})
		claimsList = append(claimsList, c)
	}

	// Rebut attacks: same subject+predicate, different value, temporally overlapping
	for i := 0; i < len(claimsList); i++ {
		a := claimsList[i]
		if a.SubjectID == "" {
			continue
		}
		for j := i + 1; j < len(claimsList); j++ {
			b := claimsList[j]
			if a.SubjectID == b.SubjectID && a.Predicate == b.Predicate &&
				a.Value != b.Value && temporal.Overlapping(a.EventInterval, b.EventInterval) {
				fw.AddAttack(argumentation.Attack{
					AttackerID: a.ID, TargetID: b.ID, Type: argumentation.Rebut,
					Desc: fmt.Sprintf("%q vs %q", a.Value, b.Value),
				})
				fw.AddAttack(argumentation.Attack{
					AttackerID: b.ID, TargetID: a.ID, Type: argumentation.Rebut,
					Desc: fmt.Sprintf("%q vs %q", b.Value, a.Value),
				})
			}
		}
	}

	// Meta-claims → attacks or supports
	for _, c := range claimsList {
		if c.TargetClaimID == "" {
			continue
		}
		switch c.Valence {
		case models.Refutes:
			fw.AddAttack(argumentation.Attack{
				AttackerID: c.ID, TargetID: c.TargetClaimID,
				Type: argumentation.Rebut,
				Desc: fmt.Sprintf("meta-claim disputes %s", c.TargetClaimID),
			})
		case models.Supports:
			fw.AddSupport(argumentation.Support{
				SupporterID: c.ID, SupportedID: c.TargetClaimID,
				Desc: fmt.Sprintf("meta-claim supports %s", c.TargetClaimID),
			})
		}
	}

	// Meta-claim conflict: meta-claims targeting the same claim with
	// opposite valences should rebut each other. This bridges components
	// when multiple actors assess the same claim differently.
	type metaRef struct {
		claimID string
		valence models.Valence
	}
	byTarget := make(map[string][]metaRef)
	for _, c := range claimsList {
		if c.TargetClaimID == "" {
			continue
		}
		byTarget[c.TargetClaimID] = append(byTarget[c.TargetClaimID], metaRef{c.ID, c.Valence})
	}
	for _, refs := range byTarget {
		for i := 0; i < len(refs); i++ {
			for j := i + 1; j < len(refs); j++ {
				if refs[i].valence != refs[j].valence {
					fw.AddAttack(argumentation.Attack{
						AttackerID: refs[i].claimID, TargetID: refs[j].claimID,
						Type: argumentation.Rebut,
						Desc: "opposing assessments of same claim",
					})
					fw.AddAttack(argumentation.Attack{
						AttackerID: refs[j].claimID, TargetID: refs[i].claimID,
						Type: argumentation.Rebut,
						Desc: "opposing assessments of same claim",
					})
				}
			}
		}
	}

	// Actor coherence: bridge an actor's claims across different subjects.
	// Only link claims in DIFFERENT subject groups (the bridging purpose).
	// Pick one representative per subject to avoid O(n²) edges.
	type actorSubj struct {
		actor   string
		subject string
	}
	subjRep := make(map[actorSubj]string) // first claim per actor+subject
	for _, c := range claimsList {
		if c.ActorID == "" {
			continue
		}
		key := actorSubj{c.ActorID, c.SubjectID}
		if _, exists := subjRep[key]; !exists {
			subjRep[key] = c.ID
		}
	}
	// Group representatives by actor
	actorReps := make(map[string][]string)
	for key, id := range subjRep {
		actorReps[key.actor] = append(actorReps[key.actor], id)
	}
	for _, reps := range actorReps {
		if len(reps) < 2 {
			continue
		}
		// Chain the per-subject representatives
		for i := 1; i < len(reps); i++ {
			fw.AddSupport(argumentation.Support{
				SupporterID: reps[i-1], SupportedID: reps[i],
				Desc: "same-actor coherence",
			})
		}
	}

	return fw
}

func (s *server) buildGraphData() graphData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var nodes []graphNode
	for _, arg := range s.fw.Arguments() {
		c := s.claims[arg.ClaimID]
		actorName := ""
		if a, ok := s.actors[c.ActorID]; ok {
			actorName = a.Name
		}
		label := "Undec"
		if l, ok := s.labels[arg.ID]; ok {
			label = l.String()
		}
		subj := c.SubjectID
		nodes = append(nodes, graphNode{
			ID:      arg.ID,
			Label:   truncate(arg.Desc, 40),
			Actor:   actorName,
			Subject: subj,
			Belnap:  arg.Status.String(),
			Status:  label,
			EP:      arg.Strength.ExpectedProbability(),
			IsMeta:  c.TargetClaimID != "",
			Group:   subj,
		})
	}

	var links []graphLink
	for _, atk := range s.fw.EffectiveAttacks() {
		links = append(links, graphLink{
			Source:     atk.AttackerID,
			Target:     atk.TargetID,
			LinkType:   "attack",
			AttackType: atk.Type.String(),
			Desc:       atk.Desc,
		})
	}
	for _, sup := range s.fw.Supports() {
		links = append(links, graphLink{
			Source:   sup.SupporterID,
			Target:   sup.SupportedID,
			LinkType: "support",
			Desc:     sup.Desc,
		})
	}

	return graphData{
		Nodes:    nodes,
		Links:    links,
		Grounded: s.fw.GroundedExtension(),
		Entropy:  s.fw.NarrativeEntropy(),
	}
}

// --- HTTP Handlers ---

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.rebuild() // rebuild on each page load for fresh data

	gd := s.buildGraphData()
	jsonBytes, _ := json.Marshal(gd)

	data := map[string]interface{}{
		"Question":       s.inv.ResearchQuestion,
		"ActorCount":     len(s.actors),
		"ClaimCount":     len(s.claims),
		"EvidenceCount":  len(s.evidence),
		"GroundedCount":  len(gd.Grounded),
		"PreferredCount": len(s.fw.PreferredExtensions()),
		"Entropy":        fmt.Sprintf("%.2f", gd.Entropy),
		"GraphJSON":      string(jsonBytes),
	}

	tmpl := template.Must(template.New("index").Parse(indexHTML))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

func handleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	fmt.Fprint(w, viewerCSS)
}

func handleJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	fmt.Fprint(w, viewerJS)
}

func (s *server) handleOverview(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="section"><h3>Investigation</h3><p>%s</p></div>`, esc(s.inv.ResearchQuestion))

	fmt.Fprint(w, `<div class="section"><h3>Actors</h3><table><tr><th>Name</th><th>Type</th><th>Reliability</th></tr>`)
	for _, a := range s.actors {
		fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%.2f</td></tr>`, esc(a.Name), a.SourceType, a.BaseReliability)
	}
	fmt.Fprint(w, `</table></div>`)

	fmt.Fprint(w, `<div class="section"><h3>Subjects</h3><table><tr><th>Name</th><th>Type</th></tr>`)
	for _, sub := range s.inv.Subjects() {
		fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td></tr>`, esc(sub.Name), esc(sub.SubjectType))
	}
	fmt.Fprint(w, `</table></div>`)

	fmt.Fprintf(w, `<div class="section"><h3>Metrics</h3>
		<div class="metric">Entropy: <span class="metric-value">%.2f bits</span></div>
		<div class="metric">Grounded: <span class="metric-value">%d args</span></div>
		<div class="metric">Preferred: <span class="metric-value">%d extensions</span></div>
		</div>`, s.fw.NarrativeEntropy(), len(s.fw.GroundedExtension()), len(s.fw.PreferredExtensions()))
}

func (s *server) handleNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.claims[id]
	if !ok {
		fmt.Fprintf(w, `<p>Claim %s not found</p>`, esc(id))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Claim content
	fmt.Fprintf(w, `<div class="section"><h3>Claim</h3><p>%s</p></div>`, esc(c.Content))

	// Actor
	if a, ok := s.actors[c.ActorID]; ok {
		adj := a.AdjustedReliability(c.SubjectID)
		fmt.Fprintf(w, `<div class="section"><h3>Actor</h3>
			<p>%s (%s)</p>
			<p>Reliability: %.3f (base %.3f)</p></div>`,
			esc(a.Name), a.SourceType, adj, a.BaseReliability)
	}

	// Status
	arg := s.fw.Argument(id)
	label := s.labels[id]
	if arg != nil {
		ep := arg.Strength.ExpectedProbability()
		fmt.Fprintf(w, `<div class="section"><h3>Status</h3>
			<p>Belnap: <span class="badge badge-%s">%s</span></p>
			<p>Argumentation: <span class="badge badge-%s">%s</span></p>
			<p>E[p] = %.3f</p>
			<p>Opinion: b=%.3f d=%.3f u=%.3f</p></div>`,
			strings.ToLower(arg.Status.String()), arg.Status,
			strings.ToLower(label.String()), label,
			ep,
			arg.Strength.Belief, arg.Strength.Disbelief, arg.Strength.Uncertainty)
	}

	// Combined Belnap (structural + evidence)
	combined := s.fw.BelnapStatus(id)
	crossExt := s.fw.CrossExtensionBelnap(id)
	fmt.Fprintf(w, `<div class="section"><h3>Combined Assessment</h3>
		<p>Structural ⊔ Evidence: <span class="badge badge-%s">%s</span></p>
		<p>Cross-extension: <span class="badge badge-%s">%s</span></p></div>`,
		belnapClass(combined), combined,
		belnapClass(crossExt), crossExt)

	// Attackers
	attackers := s.fw.AttackersOf(id)
	if len(attackers) > 0 {
		fmt.Fprint(w, `<div class="section"><h3>Attackers</h3><table><tr><th>Argument</th><th>Status</th></tr>`)
		for _, aid := range attackers {
			aLabel := s.labels[aid]
			aArg := s.fw.Argument(aid)
			desc := aid
			if aArg != nil {
				desc = truncate(aArg.Desc, 35)
			}
			fmt.Fprintf(w, `<tr><td class="clickable" hx-get="/htmx/node/%s" hx-target="#detail-content">%s</td><td><span class="badge badge-%s">%s</span></td></tr>`,
				aid, esc(desc), strings.ToLower(aLabel.String()), aLabel)
		}
		fmt.Fprint(w, `</table></div>`)
	}

	// Evidence
	if len(c.EvidenceIDs) > 0 {
		fmt.Fprint(w, `<div class="section"><h3>Evidence</h3>`)
		for _, eid := range c.EvidenceIDs {
			ev, ok := s.evidence[eid]
			if !ok {
				continue
			}
			vclass := "ev-neutral"
			switch ev.Valence {
			case models.Supports:
				vclass = "ev-supports"
			case models.Refutes:
				vclass = "ev-refutes"
			}
			fmt.Fprintf(w, `<div class="evidence-item"><span class="%s">%s</span>: %s (w=%.1f)</div>`,
				vclass, ev.Valence, esc(ev.Content), ev.EffectiveWeight())
		}
		fmt.Fprint(w, `</div>`)
	}

	// Extension membership
	pref := s.fw.PreferredExtensions()
	if len(pref) > 1 {
		fmt.Fprint(w, `<div class="section"><h3>Extension Membership</h3>`)
		for i, ext := range pref {
			member := false
			for _, eid := range ext {
				if eid == id {
					member = true
					break
				}
			}
			badge := "badge-out"
			mLabel := "Out"
			if member {
				badge = "badge-in"
				mLabel = "In"
			}
			fmt.Fprintf(w, `<p>Preferred %d: <span class="badge %s">%s</span></p>`, i+1, badge, mLabel)
		}
		fmt.Fprint(w, `</div>`)
	}
}

func (s *server) handleExtensions(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Grounded
	grounded := s.fw.GroundedExtension()
	fmt.Fprint(w, `<div class="section"><h3>Grounded Extension (least fixpoint)</h3>`)
	if len(grounded) == 0 {
		fmt.Fprint(w, `<p class="muted">Empty — no arguments are unconditionally defensible</p>`)
	} else {
		fmt.Fprint(w, `<div class="ext-group ext-grounded">`)
		for _, id := range grounded {
			arg := s.fw.Argument(id)
			desc := id
			if arg != nil {
				desc = truncate(arg.Desc, 35)
			}
			fmt.Fprintf(w, `<div class="clickable" hx-get="/htmx/node/%s" hx-target="#detail-content">%s</div>`, id, esc(desc))
		}
		fmt.Fprint(w, `</div>`)
	}
	fmt.Fprint(w, `</div>`)

	// Preferred
	pref := s.fw.PreferredExtensions()
	fmt.Fprintf(w, `<div class="section"><h3>Preferred Extensions (%d)</h3>`, len(pref))
	for i, ext := range pref {
		fmt.Fprintf(w, `<div class="ext-group"><div class="ext-label">Extension %d (%d arguments)</div><div class="ext-members">`, i+1, len(ext))
		for _, id := range ext {
			arg := s.fw.Argument(id)
			desc := id
			if arg != nil {
				desc = truncate(arg.Desc, 30)
			}
			fmt.Fprintf(w, `<span class="clickable" hx-get="/htmx/node/%s" hx-target="#detail-content">%s</span> `, id, esc(desc))
		}
		fmt.Fprint(w, `</div></div>`)
	}
	fmt.Fprint(w, `</div>`)

	// Stable
	stable := s.fw.StableExtensions()
	fmt.Fprintf(w, `<div class="section"><h3>Stable Extensions (%d)</h3>`, len(stable))
	if len(stable) == 0 {
		fmt.Fprint(w, `<p class="muted">None — indicates odd cycles in the attack graph</p>`)
	}
	for i, ext := range stable {
		fmt.Fprintf(w, `<div class="ext-group"><div class="ext-label">Stable %d</div><div class="ext-members">`, i+1)
		for _, id := range ext {
			fmt.Fprintf(w, `%s `, esc(id))
		}
		fmt.Fprint(w, `</div></div>`)
	}
	fmt.Fprint(w, `</div>`)

	// Entropy
	fmt.Fprintf(w, `<div class="section"><h3>Narrative Entropy</h3>
		<p><span class="metric-value">%.3f bits</span></p>
		<p class="muted">0 = one dominant narrative, higher = more contested</p></div>`, s.fw.NarrativeEntropy())
}

func (s *server) handleChains(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="section"><h3>Causal Chains (%d)</h3>`, len(s.chains))

	if len(s.chains) == 0 {
		fmt.Fprint(w, `<p class="muted">No causal chains (no support links in the framework)</p>`)
	}

	for i, chain := range s.chains {
		defensible := s.fw.ChainDefensible(chain)
		strength := s.fw.ChainStrength(chain)
		weak := s.fw.ChainWeakestLink(chain)

		cssClass := "chain-defeated"
		if defensible {
			cssClass = "chain-defensible"
		}

		fmt.Fprintf(w, `<div class="chain %s">`, cssClass)
		fmt.Fprintf(w, `<div><strong>Chain %d</strong></div>`, i+1)

		// Show chain path
		fmt.Fprint(w, `<div>`)
		for j, id := range chain.ArgumentIDs {
			if j > 0 {
				fmt.Fprint(w, ` <span class="chain-arrow">&rarr;</span> `)
			}
			arg := s.fw.Argument(id)
			desc := id
			if arg != nil {
				desc = truncate(arg.Desc, 25)
			}
			fmt.Fprintf(w, `<span class="clickable" hx-get="/htmx/node/%s" hx-target="#detail-content">%s</span>`, id, esc(desc))
		}
		fmt.Fprint(w, `</div>`)

		fmt.Fprintf(w, `<div class="muted">Strength: E[p]=%.3f | Defensible: %v</div>`, strength.ExpectedProbability(), defensible)
		if weak != nil {
			fmt.Fprintf(w, `<div class="muted">Weakest link: %s (E[p]=%.3f)</div>`, esc(truncate(weak.Desc, 30)), weak.Strength.ExpectedProbability())
		}
		fmt.Fprint(w, `</div>`)
	}
	fmt.Fprint(w, `</div>`)
}

// --- Helpers ---

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func belnapClass(v belnap.Value) string {
	switch v {
	case belnap.True:
		return "t"
	case belnap.False:
		return "f"
	case belnap.Both:
		return "b"
	default:
		return "n"
	}
}

// --- Embedded Assets ---

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Question}} — Investigation Viewer</title>
<link rel="stylesheet" href="/viewer.css">
<script src="https://unpkg.com/htmx.org@2.0.4"></script>
<script src="https://d3js.org/d3.v7.min.js"></script>
</head>
<body>
<header>
  <h1>{{.Question}}</h1>
  <div class="stats">
    <span class="metric">Actors: <span class="metric-value">{{.ActorCount}}</span></span>
    <span class="metric">Claims: <span class="metric-value">{{.ClaimCount}}</span></span>
    <span class="metric">Evidence: <span class="metric-value">{{.EvidenceCount}}</span></span>
    <span class="metric">|</span>
    <span class="metric">Grounded: <span class="metric-value">{{.GroundedCount}}</span></span>
    <span class="metric">Preferred: <span class="metric-value">{{.PreferredCount}}</span></span>
    <span class="metric">Entropy: <span class="metric-value">{{.Entropy}} bits</span></span>
  </div>
</header>
<main>
  <div id="graph-panel">
    <svg id="graph"></svg>
    <div class="legend">
      <div class="legend-item"><div class="legend-dot" style="background:#4ade80"></div>In (accepted)</div>
      <div class="legend-item"><div class="legend-dot" style="background:#f87171"></div>Out (defeated)</div>
      <div class="legend-item"><div class="legend-dot" style="background:#fbbf24"></div>Undec (contested)</div>
      <div class="legend-item"><div class="legend-line" style="background:#f87171"></div>Attack</div>
      <div class="legend-item"><div class="legend-line" style="background:#60a5fa"></div>Support</div>
    </div>
  </div>
  <div id="detail-panel">
    <div class="tabs">
      <div class="tab active" hx-get="/htmx/overview" hx-target="#detail-content"
           onclick="document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));this.classList.add('active')">Overview</div>
      <div class="tab" hx-get="/htmx/extensions" hx-target="#detail-content"
           onclick="document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));this.classList.add('active')">Extensions</div>
      <div class="tab" hx-get="/htmx/chains" hx-target="#detail-content"
           onclick="document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));this.classList.add('active')">Chains</div>
    </div>
    <div id="detail-content" class="detail-content" hx-get="/htmx/overview" hx-trigger="load"></div>
  </div>
</main>
<script id="graph-data" type="application/json">{{.GraphJSON}}</script>
<script src="/viewer.js"></script>
</body>
</html>`

const viewerCSS = `
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Menlo','Monaco','Courier New',monospace;background:#0f0f1a;color:#e0e0e0}
header{padding:12px 20px;background:#1a1a2e;border-bottom:1px solid #333}
header h1{font-size:16px;color:#7ec8e3;font-weight:normal}
.stats{font-size:12px;color:#888;margin-top:4px}
main{display:flex;height:calc(100vh - 56px)}
#graph-panel{flex:7;position:relative;overflow:hidden}
#graph-panel svg{width:100%;height:100%;background:#0f0f1a}
#detail-panel{flex:3;min-width:300px;border-left:1px solid #333;overflow-y:auto;background:#16162a}
.tabs{display:flex;border-bottom:1px solid #333;background:#1a1a2e}
.tab{padding:8px 14px;cursor:pointer;font-size:12px;color:#888;border-bottom:2px solid transparent;transition:all .15s}
.tab:hover{color:#e0e0e0}
.tab.active{color:#7ec8e3;border-bottom-color:#7ec8e3}
.detail-content{padding:16px;font-size:13px;line-height:1.6}
.section{margin-bottom:20px}
.section h3{color:#7ec8e3;font-size:13px;margin-bottom:6px;border-bottom:1px solid #2a2a3a;padding-bottom:4px}
.badge{display:inline-block;padding:2px 8px;border-radius:3px;font-size:11px;font-weight:bold}
.badge-in,.badge-t{background:#1a4a2e;color:#4ade80}
.badge-out,.badge-f{background:#4a1a1a;color:#f87171}
.badge-undec,.badge-n{background:#2a2a3a;color:#888}
.badge-b{background:#3a1a4a;color:#c084fc}
.metric{display:inline-block;margin-right:12px}
.metric-value{color:#7ec8e3}
.muted{color:#666;font-size:11px;margin-top:2px}
table{border-collapse:collapse;width:100%}
th,td{text-align:left;padding:4px 8px;border-bottom:1px solid #2a2a3a;font-size:12px}
th{color:#7ec8e3}
.clickable{cursor:pointer;color:#60a5fa;text-decoration:underline}
.clickable:hover{color:#93c5fd}
.chain{padding:10px;margin:6px 0;background:#1a1a2e;border-radius:4px;font-size:12px}
.chain-arrow{color:#60a5fa;font-weight:bold}
.chain-defensible{border-left:3px solid #4ade80}
.chain-defeated{border-left:3px solid #f87171}
.ext-group{margin:6px 0;padding:8px;background:#1a1a2e;border-radius:4px}
.ext-grounded{border-left:3px solid #4ade80}
.ext-label{color:#7ec8e3;font-size:11px;margin-bottom:4px}
.ext-members{font-size:12px}
.evidence-item{padding:4px 0;border-bottom:1px solid #2a2a3a}
.ev-supports{color:#4ade80}
.ev-refutes{color:#f87171}
.ev-neutral{color:#888}
.legend{position:absolute;bottom:12px;left:12px;background:rgba(22,22,42,.92);padding:10px 14px;border-radius:6px;font-size:11px;border:1px solid #333}
.legend-item{display:flex;align-items:center;margin:3px 0}
.legend-dot{width:12px;height:12px;border-radius:50%;margin-right:8px;flex-shrink:0}
.legend-line{width:20px;height:3px;margin-right:8px;border-radius:1px;flex-shrink:0}
.node{cursor:pointer}
.node text{font-size:10px;fill:#aaa;pointer-events:none}
.node.selected circle{stroke:#fff!important;stroke-width:3px!important}
`

const viewerJS = `
window.addEventListener('load', function() {
  if (typeof d3 === 'undefined') {
    document.getElementById('graph-panel').innerHTML =
      '<p style="color:#f87171;padding:20px">Error: D3.js failed to load from CDN</p>';
    return;
  }

  const raw = document.getElementById('graph-data').textContent;
  const data = JSON.parse(raw);
  if (!data.nodes || data.nodes.length === 0) {
    document.getElementById('graph-panel').innerHTML =
      '<p style="color:#888;padding:20px">No arguments in the framework</p>';
    return;
  }

  const svg = d3.select('svg#graph');
  const panel = document.getElementById('graph-panel');
  const W = panel.clientWidth || 800;
  const H = panel.clientHeight || 600;
  svg.attr('width', W).attr('height', H);

  // --- Hierarchical layout: rows = status (In/Undec/Out), columns = subject ---
  // Collect unique subjects (columns)
  const subjSet = new Set();
  data.nodes.forEach(n => subjSet.add(n.group || '_meta'));
  const subjects = [...subjSet].filter(s => s !== '_meta');
  subjects.sort();
  subjects.push('_meta');

  // Bucket nodes into grid cells: [status][subject] → nodes[]
  const statusOrder = ['In','Out','Undec'];
  const statusLabel = {In:'Accepted (Grounded)', Out:'Defeated', Undec:'Contested'};
  const grid = {};
  statusOrder.forEach(s => { grid[s] = {}; subjects.forEach(subj => { grid[s][subj] = []; }); });
  data.nodes.forEach(n => {
    const s = statusOrder.includes(n.status) ? n.status : 'Undec';
    const subj = n.group || '_meta';
    grid[s][subj].push(n);
  });

  // Layout in a virtual canvas larger than viewport (zoom to explore)
  const vW = Math.max(W, subjects.length * 200);
  const vH = Math.max(H, 900);
  const pad = 70;
  const colW = (vW - pad*2) / subjects.length;

  // Row heights proportional to node count
  const rowCounts = statusOrder.map(s =>
    subjects.reduce((sum, subj) => sum + grid[s][subj].length, 0));
  const totalNodes = rowCounts.reduce((a,b) => a+b, 0) || 1;
  const availH = vH - pad*2;
  const minRowH = 80;
  const rowHeights = rowCounts.map(c =>
    Math.max(minRowH, availH * Math.max(c / totalNodes, 0.15)));
  // Normalize to fit
  const sumH = rowHeights.reduce((a,b) => a+b, 0);
  const scale = availH / sumH;
  rowHeights.forEach((h, i) => { rowHeights[i] = h * scale; });

  let rowY = pad;
  const rowYs = [];
  rowHeights.forEach(h => { rowYs.push(rowY); rowY += h; });

  // Position nodes within each cell
  statusOrder.forEach((status, ri) => {
    subjects.forEach((subj, ci) => {
      const cell = grid[status][subj];
      const cellX = pad + ci * colW;
      const cellY = rowYs[ri];
      const cellH = rowHeights[ri];
      const nCell = cell.length;
      if (nCell === 0) return;
      // Single column layout within cell, staggered vertically
      const dy = Math.min(40, cellH / (nCell + 1));
      cell.forEach((n, idx) => {
        // Stagger X slightly for readability
        const xOff = (idx % 2) * 20;
        n.x = cellX + colW * 0.3 + xOff;
        n.y = cellY + dy * (idx + 1);
      });
    });
  });

  // Set viewBox for pan/zoom
  svg.attr('viewBox', '0 0 '+vW+' '+vH)
     .attr('preserveAspectRatio', 'xMidYMid meet');

  // Build lookup for link endpoints
  const nodeById = {};
  data.nodes.forEach(n => { nodeById[n.id] = n; });

  // --- Render ---
  const defs = svg.append('defs');
  [['attack','#f87171'],['support','#60a5fa']].forEach(([name,color]) => {
    defs.append('marker').attr('id','arrow-'+name)
      .attr('viewBox','0 -5 10 10').attr('refX',22).attr('refY',0)
      .attr('markerWidth',6).attr('markerHeight',6).attr('orient','auto')
      .append('path').attr('d','M0,-5L10,0L0,5').attr('fill',color);
  });

  const g = svg.append('g');
  svg.call(d3.zoom().scaleExtent([0.2,5]).on('zoom', e => g.attr('transform',e.transform)));

  // Only render a subset of attack links to avoid hairball
  // Keep: support links (always), attack links where at least one endpoint is In or Out
  const visibleLinks = data.links.filter(l => {
    if (l.link_type === 'support') return true;
    const s = nodeById[l.source], t = nodeById[l.target];
    if (!s || !t) return false;
    return s.status === 'In' || s.status === 'Out' || t.status === 'In' || t.status === 'Out';
  });

  g.selectAll('.link').data(visibleLinks).join('line')
    .attr('class', 'link')
    .attr('x1', d => (nodeById[d.source]||{}).x||0)
    .attr('y1', d => (nodeById[d.source]||{}).y||0)
    .attr('x2', d => (nodeById[d.target]||{}).x||0)
    .attr('y2', d => (nodeById[d.target]||{}).y||0)
    .attr('stroke', d => d.link_type==='attack'?'#f87171':'#60a5fa')
    .attr('stroke-opacity', d => d.link_type==='attack'?0.45:0.5)
    .attr('stroke-width', d => d.link_type==='attack'?1.5:2)
    .attr('stroke-dasharray', d => d.link_type==='attack'?'4,3':null)
    .attr('marker-end', d => d.link_type==='support'?'url(#arrow-support)':null);

  // Row labels (status)
  const rowColors = {In:'#4ade80', Out:'#f87171', Undec:'#fbbf24'};
  statusOrder.forEach((status, ri) => {
    g.append('text')
      .attr('x', 8)
      .attr('y', rowYs[ri] + 14)
      .attr('fill', rowColors[status])
      .attr('font-size', '11px')
      .attr('font-weight', 'bold')
      .text(statusLabel[status]);
    if (ri > 0) {
      g.append('line')
        .attr('x1', 0).attr('x2', vW)
        .attr('y1', rowYs[ri]).attr('y2', rowYs[ri])
        .attr('stroke', '#2a2a3a').attr('stroke-width', 1)
        .attr('stroke-dasharray', '6,4');
    }
  });

  // Column labels (subject) — rotated 30° for readability
  subjects.forEach((subj, ci) => {
    const label = subj === '_meta' ? 'meta-claims' : subj;
    g.append('text')
      .attr('x', pad + ci * colW + colW * 0.3)
      .attr('y', pad - 12)
      .attr('fill', '#666')
      .attr('font-size', '10px')
      .attr('transform', 'rotate(-20,'+(pad + ci * colW + colW*0.3)+','+(pad-12)+')')
      .text(label);
    if (ci > 0) {
      g.append('line')
        .attr('x1', pad + ci * colW).attr('x2', pad + ci * colW)
        .attr('y1', pad - 5).attr('y2', rowYs[rowYs.length-1] + rowHeights[rowHeights.length-1])
        .attr('stroke', '#1a1a2e').attr('stroke-width', 1);
    }
  });

  const colors = {In:'#4ade80', Out:'#f87171', Undec:'#fbbf24'};

  const node = g.selectAll('.node').data(data.nodes).join('g')
    .attr('class','node')
    .attr('transform', d => 'translate('+d.x+','+d.y+')');

  node.append('circle')
    .attr('r', d => 6 + d.ep * 8)
    .attr('fill', d => colors[d.status]||'#888')
    .attr('fill-opacity', 0.3)
    .attr('stroke', d => colors[d.status]||'#888')
    .attr('stroke-width', 2);

  node.filter(d => d.is_meta).append('rect')
    .attr('x', d => -(5+d.ep*6)).attr('y', d => -(5+d.ep*6))
    .attr('width', d => 2*(5+d.ep*6)).attr('height', d => 2*(5+d.ep*6))
    .attr('fill','none').attr('stroke','#444').attr('stroke-width',1).attr('stroke-dasharray','2,2');

  // Labels: rotated 30° down, short text, hidden by default in dense cells
  node.append('text')
    .attr('dx', 14).attr('dy', 2)
    .attr('font-size','8px').attr('fill','#777')
    .attr('transform', 'rotate(30)')
    .attr('class', 'node-label')
    .text(d => {
      // Short: actor initial + predicate snippet
      const actor = d.actor ? d.actor.split(' ')[0].replace(/[()]/g,'') : '';
      const parts = d.label.split(' ');
      const short = parts.length > 2 ? parts.slice(1,3).join(' ') : d.label;
      return actor ? actor+': '+short.substring(0,18) : short.substring(0,22);
    });

  node.append('title').text(d =>
    d.label+'\\nActor: '+d.actor+'\\nBelnap: '+d.belnap+'\\nStatus: '+d.status+'\\nE[p]: '+d.ep.toFixed(3));

  // Show full label on hover
  node.on('mouseenter', function(e,d) {
    d3.select(this).select('.node-label')
      .text(d.actor ? d.actor.split('(')[0].trim()+': '+d.label : d.label)
      .attr('fill','#ddd').attr('font-size','10px');
    d3.select(this).raise(); // bring to front
  }).on('mouseleave', function(e,d) {
    const actor = d.actor ? d.actor.split(' ')[0].replace(/[()]/g,'') : '';
    const parts = d.label.split(' ');
    const short = parts.length > 2 ? parts.slice(1,3).join(' ') : d.label;
    const txt = actor ? actor+': '+short.substring(0,18) : short.substring(0,22);
    d3.select(this).select('.node-label')
      .text(txt).attr('fill','#777').attr('font-size','8px');
  });

  node.on('click', function(e,d) {
    // Highlight selection
    d3.selectAll('.node').classed('selected', false);
    d3.select(this).classed('selected', true);
    // Highlight connected links
    g.selectAll('.link').attr('stroke-opacity', l => {
      const sid = typeof l.source === 'object' ? l.source.id : l.source;
      const tid = typeof l.target === 'object' ? l.target.id : l.target;
      if (sid === d.id || tid === d.id) return 0.8;
      return l.link_type === 'attack' ? 0.2 : 0.25;
    }).attr('stroke-width', l => {
      const sid = typeof l.source === 'object' ? l.source.id : l.source;
      const tid = typeof l.target === 'object' ? l.target.id : l.target;
      return (sid === d.id || tid === d.id) ? 2.5 : (l.link_type === 'attack' ? 1 : 1.5);
    });
    document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));
    htmx.ajax('GET','/htmx/node/'+d.id,'#detail-content');
  });

  // Click background to reset highlighting
  svg.on('click', function(e) {
    if (e.target === this || e.target === svg.node()) {
      g.selectAll('.link')
        .attr('stroke-opacity', d => d.link_type==='attack'?0.45:0.5)
        .attr('stroke-width', d => d.link_type==='attack'?1.5:2);
      d3.selectAll('.node').classed('selected', false);
    }
  });
});
`
