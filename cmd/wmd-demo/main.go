// Command wmd-demo launches the investigation viewer with a model of the
// Iraq WMD intelligence failure (2001-2004).
//
// This demonstrates all four formal legs of uncertain_logic:
//   - Belnap four-valued logic: contradictory evidence → Both
//   - Jøsang subjective logic: source reliability, trust discounting
//   - Allen temporal logic: pre-invasion claims vs post-invasion findings
//   - Dung argumentation: attacks, support chains, extension semantics
//
// Usage:
//
//	go run ./cmd/wmd-demo
//	# then open http://localhost:8080
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/pnathan/uncertain_logic/investigation"
	"github.com/pnathan/uncertain_logic/models"
	"github.com/pnathan/uncertain_logic/temporal"
	"github.com/pnathan/uncertain_logic/viewer"
)

func main() {
	inv := buildIraqWMDInvestigation()

	// Print a summary before launching
	fmt.Println("=== Iraq WMD Intelligence Failure — Investigation Model ===")
	fmt.Println(inv.Summary())
	fmt.Println()

	// Show the echo-chamber effect on mobile-bio-labs.
	// We build a second investigation WITHOUT dependencies to get a clean
	// comparison, avoiding mutation of the primary investigation's state.
	fmt.Println("─── Echo-Chamber Effect: mobile-bio-labs / existence ───")

	bioQuery := temporal.Open("full timeline")

	// Build a separate no-dependency investigation for the "before" baseline.
	invNoDeps := buildIraqWMDInvestigation()
	resultsBefore := invNoDeps.Q("mobile-bio-labs", "existence", bioQuery)
	if len(resultsBefore) > 0 {
		r := resultsBefore[0]
		fmt.Printf("  WITHOUT dependency-aware fusion (echo-chamber):\n")
		fmt.Printf("    Belnap: %s  |  E[p]=%.3f  |  b=%.3f d=%.3f u=%.3f\n",
			r.Belnap, r.Opinion.ExpectedProbability(),
			r.Opinion.Belief, r.Opinion.Disbelief, r.Opinion.Uncertainty)
	}

	// Declare source dependencies on the primary investigation: the Curveball echo chamber
	inv.DeclareSourceDependency("cia", "curveball")
	inv.DeclareSourceDependency("dia", "curveball")
	inv.DeclareSourceDependency("mi6", "curveball")
	inv.DeclareSourceDependency("powell", "cia")
	inv.DeclareSourceDependency("inc", "curveball")

	// After: dependency-aware fusion (ABF within groups, CBF across)
	resultsAfter := inv.Q("mobile-bio-labs", "existence", bioQuery)
	if len(resultsAfter) > 0 {
		r := resultsAfter[0]
		fmt.Printf("  WITH dependency-aware fusion (echo-chain corrected):\n")
		fmt.Printf("    Belnap: %s  |  E[p]=%.3f  |  b=%.3f d=%.3f u=%.3f\n",
			r.Belnap, r.Opinion.ExpectedProbability(),
			r.Opinion.Belief, r.Opinion.Disbelief, r.Opinion.Uncertainty)
	}
	fmt.Println()

	// Analyze key claims and print five-questions output
	subjects := []struct {
		subjectID string
		predicate string
	}{
		{"aluminum-tubes", "purpose"},
		{"mobile-bio-labs", "existence"},
		{"niger-uranium", "acquisition"},
		{"iraq-nuclear", "active-program"},
		{"iraq-chem", "stockpiles"},
	}

	for _, s := range subjects {
		results := inv.Q(s.subjectID, s.predicate, temporal.Open("full timeline"))
		for _, r := range results {
			fmt.Printf("─── %s / %s ───\n", s.subjectID, s.predicate)
			fmt.Printf("  Belnap: %s  |  E[p]=%.3f  |  Opinion: b=%.3f d=%.3f u=%.3f\n",
				r.Belnap, r.Opinion.ExpectedProbability(),
				r.Opinion.Belief, r.Opinion.Disbelief, r.Opinion.Uncertainty)
			fmt.Println()
		}
	}

	fmt.Println("Launching viewer at http://localhost:8080 ...")
	log.Fatal(viewer.Serve(inv, ":8080"))
}

// --- helpers ---

func t(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func iv(y1, m1, d1, y2, m2, d2 int) temporal.EventInterval {
	start := t(y1, m1, d1)
	end := t(y2, m2, d2)
	return temporal.EventInterval{Start: &start, End: &end}
}

func prop(subject, predicate, value string) models.Proposition {
	return models.Proposition{Subject: subject, Predicate: predicate, Value: value}
}

func buildIraqWMDInvestigation() *investigation.Investigation {
	inv := investigation.New("Did Iraq possess weapons of mass destruction in 2002-2003?")

	// ─────────────────────────────────────────────────────────
	// ACTORS — the cast of the intelligence failure
	// ─────────────────────────────────────────────────────────

	// US Intelligence Community
	inv.AddActor("cia", "CIA (Directorate of Intelligence)", models.Institutional,
		investigation.WithReliability(0.70),
		investigation.WithActorNotes("Primary all-source intelligence agency; produced the 2002 NIE"))

	inv.AddActor("dia", "DIA (Defense Intelligence Agency)", models.Institutional,
		investigation.WithReliability(0.65),
		investigation.WithActorNotes("Pentagon intelligence arm; tended to align with CIA on WMD"))

	inv.AddActor("inr", "INR (State Dept Bureau of Intelligence and Research)", models.Analyst,
		investigation.WithReliability(0.80),
		investigation.WithCompetence("active-program", 0.90), // geopolitical analysis — INR's core strength
		investigation.WithCompetence("purpose", 0.70),        // technical evidence assessment
		investigation.WithCompetence("operational-link", 0.85),
		investigation.WithActorNotes("Smallest IC agency; filed formal dissent on nuclear claims in the 2002 NIE. "+
			"Post-invasion, proved to be the most accurate assessor"))

	inv.AddActor("doe", "DOE (Department of Energy)", models.Expert,
		investigation.WithReliability(0.85),
		investigation.WithCompetence("purpose", 0.95),         // centrifuge/nuclear hardware — DOE's core domain
		investigation.WithCompetence("active-program", 0.85),  // nuclear program assessment
		investigation.WithCompetence("existence", 0.40),       // biological weapons — outside DOE expertise
		investigation.WithCompetence("stockpiles", 0.40),      // chemical weapons — outside DOE expertise
		investigation.WithCompetence("operational-link", 0.30), // geopolitical analysis — not DOE's domain
		investigation.WithDefaultCompetence(0.50),
		investigation.WithActorNotes("Technical nuclear expertise; dissented on aluminum tubes purpose. "+
			"High competence on nuclear physics, low on biological/chemical weapons"))

	// Foreign intelligence
	inv.AddActor("mi6", "MI6 (UK Secret Intelligence Service)", models.Institutional,
		investigation.WithReliability(0.65),
		investigation.WithActorNotes("Provided the '45-minute WMD deployment' claim and Niger intelligence"))

	inv.AddActor("bnd", "BND (German Federal Intelligence Service)", models.Institutional,
		investigation.WithReliability(0.70),
		investigation.WithActorNotes("Handled Curveball; repeatedly warned about his unreliability"))

	// The infamous Curveball
	inv.AddActor("curveball", "Curveball (Rafid Ahmed Alwan al-Janabi)", models.Anonymous,
		investigation.WithReliability(0.25),
		investigation.WithActorNotes("Iraqi chemical engineer defector; fabricated mobile bio-lab claims. "+
			"Admitted fabrication in 2011 Guardian interview"))

	// Iraqi exile group
	inv.AddActor("inc", "Iraqi National Congress (Ahmed Chalabi)", models.Insider,
		investigation.WithReliability(0.30),
		investigation.WithConflict(models.ConflictOfInterest{
			Description: "INC sought US military intervention for regime change; Chalabi wanted to lead post-Saddam Iraq",
			Direction:   models.Other,
			Disclosed:   false, // conflict was known but downplayed
		}),
		investigation.WithActorNotes("Exile opposition group; funneled defectors to US intelligence; "+
			"later revealed many defector claims were fabricated or coached"))

	// International inspectors
	inv.AddActor("unmovic", "UNMOVIC (Hans Blix)", models.Regulator,
		investigation.WithReliability(0.85),
		investigation.WithActorNotes("UN weapons inspectors; returned to Iraq Nov 2002. "+
			"Found no WMD evidence in 700+ inspections before invasion"))

	inv.AddActor("iaea", "IAEA (Mohamed ElBaradei)", models.Regulator,
		investigation.WithReliability(0.90),
		investigation.WithActorNotes("International Atomic Energy Agency; determined Niger docs were forgeries, "+
			"found no evidence of nuclear reconstitution"))

	// Key individuals
	inv.AddActor("powell", "Colin Powell (Secretary of State)", models.Institutional,
		investigation.WithReliability(0.70),
		investigation.WithActorNotes("Presented the WMD case to UN Security Council on Feb 5, 2003; "+
			"later called it a 'blot' on his record"))

	inv.AddActor("wilson", "Joe Wilson (former Ambassador)", models.Expert,
		investigation.WithReliability(0.75),
		investigation.WithActorNotes("Sent by CIA to Niger in Feb 2002; found no evidence of uranium sale. "+
			"Published NYT op-ed July 2003"))

	// Post-invasion investigators (ground truth)
	inv.AddActor("isg", "Iraq Survey Group (Kay/Duelfer)", models.Expert,
		investigation.WithReliability(0.95),
		investigation.WithActorNotes("1400-member post-invasion team; exhaustive on-the-ground investigation. "+
			"Duelfer Report (2004) is definitive"))

	// ─────────────────────────────────────────────────────────
	// SUBJECTS — the contested questions
	// ─────────────────────────────────────────────────────────

	inv.AddSubject("iraq-nuclear", "Iraq Nuclear Weapons Program", "weapons-program")
	inv.AddSubject("iraq-bio", "Iraq Biological Weapons Program", "weapons-program")
	inv.AddSubject("iraq-chem", "Iraq Chemical Weapons Stockpiles", "weapons-program")
	inv.AddSubject("aluminum-tubes", "Aluminum Tubes", "evidence-item")
	inv.AddSubject("mobile-bio-labs", "Mobile Biological Weapons Labs", "evidence-item")
	inv.AddSubject("niger-uranium", "Niger Yellowcake Uranium", "evidence-item")
	inv.AddSubject("iraq-aq", "Iraq–Al Qaeda Operational Link", "relationship")

	// ─────────────────────────────────────────────────────────
	// Temporal intervals
	// ─────────────────────────────────────────────────────────
	// The WMD question spans the entire period: 9/11 through ISG final report.
	// Analytical assessments and ground truth use this wide interval so that
	// temporal overlap correctly gates contradictions.
	wmdQuestion := iv(2001, 9, 12, 2004, 9, 30)  // full analytical period
	preInvasion := iv(2001, 9, 12, 2003, 3, 19)  // post-9/11 to invasion
	inspections := iv(2002, 11, 27, 2003, 3, 18) // UNMOVIC return to invasion
	_ = iv(2003, 6, 1, 2004, 9, 30)              // postInvasion — now subsumed by wmdQuestion

	// =====================================================
	// THREAD 1: ALUMINUM TUBES
	// The single most contested piece of physical evidence
	// =====================================================

	ciaTubes := inv.AssertClaim("cia", prop("aluminum-tubes", "purpose", "centrifuge-components"),
		t(2001, 9, 1), preInvasion,
		investigation.WithClaimNotes("CIA assessed 7075-T6 aluminum tubes were for gas centrifuge rotors. "+
			"Key judgment in 2002 NIE, despite dissent"),
		investigation.WithSourceDescription("CIA WINPAC analysis"))
	inv.AddEvidence(ciaTubes, "Tubes match dimensions that could work in Zippe-type centrifuge",
		models.Supports, investigation.WithWeight(1.0))
	inv.AddEvidence(ciaTubes, "Iraq attempted to procure 60,000 tubes through front companies",
		models.Supports, investigation.WithWeight(1.2))
	inv.AddEvidence(ciaTubes, "Tubes had tight tolerances and anodized coating",
		models.Supports, investigation.WithWeight(0.8))

	doeTubes := inv.AssertClaim("doe", prop("aluminum-tubes", "purpose", "conventional-rockets"),
		t(2001, 9, 1), preInvasion,
		investigation.WithClaimNotes("DOE — the agency with actual centrifuge expertise — assessed tubes "+
			"were for 81mm Nasser rocket casings, matching known Iraqi rocket program specs"),
		investigation.WithSourceDescription("DOE/NNSA technical assessment"))
	inv.AddEvidence(doeTubes, "Tubes match specs of Italian Medusa 81mm rockets Iraq already uses",
		models.Supports, investigation.WithWeight(1.5))
	inv.AddEvidence(doeTubes, "Tube dimensions are wrong for Zippe centrifuge (too narrow, too thick-walled, wrong alloy)",
		models.Supports, investigation.WithWeight(1.8))
	inv.AddEvidence(doeTubes, "DOE has decades of centrifuge engineering experience; CIA analysts do not",
		models.Supports, investigation.WithWeight(1.3))

	inrTubes := inv.AssertClaim("inr", prop("aluminum-tubes", "purpose", "conventional-rockets"),
		t(2002, 10, 1), wmdQuestion,
		investigation.WithClaimNotes("INR's formal dissent in the 2002 NIE: tubes are for conventional rockets, "+
			"not centrifuges. The strongest institutional dissent in the NIE"),
		investigation.WithSourceDescription("INR dissent, Annex B of October 2002 NIE"))
	inv.AddEvidence(inrTubes, "INR concurred with DOE technical analysis on tube specifications",
		models.Supports, investigation.WithWeight(1.4))
	inv.AddEvidence(inrTubes, "No other evidence of reconstituted nuclear program besides tubes",
		models.Supports, investigation.WithWeight(1.2))

	// IAEA definitively resolved the tube question
	iaeaTubes := inv.AssertClaim("iaea", prop("aluminum-tubes", "purpose", "conventional-rockets"),
		t(2003, 1, 27), inspections,
		investigation.WithClaimNotes("IAEA report to UNSC: tubes are for 81mm rockets. Inspectors found "+
			"identical tubes in Iraqi rocket inventory, including 13,000 already in production"),
		investigation.WithSourceDescription("IAEA report to UN Security Council, Jan 2003"))
	inv.AddEvidence(iaeaTubes, "Found thousands of identical tubes already in use as rocket casings in Iraq",
		models.Supports, investigation.WithWeight(2.0))
	inv.AddEvidence(iaeaTubes, "Reverse-engineering analysis showed tubes incompatible with centrifuge use without modification",
		models.Supports, investigation.WithWeight(1.8))

	// Post-invasion ground truth
	inv.AssertFact(prop("aluminum-tubes", "purpose", "conventional-rockets"),
		wmdQuestion,
		investigation.WithClaimNotes("ISG/Duelfer Report confirmed: tubes were for Nasser 81mm rockets. "+
			"No centrifuge program existed. CIA was wrong, DOE/INR were right"))

	// =====================================================
	// THREAD 2: MOBILE BIOLOGICAL WEAPONS LABS
	// The Curveball fabrication
	// =====================================================

	curveballClaim := inv.AssertClaim("curveball", prop("mobile-bio-labs", "existence", "confirmed"),
		t(2000, 1, 1), iv(1998, 1, 1, 2003, 3, 19),
		investigation.WithClaimNotes("Curveball claimed to have worked at Djerf al-Nadaf seed plant where "+
			"mobile bio-labs were designed. He described trucks with fermenters for anthrax/botulinum"),
		investigation.WithSourceDescription("BND debriefings of Iraqi defector, relayed to DIA"))
	inv.AddEvidence(curveballClaim, "Curveball provided detailed technical drawings of mobile lab layout",
		models.Supports, investigation.WithWeight(1.0))
	inv.AddEvidence(curveballClaim, "Curveball was the sole source for mobile lab claims — no corroboration",
		models.Refutes, investigation.WithWeight(1.5))
	inv.AddEvidence(curveballClaim, "Curveball's supervisor at Djerf al-Nadaf denied any such program existed",
		models.Refutes, investigation.WithWeight(1.3))

	// BND warned about Curveball — a critical meta-claim
	bndWarning := inv.AssertMetaClaim("bnd", curveballClaim, "source-reliability", "unreliable",
		t(2002, 12, 1),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("BND repeatedly warned CIA that Curveball was unreliable, possibly "+
			"mentally unstable, and had a drinking problem. Warnings were ignored"))

	// CIA elevated Curveball's claims anyway
	ciaBio := inv.AssertClaim("cia", prop("mobile-bio-labs", "existence", "confirmed"),
		t(2002, 10, 1), preInvasion,
		investigation.WithClaimNotes("CIA included mobile bio-labs as a key judgment in the 2002 NIE, "+
			"based almost entirely on Curveball's uncorroborated testimony"),
		investigation.WithSourceDescription("2002 NIE Key Judgment"))
	inv.AddEvidence(ciaBio, "Based on single-source (Curveball) reporting via BND",
		models.Supports, investigation.WithWeight(0.5))
	inv.AddEvidence(ciaBio, "No CIA officer ever directly interviewed Curveball before the war",
		models.Refutes, investigation.WithWeight(1.4))

	// CIA's claim supports (builds on) Curveball — causal chain
	inv.AssertMetaClaim("cia", curveballClaim, "assessment", "credible",
		t(2002, 10, 1),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("CIA assessed Curveball's reporting as credible despite BND warnings"))

	// Powell presented Curveball's claims at the UN — another link in the chain
	powellBio := inv.AssertClaim("powell", prop("mobile-bio-labs", "existence", "confirmed"),
		t(2003, 2, 5), preInvasion,
		investigation.WithClaimNotes("Powell showed artists' renderings of mobile bio-labs at UN, stating "+
			"'we have firsthand descriptions.' He was unaware of doubts about Curveball"),
		investigation.WithSourceDescription("UN Security Council presentation, Feb 5, 2003"))
	inv.AddEvidence(powellBio, "Based on CIA briefing materials derived from Curveball",
		models.Supports, investigation.WithWeight(0.8))

	// Powell's claim builds on CIA's analysis
	inv.AssertMetaClaim("powell", ciaBio, "presentation", "endorsed",
		t(2003, 2, 5),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("Powell relied on CIA's assessment for his UN presentation"))

	// INC also pushed mobile lab claims
	incBio := inv.AssertClaim("inc", prop("mobile-bio-labs", "existence", "confirmed"),
		t(2001, 6, 1), preInvasion,
		investigation.WithClaimNotes("INC-sponsored defectors corroborated Curveball's mobile lab claims, "+
			"though later investigation showed coaching"),
		investigation.WithSourceDescription("INC-facilitated defector reporting"))
	inv.AddEvidence(incBio, "Multiple INC defectors described mobile labs",
		models.Supports, investigation.WithWeight(0.6))
	inv.AddEvidence(incBio, "DIA later assessed INC defector reports were fabricated or coached",
		models.Refutes, investigation.WithWeight(1.5))

	// UNMOVIC found no evidence
	inv.AssertClaim("unmovic", prop("mobile-bio-labs", "existence", "unconfirmed"),
		t(2003, 2, 14), inspections,
		investigation.WithClaimNotes("Blix reported to UNSC that inspectors found no mobile bio-labs "+
			"or evidence of mobile bio-weapons production in hundreds of inspections"),
		investigation.WithSourceDescription("UNMOVIC report to UNSC"))

	// Post-invasion ground truth
	inv.AssertFact(prop("mobile-bio-labs", "existence", "fabricated"),
		wmdQuestion,
		investigation.WithClaimNotes("ISG found no mobile bio-weapons labs. Two trailers found in 2003 "+
			"were hydrogen generators for artillery weather balloons, not bio-weapons labs"))

	// Curveball's own admission
	inv.AssertMetaClaim("curveball", curveballClaim, "veracity", "fabricated",
		t(2011, 2, 15),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("Curveball admitted to The Guardian in Feb 2011: 'I had the chance "+
			"to fabricate something to topple the regime.' He confirmed the entire story was a lie"))

	// Meta-claim: BND warning was ignored by CIA — undermines the whole chain
	inv.AssertMetaClaim("isg", bndWarning, "assessment", "warning-was-correct",
		t(2004, 9, 30),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("ISG confirmed BND's warnings about Curveball were accurate all along"))

	// =====================================================
	// THREAD 3: NIGER YELLOWCAKE URANIUM
	// The forged documents
	// =====================================================

	mi6Niger := inv.AssertClaim("mi6", prop("niger-uranium", "acquisition", "attempted"),
		t(2001, 10, 1), preInvasion,
		investigation.WithClaimNotes("MI6 reported Iraq had sought significant quantities of uranium from Africa, "+
			"based partly on Italian intelligence (SISMI) documents"),
		investigation.WithSourceDescription("MI6 reporting via SISMI"))
	inv.AddEvidence(mi6Niger, "Documents purportedly showing Iraq-Niger uranium agreement",
		models.Supports, investigation.WithWeight(0.8))
	inv.AddEvidence(mi6Niger, "Documents were later proven to be crude forgeries",
		models.Refutes, investigation.WithWeight(2.0))

	ciaNiger := inv.AssertClaim("cia", prop("niger-uranium", "acquisition", "attempted"),
		t(2002, 10, 1), wmdQuestion,
		investigation.WithClaimNotes("CIA included the Africa uranium claim in the NIE, and the '16 words' "+
			"appeared in Bush's 2003 State of the Union despite CIA objections to earlier drafts"),
		investigation.WithSourceDescription("2002 NIE; State of the Union Jan 28, 2003"))
	inv.AddEvidence(ciaNiger, "CIA had previously removed the claim from a Bush speech (Oct 2002 Cincinnati) "+
		"because evidence was weak", models.Refutes, investigation.WithWeight(1.2))

	// Wilson investigated and found nothing
	wilsonNiger := inv.AssertClaim("wilson", prop("niger-uranium", "acquisition", "no-evidence"),
		t(2002, 3, 1), wmdQuestion,
		investigation.WithClaimNotes("Wilson traveled to Niger at CIA request, interviewed officials, "+
			"and reported back that the uranium sale claim was baseless"),
		investigation.WithSourceDescription("Wilson trip report, Feb-Mar 2002"))
	inv.AddEvidence(wilsonNiger, "Niger officials denied any uranium transaction with Iraq",
		models.Supports, investigation.WithWeight(1.3))
	inv.AddEvidence(wilsonNiger, "Niger uranium mines under consortium control (French/Japanese); "+
		"covert diversion would be extremely difficult", models.Supports, investigation.WithWeight(1.0))

	// Wilson's finding directly attacks the CIA/MI6 claims
	inv.AssertMetaClaim("wilson", mi6Niger, "accuracy", "unsubstantiated",
		t(2002, 3, 8),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("Wilson reported to CIA that Niger uranium claim was baseless"))

	inv.AssertMetaClaim("wilson", ciaNiger, "accuracy", "unsubstantiated",
		t(2003, 7, 6),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("Wilson published NYT op-ed 'What I Didn't Find in Africa' "+
			"publicly contradicting the 16 words"))

	// IAEA proved the documents were forgeries
	iaeaNiger := inv.AssertClaim("iaea", prop("niger-uranium", "acquisition", "forged-documents"),
		t(2003, 3, 7), inspections,
		investigation.WithClaimNotes("ElBaradei told UNSC the Niger documents were 'not authentic' — "+
			"determined within hours of receiving them using Google and basic analysis"),
		investigation.WithSourceDescription("IAEA report to UN Security Council, Mar 7, 2003"))
	inv.AddEvidence(iaeaNiger, "Wrong letterhead, wrong signatures, wrong dates — crude forgeries",
		models.Supports, investigation.WithWeight(2.0))
	inv.AddEvidence(iaeaNiger, "Named officials had left office years before the purported dates",
		models.Supports, investigation.WithWeight(1.8))

	inv.AssertMetaClaim("iaea", mi6Niger, "document-authenticity", "forged",
		t(2003, 3, 7),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("IAEA definitively proved the Niger documents were forgeries"))

	// Ground truth
	inv.AssertFact(prop("niger-uranium", "acquisition", "no-evidence"),
		wmdQuestion,
		investigation.WithClaimNotes("ISG/Duelfer Report: no evidence Iraq sought uranium from Africa"))

	// =====================================================
	// THREAD 4: NUCLEAR WEAPONS PROGRAM (overall)
	// =====================================================

	ciaNuclear := inv.AssertClaim("cia", prop("iraq-nuclear", "active-program", "reconstituting"),
		t(2002, 10, 1), wmdQuestion,
		investigation.WithClaimNotes("CIA NIE Key Judgment: 'Iraq is reconstituting its nuclear weapons program.' "+
			"Based primarily on aluminum tubes assessment plus other circumstantial evidence"),
		investigation.WithSourceDescription("NIE October 2002, Key Judgments"))
	inv.AddEvidence(ciaNuclear, "Aluminum tubes procurement (disputed — see tubes thread)",
		models.Supports, investigation.WithWeight(0.8))
	inv.AddEvidence(ciaNuclear, "Some dual-use equipment procurement attempts",
		models.Supports, investigation.WithWeight(0.5))
	inv.AddEvidence(ciaNuclear, "Niger uranium claim (later debunked)",
		models.Supports, investigation.WithWeight(0.3))

	diaNuclear := inv.AssertClaim("dia", prop("iraq-nuclear", "active-program", "reconstituting"),
		t(2002, 10, 1), wmdQuestion,
		investigation.WithClaimNotes("DIA concurred with CIA assessment on nuclear reconstitution"),
		investigation.WithSourceDescription("NIE October 2002"))
	inv.AddEvidence(diaNuclear, "Concurred with CIA aluminum tube assessment",
		models.Supports, investigation.WithWeight(0.6))

	// INR's famous dissent
	inrNuclear := inv.AssertClaim("inr", prop("iraq-nuclear", "active-program", "no-compelling-evidence"),
		t(2002, 10, 1), wmdQuestion,
		investigation.WithClaimNotes("INR's dissent: 'The activities we have detected do not add up to a "+
			"compelling case that Iraq is currently pursuing an integrated and comprehensive approach "+
			"to acquire nuclear weapons.' The most important analytical judgment in the entire NIE"),
		investigation.WithSourceDescription("INR dissent, NIE October 2002"))
	inv.AddEvidence(inrNuclear, "Tubes are for rockets, not centrifuges (concurs with DOE)",
		models.Supports, investigation.WithWeight(1.5))
	inv.AddEvidence(inrNuclear, "No evidence of reconstituted nuclear infrastructure",
		models.Supports, investigation.WithWeight(1.3))
	inv.AddEvidence(inrNuclear, "Procurement activities are ambiguous and could be for conventional programs",
		models.Supports, investigation.WithWeight(1.0))

	// INR explicitly attacks the CIA's nuclear conclusion
	inv.AssertMetaClaim("inr", ciaNuclear, "assessment-quality", "lacks-compelling-evidence",
		t(2002, 10, 1),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("INR formally dissented from the NIE's nuclear reconstitution judgment"))

	// IAEA found no evidence of nuclear reconstitution
	iaeaNuclear := inv.AssertClaim("iaea", prop("iraq-nuclear", "active-program", "no-evidence"),
		t(2003, 3, 7), inspections,
		investigation.WithClaimNotes("ElBaradei to UNSC: 'We have to date found no evidence or plausible "+
			"indication of the revival of a nuclear weapons programme in Iraq'"),
		investigation.WithSourceDescription("IAEA report to UNSC, Mar 7, 2003"))
	inv.AddEvidence(iaeaNuclear, "Hundreds of inspections at nuclear-related sites found nothing",
		models.Supports, investigation.WithWeight(2.0))
	inv.AddEvidence(iaeaNuclear, "Environmental sampling showed no enrichment activity",
		models.Supports, investigation.WithWeight(1.8))

	inv.AssertMetaClaim("iaea", ciaNuclear, "accuracy", "contradicted-by-inspections",
		t(2003, 3, 7),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("IAEA's on-the-ground inspections directly contradicted CIA's nuclear assessment"))

	// Ground truth
	inv.AssertFact(prop("iraq-nuclear", "active-program", "no-program"),
		wmdQuestion,
		investigation.WithClaimNotes("ISG/Duelfer Report: Iraq's nuclear program had been dismantled in 1991 "+
			"and never reconstituted. Saddam retained the 'intention' but had no active program"))

	// =====================================================
	// THREAD 5: CHEMICAL WEAPONS
	// =====================================================

	ciaChem := inv.AssertClaim("cia", prop("iraq-chem", "stockpiles", "100-500-tonnes"),
		t(2002, 10, 1), wmdQuestion,
		investigation.WithClaimNotes("NIE: Iraq has 'chemical weapons stockpiles' of 100-500 tonnes of CW agents"),
		investigation.WithSourceDescription("NIE October 2002"))
	inv.AddEvidence(ciaChem, "Iraq had used CW in 1980s (Iran-Iraq war, Halabja)",
		models.Supports, investigation.WithWeight(0.8),
		investigation.WithEvidenceNotes("Historical use is not evidence of current stockpiles"))
	inv.AddEvidence(ciaChem, "UNSCOM destroyed large CW stockpiles in 1990s; discrepancy in accounting",
		models.Supports, investigation.WithWeight(0.6))
	inv.AddEvidence(ciaChem, "No satellite evidence of CW production facility activity",
		models.Refutes, investigation.WithWeight(1.0))

	unmovicChem := inv.AssertClaim("unmovic", prop("iraq-chem", "stockpiles", "no-evidence-found"),
		t(2003, 2, 14), inspections,
		investigation.WithClaimNotes("Blix to UNSC: inspectors found no chemical weapons in hundreds of inspections; "+
			"some discrepancies in Iraqi declarations but no prohibited items discovered"),
		investigation.WithSourceDescription("UNMOVIC report to UNSC"))
	inv.AddEvidence(unmovicChem, "550+ inspections at 350 sites found no CW stockpiles",
		models.Supports, investigation.WithWeight(1.8))

	inv.AssertMetaClaim("unmovic", ciaChem, "accuracy", "not-corroborated-by-inspections",
		t(2003, 2, 14),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("UNMOVIC inspections failed to find the stockpiles CIA predicted"))

	// Ground truth
	inv.AssertFact(prop("iraq-chem", "stockpiles", "no-stockpiles"),
		wmdQuestion,
		investigation.WithClaimNotes("ISG: No CW stockpiles found. Only degraded, unusable pre-1991 munitions "+
			"buried and forgotten. Iraq had destroyed its CW stocks in the 1990s"))

	// =====================================================
	// THREAD 6: IRAQ–AL QAEDA LINK
	// =====================================================

	ciaAQ := inv.AssertClaim("cia", prop("iraq-aq", "operational-link", "contacts-exist"),
		t(2002, 6, 1), preInvasion,
		investigation.WithClaimNotes("CIA assessed 'credible reporting' of senior-level contacts between Iraq and AQ, "+
			"but noted 'we have no reliable reporting on whether Iraq is complicit in AQ operations'"),
		investigation.WithSourceDescription("CIA reporting, 2002"))
	inv.AddEvidence(ciaAQ, "Reports of AQ operatives transiting Iraq",
		models.Supports, investigation.WithWeight(0.5))
	inv.AddEvidence(ciaAQ, "Alleged Zarqawi presence in northeastern Iraq (Kurdish area outside Baghdad control)",
		models.Supports, investigation.WithWeight(0.4))
	inv.AddEvidence(ciaAQ, "No evidence of Iraqi government direction or control of AQ activities",
		models.Refutes, investigation.WithWeight(1.2))

	incAQ := inv.AssertClaim("inc", prop("iraq-aq", "operational-link", "active-collaboration"),
		t(2002, 1, 1), preInvasion,
		investigation.WithClaimNotes("INC-sponsored sources claimed operational collaboration between Iraq and AQ"),
		investigation.WithSourceDescription("INC defector reporting"))
	inv.AddEvidence(incAQ, "INC defectors described training camps",
		models.Supports, investigation.WithWeight(0.4))
	inv.AddEvidence(incAQ, "DIA later assessed INC source reporting on AQ as 'fabrication'",
		models.Refutes, investigation.WithWeight(1.6))

	diaAQ := inv.AssertClaim("dia", prop("iraq-aq", "operational-link", "evidence-insufficient"),
		t(2002, 7, 1), preInvasion,
		investigation.WithClaimNotes("DIA assessed the evidence for an operational Iraq-AQ link was insufficient. "+
			"Notably assessed Ibn al-Shaykh al-Libi's claims (made under torture) as unreliable"),
		investigation.WithSourceDescription("DIA assessment, 2002"))
	inv.AddEvidence(diaAQ, "Al-Libi's claims of Iraq-AQ CW training were obtained under coercive interrogation",
		models.Supports, investigation.WithWeight(1.3))
	inv.AddEvidence(diaAQ, "Al-Libi later recanted his statements",
		models.Supports, investigation.WithWeight(1.5))

	// DIA's assessment attacks INC's claim
	inv.AssertMetaClaim("dia", incAQ, "source-quality", "fabricated",
		t(2002, 7, 1),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("DIA assessed INC reporting on Iraq-AQ as fabrication"))

	// Ground truth
	inv.AssertFact(prop("iraq-aq", "operational-link", "no-collaborative-relationship"),
		wmdQuestion,
		investigation.WithClaimNotes("9/11 Commission (2004) and Senate Intelligence Committee: "+
			"'no collaborative relationship' between Iraq and AQ"))

	// =====================================================
	// THREAD 7: BIOLOGICAL WEAPONS PROGRAM (overall)
	// =====================================================

	ciaBioOverall := inv.AssertClaim("cia", prop("iraq-bio", "active-program", "active"),
		t(2002, 10, 1), wmdQuestion,
		investigation.WithClaimNotes("NIE: Iraq 'has' biological weapons and an active BW program. "+
			"Based heavily on Curveball and INC source reporting"),
		investigation.WithSourceDescription("NIE October 2002"))
	inv.AddEvidence(ciaBioOverall, "Iraq had past BW program (pre-1991), undeclared to UNSCOM until 1995",
		models.Supports, investigation.WithWeight(0.7))
	inv.AddEvidence(ciaBioOverall, "Mobile bio-lab claims from Curveball (single source, disputed)",
		models.Supports, investigation.WithWeight(0.4))

	// CIA's bio assessment builds on Curveball's claims
	inv.AssertMetaClaim("cia", curveballClaim, "sourcing", "key-source-for-bio-assessment",
		t(2002, 10, 1),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("Curveball was the primary basis for the BW program assessment"))

	unmovicBio := inv.AssertClaim("unmovic", prop("iraq-bio", "active-program", "no-evidence"),
		t(2003, 2, 14), inspections,
		investigation.WithClaimNotes("UNMOVIC found no evidence of active BW program during inspections"),
		investigation.WithSourceDescription("UNMOVIC reports to UNSC, 2003"))
	inv.AddEvidence(unmovicBio, "Inspections of declared and suspected BW sites found no prohibited activity",
		models.Supports, investigation.WithWeight(1.6))

	inv.AssertMetaClaim("unmovic", ciaBioOverall, "accuracy", "not-corroborated",
		t(2003, 2, 14),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("UNMOVIC inspections contradicted CIA's BW assessment"))

	// Ground truth
	inv.AssertFact(prop("iraq-bio", "active-program", "no-program"),
		wmdQuestion,
		investigation.WithClaimNotes("ISG/Duelfer: Iraq destroyed its BW program in 1995-96. "+
			"No evidence of any BW agent production after 1991"))

	// =====================================================
	// CROSS-CUTTING: Meta-claim on NIE quality
	// =====================================================

	// ISG's post-invasion findings serve as meta-critique of the entire NIE
	inv.AssertMetaClaim("isg", ciaNuclear, "post-invasion-assessment", "wrong",
		t(2004, 9, 30),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("Duelfer Report found all major NIE nuclear judgments were incorrect"))

	inv.AssertMetaClaim("isg", ciaBio, "post-invasion-assessment", "wrong",
		t(2004, 9, 30),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("Duelfer Report found all major NIE biological judgments were incorrect"))

	inv.AssertMetaClaim("isg", ciaChem, "post-invasion-assessment", "wrong",
		t(2004, 9, 30),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("Duelfer Report found all major NIE chemical judgments were incorrect"))

	// INR was vindicated
	inv.AssertMetaClaim("isg", inrNuclear, "post-invasion-assessment", "vindicated",
		t(2004, 9, 30),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("ISG findings matched INR's dissent almost exactly — "+
			"the smallest IC agency was the most accurate"))

	// =====================================================
	// CROSS-THREAD LINKS: How evidence fed into assessments
	// These create the connective tissue between subject clusters
	// =====================================================

	// --- Evidence → Nuclear program assessment ---
	// The CIA's nuclear "reconstituting" judgment rested on tubes + Niger
	inv.AssertMetaClaim("cia", ciaTubes, "evidential-basis", "key-evidence-for-nuclear",
		t(2002, 10, 1),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("CIA cited aluminum tubes as primary physical evidence for nuclear reconstitution"))

	inv.AssertMetaClaim("cia", ciaNiger, "evidential-basis", "supporting-evidence-for-nuclear",
		t(2002, 10, 1),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("Niger uranium claim was cited as corroborating evidence for nuclear intent"))

	// Debunking tubes/Niger weakens the nuclear case
	inv.AssertMetaClaim("doe", doeTubes, "implication", "undermines-nuclear-case",
		t(2001, 9, 1),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("If tubes are for rockets, the primary physical evidence for nuclear reconstitution collapses"))

	inv.AssertMetaClaim("iaea", iaeaTubes, "implication", "undermines-nuclear-case",
		t(2003, 1, 27),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("IAEA's definitive tubes finding removed the centerpiece of the nuclear argument"))

	inv.AssertMetaClaim("iaea", iaeaNiger, "implication", "undermines-nuclear-case",
		t(2003, 3, 7),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("Forged Niger documents removed the uranium acquisition pillar of the nuclear case"))

	// INR's tubes dissent supports INR's nuclear dissent
	inv.AssertMetaClaim("inr", inrTubes, "evidential-basis", "supports-nuclear-dissent",
		t(2002, 10, 1),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("INR's tube assessment was the foundation for their nuclear program dissent"))

	// --- Evidence → Bio program assessment ---
	// Curveball's mobile labs → CIA's overall bio assessment
	inv.AssertMetaClaim("cia", ciaBio, "evidential-basis", "mobile-labs-prove-bio-program",
		t(2002, 10, 1),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("CIA's mobile bio-lab claim was the key pillar of the overall BW assessment"))

	// INC reporting also fed the bio assessment
	inv.AssertMetaClaim("cia", incBio, "evidential-basis", "corroborating-bio-evidence",
		t(2002, 10, 1),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("INC defector reporting was used to corroborate Curveball"))

	// --- Powell's UN presentation tied ALL threads together ---
	// Powell cited evidence from nuclear, bio, chem, and AQ threads
	powellNuclear := inv.AssertClaim("powell", prop("iraq-nuclear", "active-program", "reconstituting"),
		t(2003, 2, 5), preInvasion,
		investigation.WithClaimNotes("Powell presented the nuclear case at the UN, featuring aluminum tubes "+
			"and citing the Niger intelligence"),
		investigation.WithSourceDescription("UN Security Council presentation, Feb 5, 2003"))

	powellChem := inv.AssertClaim("powell", prop("iraq-chem", "stockpiles", "confirmed"),
		t(2003, 2, 5), preInvasion,
		investigation.WithClaimNotes("Powell showed satellite imagery of alleged CW sites at the UN, "+
			"claiming Iraq had 'between 100 and 500 tonnes of chemical weapons agent'"),
		investigation.WithSourceDescription("UN Security Council presentation, Feb 5, 2003"))

	powellAQ := inv.AssertClaim("powell", prop("iraq-aq", "operational-link", "sinister-nexus"),
		t(2003, 2, 5), preInvasion,
		investigation.WithClaimNotes("Powell described a 'sinister nexus' between Iraq and al-Qaeda, "+
			"citing Zarqawi's presence and alleged CW training"),
		investigation.WithSourceDescription("UN Security Council presentation, Feb 5, 2003"))

	// Powell's claims built on CIA assessments (support chain)
	inv.AssertMetaClaim("powell", ciaNuclear, "presentation", "endorsed",
		t(2003, 2, 5),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("Powell presented CIA's nuclear assessment to the UN"))

	inv.AssertMetaClaim("powell", ciaChem, "presentation", "endorsed",
		t(2003, 2, 5),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("Powell presented CIA's chemical weapons assessment to the UN"))

	inv.AssertMetaClaim("powell", ciaAQ, "presentation", "endorsed",
		t(2003, 2, 5),
		investigation.WithValence(models.Supports),
		investigation.WithClaimNotes("Powell presented CIA's Iraq-AQ assessment to the UN"))

	// --- DIA's AQ skepticism links to CIA's AQ claim ---
	inv.AssertMetaClaim("dia", ciaAQ, "assessment-quality", "evidence-insufficient",
		t(2002, 7, 1),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("DIA assessed the evidence for an operational Iraq-AQ link was insufficient"))

	// --- Wilson's Niger finding attacks CIA's nuclear case indirectly ---
	inv.AssertMetaClaim("wilson", ciaNuclear, "evidential-basis", "niger-pillar-removed",
		t(2003, 7, 6),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("Wilson's debunking of Niger claim removed a key pillar "+
			"of the overall nuclear reconstitution argument"))

	// --- IAEA's nuclear finding attacks CIA's nuclear case ---
	inv.AssertMetaClaim("iaea", ciaNuclear, "inspections", "no-nuclear-evidence",
		t(2003, 3, 7),
		investigation.WithValence(models.Refutes),
		investigation.WithClaimNotes("IAEA told the UNSC it found no evidence of nuclear reconstitution — "+
			"a direct rebuttal of the NIE's central nuclear judgment"))

	_ = diaNuclear
	_ = iaeaNuclear
	_ = wilsonNiger
	_ = unmovicChem
	_ = unmovicBio
	_ = diaAQ
	_ = powellNuclear
	_ = powellChem
	_ = powellAQ

	return inv
}
