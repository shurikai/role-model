// Package fitgate implements the deterministic fit gate and scoring pass
// that runs between Stage 1 (JD signal extraction) and Stage 2 (resume
// generation). Nothing in this file talks to the database or an LLM — it is
// pure scoring logic over data the caller assembles.
package fitgate

import (
	"slices"
	"strings"
	"unicode"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
	"github.com/shurikai/role-model/internal/vocabulary"
)

// JDSignals is the structured output of Stage 1 extraction.
type JDSignals = generation.JDSignals

// alternativesDelimiter separates interchangeable alternatives inside a single
// required_skills or preferred_skills entry. Extraction emits one entry per
// requirement, so "Spring Boot, Quarkus, Micronaut, or Vert.x" — one
// requirement satisfiable four ways — arrives as
// "Spring Boot | Quarkus | Micronaut | Vert.x" rather than four entries.
//
// Flattening those into separate entries scored each one independently:
// holding Spring Boot earned a quarter of the points and reported the other
// three as gaps, for a requirement that was fully met. The delimiter is " | "
// and not "/" because real skill names contain slashes (CI/CD, TCP/IP).
const alternativesDelimiter = " | "

// splitAlternatives returns the interchangeable alternatives in a skills
// entry. An entry with no delimiter is a set of one. Blank alternatives are
// dropped, so a malformed entry yields none — which reads as unsatisfied
// rather than matching every skill, as an empty needle otherwise would.
func splitAlternatives(entry string) []string {
	var out []string
	for _, part := range strings.Split(entry, alternativesDelimiter) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// collapseSubsumed drops any entry whose alternatives are a strict subset of
// another entry's, returning the survivors in their original order.
// ["Java | Python", "Python", "Java"] collapses to ["Java | Python"].
//
// Postings routinely state the same stack twice — once in the must-have
// qualifications as a choice ("Proficiency in Java and/or Python") and again in
// a "Core Technical Stack" list as a flat enumeration ("Java and Python") — and
// the two statements disagree about whether either language is actually
// required. The stricter statement is the group: it says the role can be done
// without any one member. A bare restatement of a member is the looser claim
// and loses.
//
// This exists as a Go rule rather than a prompt instruction on purpose.
// jd_extraction.tmpl asks for the same deduplication, but a prompt is a request
// and this is the exact hazard that made #68 a hard gate misfire rather than a
// rounding error. Extraction noise here must be harmless, not merely unlikely.
func collapseSubsumed(entries []string) []string {
	groups := make([]map[string]bool, len(entries))
	for i, e := range entries {
		g := make(map[string]bool)
		for _, alt := range splitAlternatives(e) {
			g[strings.ToLower(alt)] = true
		}
		groups[i] = g
	}

	var out []string
	for i := range entries {
		if len(groups[i]) == 0 {
			continue
		}
		subsumed := false
		for j := range entries {
			// Strict subset only. Equal groups subsume each other, which would
			// drop both and lose the entry entirely.
			if i != j && len(groups[i]) < len(groups[j]) && isSubset(groups[i], groups[j]) {
				subsumed = true
				break
			}
		}
		if !subsumed {
			out = append(out, entries[i])
		}
	}
	return out
}

func isSubset(a, b map[string]bool) bool {
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// SkillTerm is one active skill as the matcher sees it: the canonical name,
// the synonyms a JD might use instead, and the category it belongs to along
// with that category's competency vocabulary.
//
// It exists because matching against the name alone loses two different
// things. Aliases lose a spelling difference ("Golang" against a stored
// "Go"). The category loses a whole level of abstraction: a JD that asks for
// "CI/CD" is asking about Jenkins and GitHub Actions without naming either.
type SkillTerm struct {
	Name            string
	Aliases         []string
	Category        string
	CategoryAliases []string

	// Proficiency is the recorded depth — novice, proficient, or expert. It is
	// compared against a level the JD asked for, and only when the JD asked
	// for one; see ScoreCapabilityFit. years_experience is deliberately not
	// carried alongside it, because nothing compares against a number.
	Proficiency string
}

// MatchKind records how a requirement came to be satisfied. It is carried
// into the report so a reader can tell strong evidence from weak: a direct
// name match is the skill itself, while a category match means the JD asked
// for a capability and the user holds tools within it.
type MatchKind string

const (
	MatchDirect   MatchKind = "direct"
	MatchAlias    MatchKind = "alias"
	MatchCategory MatchKind = "category"
)

// MatchOrigin records WHICH LIST the requirement came from. It is a third axis
// alongside Kind (how the evidence was found) and the level fields (whether the
// evidence clears the bar stated for it).
//
// The plan for this called for a fourth MatchKind value, "competency". That
// would have collapsed two questions into one enum for the third time in this
// codebase — the same mistake as folding partial into Kind, and the same one
// work_type made by naming two different things. A competency can be answered
// directly, by alias, or by category, exactly as a named skill can, and a
// reader wants both facts.
type MatchOrigin string

const (
	OriginRequired   MatchOrigin = "required"
	OriginPreferred  MatchOrigin = "preferred"
	OriginCompetency MatchOrigin = "competency"
)

// SkillMatch is one satisfied requirement and the evidence behind it.
//
// # Kind and level are two different axes, and collapsing them loses one
//
// Kind says HOW the evidence was found — the skill's own name, a recorded
// synonym, or the category it sits in. The level fields say WHETHER the depth
// found meets the depth the JD asked for. A match can be direct and still
// short of the bar: holding a skill named exactly right, at novice, against a
// posting that wants an expert. Folding "partial" into Kind as a fourth value
// would make that match pick one fact to report and discard the other.
//
// # The level fields are empty when no level was assessed, and that is not "unmet"
//
// RequiredLevel is empty exactly when the JD stated no depth for this
// requirement, which is the common case. It is NOT a bool for the same reason
// CapabilityFit.Scored is not a sentinel score: "the posting asked for expert
// and the profile is proficient" and "the posting asked for nothing" are
// opposite findings, and a false would read as the former while meaning the
// latter. Carrying both levels also makes the comparison auditable — a reader
// sees what was asked and what answered it, rather than a verdict to trust.
type SkillMatch struct {
	Requirement string    `json:"requirement"`
	Kind        MatchKind `json:"kind"`
	Category    string    `json:"category,omitempty"`
	Evidence    []string  `json:"evidence"`

	// Origin is which list the requirement came from. Empty on matches
	// recorded before it existed, which reads as "a named skill" — the only
	// thing that was scored then.
	Origin MatchOrigin `json:"origin,omitempty"`

	// RequiredLevel is the depth the JD asked for, empty when it asked for
	// none. EvidenceLevel is the strongest proficiency behind the match, and
	// is populated only alongside RequiredLevel — absent a stated requirement
	// there is nothing to report it against. LevelSignal quotes the JD
	// language the requirement was read from.
	RequiredLevel string `json:"required_level,omitempty"`
	EvidenceLevel string `json:"evidence_level,omitempty"`
	LevelSignal   string `json:"level_signal,omitempty"`
}

// satisfies reports how skills cover the requirement in entry, and returns
// the evidence for it. An entry may offer several interchangeable
// alternatives; matching any one satisfies the whole entry.
//
// The three kinds are tried in order of strength across every alternative,
// rather than settling for the first alternative that matches somehow. A JD
// entry of "Kubernetes | container orchestration" should report the stored
// Kubernetes skill, not a category match on the phrase beside it.
func satisfies(skills []SkillTerm, entry string) (kind MatchKind, category string, evidence []SkillTerm, ok bool) {
	alts := splitAlternatives(entry)

	// Strongest: the skill's own name answers the requirement.
	for _, alt := range alts {
		for _, s := range skills {
			if matchesTerm([]string{s.Name}, alt) {
				evidence = appendUnique(evidence, s)
			}
		}
	}
	if len(evidence) > 0 {
		return MatchDirect, "", evidence, true
	}

	// Next: a recorded synonym of the skill answers it.
	for _, alt := range alts {
		for _, s := range skills {
			if matchesTerm(s.Aliases, alt) {
				evidence = appendUnique(evidence, s)
			}
		}
	}
	if len(evidence) > 0 {
		return MatchAlias, "", evidence, true
	}

	// Weakest, and the one that rescues competency-worded JDs: the
	// requirement names a capability that one of the user's categories
	// covers. Evidence is every active skill in that category — the category
	// is only reachable here because at least one such skill exists, since
	// the skill list is what these categories were built from.
	for _, alt := range alts {
		for _, s := range skills {
			if s.Category == "" {
				continue
			}
			if !matchesTerm(append([]string{s.Category}, s.CategoryAliases...), alt) {
				continue
			}
			category = s.Category
			for _, peer := range skills {
				if peer.Category == category {
					evidence = appendUnique(evidence, peer)
				}
			}
			return MatchCategory, category, evidence, true
		}
	}

	return "", "", nil, false
}

// appendUnique appends v to out if it isn't already there, keeping evidence
// lists stable and free of repeats when several alternatives hit the same
// skill. Identity is the skill name — the same skill reached twice through
// two alternatives is one piece of evidence.
func appendUnique(out []SkillTerm, v SkillTerm) []SkillTerm {
	for _, s := range out {
		if s.Name == v.Name {
			return out
		}
	}
	return append(out, v)
}

// evidenceNames reduces matched skills to the names a SkillMatch records. The
// match carries names rather than whole terms because that is what the report
// stores and what the narrative cites.
func evidenceNames(evidence []SkillTerm) []string {
	out := make([]string, 0, len(evidence))
	for _, s := range evidence {
		out = append(out, s.Name)
	}
	return out
}

// ScoreCapabilityFit scores how well the user's skills cover the JD's required
// and preferred skills. Required matches are worth twice a preferred match.
// Gaps list required skills nothing answered; matches record the satisfied
// requirements and the evidence for each.
//
// Each entry counts once toward the total no matter how many alternatives it
// offers, and an unmet entry is reported as a single gap holding the whole
// original string — one requirement, one gap.
//
// A category match earns the same points as a direct one. Five CI/CD tools
// really is evidence of CI/CD, and a fractional constant would need defending
// against every future JD. The distinction is preserved in the match kind,
// where a reader can weigh it, rather than baked into the number.
//
// # Partial matches
//
// A requirement the JD attached a depth to — "expert-level Kafka", "5+ years
// of Go" — is scored against the strongest proficiency behind its evidence.
// Falling short earns half credit (1 of 2 for a required skill, 0.5 of 1 for a
// preferred one) and files the requirement under Partial rather than Matches.
//
// Three properties hold this together, and each is easy to break:
//
//   - **No stated level means no change.** signals.SkillLevels is sparse by
//     design — most requirements carry no entry — and a requirement without one
//     takes the original path and the original full credit. A JD carrying no
//     skill_levels at all must score byte-for-byte what it scored before this
//     existed, which is what makes the feature additive rather than a
//     rescoring of every stored application.
//   - **pointsPossible does not move.** Each entry still counts once at its
//     full required or preferred weight. Discounting the denominator too would
//     cancel the penalty out and make a partial match score identically to a
//     full one.
//   - **Level is only consulted once presence is established.** A requirement
//     nothing answers is a plain gap whether or not the JD stated a depth for
//     it, and the gap records the JD's phrasing exactly as before. "You do not
//     have this" and "you have this but not deeply enough" are different
//     findings, and a gap that started reporting the level would blur them.
//
// Partial is its own list for the same reason gaps and conflicts are separate
// on the preference axis: a partial match is real evidence with a real caveat.
// Filing it under Matches loses the caveat and filing it under Gaps loses the
// evidence, and the narrative would then have to re-derive a distinction the
// scorer already made.
//
// Depth still does nothing where the JD asks for nothing, which is most of the
// time — the profile-side half of #44 (a novice match and a twenty-year match
// scoring identically on an unqualified requirement) is untouched by this.
//
// # The unscored case
//
// A JD that states no technical requirements at all yields Scored: false, and
// Score is then meaningless and must not be read. It used to return 100 — a
// perfect score with no matches and no evidence, which the narrative wrote
// confident coverage prose around ("complete coverage across backend
// engineering, distributed systems, payment platforms" — none of it scored).
//
// "This profile answers none of the requirements" and "this JD stated no
// requirements to answer" are opposite findings and cannot share a
// representation. Scored is what separates them, and returning a struct is
// what makes the confusion unrepresentable rather than merely discouraged.
//
// It is much rarer than it was. core_competencies now scores as a third input
// (below), so a posting has to name no requirement of any shape to reach it.
//
// # Competencies are the third scored input
//
// core_competencies holds the capability-level asks a posting states in prose —
// "decomposing a legacy service", "production ownership of services", "triage
// under surge conditions". Nothing scored them, which was a degradation for a
// staff software posting and total failure for a competency-shaped career: a
// nursing, teaching, or trades JD may name no "skill" at all, so every
// evaluation returned Scored: false with an empty gap list, every time (#72).
//
// They score at preferred weight through the same three layers, and three
// rules hold it together:
//
//   - **pointsPossible grows to include them.** Unlike the partial-match rule,
//     these are genuine additional asks: a posting requiring "production
//     ownership" is asking for something, and a profile that answers none of it
//     should not score as though the posting were silent.
//   - **A competency already restated as a skill is not counted twice.**
//     jd_extraction.tmpl forbids the duplication, and a prompt is a request —
//     dedupeCompetencies backs it in Go the way collapseSubsumed backs the
//     deduplication rule it mirrors.
//   - **An unanswered competency is not a gap.** Gaps are the JD's named
//     requirements, and a competency is prose. Reporting "production ownership
//     of services" among the technical gaps reads as a missing tool. It costs
//     its point and is otherwise silent, which is exactly how an unmet
//     preferred skill behaves.
//
// Matched competencies carry Origin: competency so a reader can tell "answered
// a named requirement" from "answered a stated capability" — a distinction the
// narrative needs and Kind cannot carry, because a competency can be answered
// directly, by alias, or by category like anything else.
func ScoreCapabilityFit(skills []SkillTerm, signals JDSignals, levels LevelScale) CapabilityFit {
	competencies := dedupeCompetencies(signals.CoreCompetencies, signals.RequiredSkills, signals.PreferredSkills)

	pointsPossible := float64(len(signals.RequiredSkills)*2 + len(signals.PreferredSkills) + len(competencies))
	if pointsPossible == 0 {
		return CapabilityFit{Scored: false}
	}

	var (
		pointsEarned float64
		gaps         []string
		matches      []SkillMatch
		partial      []SkillMatch
	)

	// score awards one entry its points and files it, reporting whether
	// anything answered it at all. Full points when it is satisfied at or
	// above any level the JD asked for, half when it is satisfied but short of
	// that level.
	score := func(entry string, full float64, origin MatchOrigin) (matched bool) {
		kind, category, evidence, ok := satisfies(skills, entry)
		if !ok {
			return false
		}
		match := SkillMatch{
			Requirement: entry, Kind: kind, Category: category,
			Evidence: evidenceNames(evidence), Origin: origin,
		}

		required, signal := requiredLevelFor(entry, signals.SkillLevels, levels)
		if required == "" {
			// The JD stated no depth here. Original path, original credit.
			pointsEarned += full
			matches = append(matches, match)
			return true
		}

		held := strongestProficiency(evidence, levels)
		match.RequiredLevel = required
		match.EvidenceLevel = held
		match.LevelSignal = signal

		if levels.Rank(held) >= levels.Rank(required) {
			pointsEarned += full
			matches = append(matches, match)
			return true
		}
		pointsEarned += full / 2
		partial = append(partial, match)
		return true
	}

	for _, req := range signals.RequiredSkills {
		// A gap records the JD's phrasing and nothing else, level stated or
		// not. Level only means something once presence is established.
		if !score(req, 2, OriginRequired) {
			gaps = append(gaps, req)
		}
	}
	for _, pref := range signals.PreferredSkills {
		score(pref, 1, OriginPreferred)
	}
	for _, comp := range competencies {
		// Deliberately not appended to gaps when unmatched — see the doc
		// comment. It costs its point and says nothing, like a preferred skill.
		score(comp, 1, OriginCompetency)
	}

	return CapabilityFit{
		Score:   clampScore(pointsEarned / pointsPossible * 100),
		Scored:  true,
		Gaps:    gaps,
		Matches: matches,
		Partial: partial,
	}
}

// dedupeCompetencies drops competencies the posting already stated as a named
// skill, so one requirement is not scored twice.
//
// jd_extraction.tmpl asks for exactly this ("do not duplicate required_skills")
// and a prompt is a request. This is the Go backstop, the same relationship
// collapseSubsumed has to the deduplication rule it mirrors: extraction noise
// here should be harmless rather than merely unlikely.
//
// The comparison goes through the same whole-word phrase matcher the scoring
// does, in both directions, so "production ownership of services" is dropped
// against a required "production ownership" and vice versa. A competency that
// merely shares a word with a skill survives — "observability" against "Splunk"
// is two different asks, and dropping it would silently discount the posting.
func dedupeCompetencies(competencies []string, skillLists ...[]string) []string {
	var named []string
	for _, list := range skillLists {
		for _, entry := range list {
			named = append(named, splitAlternatives(entry)...)
		}
	}

	out := make([]string, 0, len(competencies))
	for _, comp := range competencies {
		if strings.TrimSpace(comp) == "" {
			continue
		}
		if matchesSignal(comp, named) {
			continue
		}
		out = append(out, comp)
	}
	return out
}

// CapabilityFit is the outcome of technical scoring.
//
// Scored is false when the JD stated no technical requirements at all. Score
// carries no meaning in that case and callers must not display, store, or
// narrate it — see the ScoreCapabilityFit doc comment for why this is a field
// rather than a magic value.
type CapabilityFit struct {
	Score   float64
	Scored  bool
	Gaps    []string
	Matches []SkillMatch

	// Partial holds requirements the profile answers but not at the depth the
	// JD asked for. Parallel to Matches and Gaps, never merged into either —
	// see ScoreCapabilityFit. Always empty for a JD whose signals carry no
	// skill_levels, which is most of them.
	Partial []SkillMatch
}

// ScorePreferenceFit reports how a JD lines up with the user's preferences.
// It returns four lists and no number, deliberately.
//
// It used to return a 0-100 float built from a normalized weighted average, a
// raw-points penalty for matched hard gates, and a ceiling clamped over the
// top. Nearly every preference-scoring defect this package has carried came
// out of that arithmetic rather than out of the matching: unmatched negatives
// paid a bonus because an absent dislike earned its weight, gate hits read as
// a ten-point dip until a ceiling was bolted on, and a gates-only profile
// scored a perfect 100 through the empty-average short-circuit. Each fix
// added another interacting term to a number nobody could read back to the
// rows that produced it.
//
// A preference is a fact about one row matching or not matching one JD, and
// that is what this returns. What a reader needs is which preferences the
// posting answers, which it is silent about, and which it runs against —
// every one of them nameable. Anything that looks like an at-a-glance summary
// is a presentation concern and belongs above this function, not inside it.
//
// The four lists:
//
//   - matches: positive preferences the JD answers.
//   - gaps: positive preferences it doesn't mention. Unmentioned is not
//     opposed — the JD is silent on these, and calling them conflicts is how
//     false conflict language gets written. That distinction is why gaps and
//     conflicts are separate lists and must stay separate.
//   - conflicts: matched negative preferences that are not hard gates — the
//     JD actively signals something the user dislikes.
//   - gateHits: matched hard-gate preferences, the rows marked disqualifying
//     rather than merely disliked.
//
// gateHits stays structurally apart from conflicts rather than folding in as
// a severity flag on one list. A disqualifying match and an ordinary matched
// negative are different kinds of finding, not different weights of the same
// one, and the narrative has to be able to say so. Note that a gate hit is no
// longer also copied into conflicts as it was when both fed one score; a
// caller wanting every objection reads both lists.
//
// An unmatched negative — hard gate or not — is reported nowhere. Avoiding a
// stated dislike is the ideal outcome and there is nothing notable to say
// about the absence of an absence. This is also the behavior that made the
// old average pay it out as a bonus; with no average, it simply costs and
// earns nothing.
//
// Gating still does not block. A JD that trips a gate is scored, narrated,
// and generated; the trip is a finding, not a stop.
func ScorePreferenceFit(prefs []db.Preference, signals JDSignals) (
	matches []db.Preference, gaps []db.Preference, conflicts []db.Preference, gateHits []db.Preference,
) {
	for _, p := range prefs {
		matched := preferenceAnswered(p, signals)

		if p.IsHardGate {
			if matched {
				gateHits = append(gateHits, p)
			}
			continue
		}

		switch p.Sentiment {
		case "positive":
			if matched {
				matches = append(matches, p)
			} else {
				gaps = append(gaps, p)
			}
		case "negative":
			if matched {
				conflicts = append(conflicts, p)
			}
		}
	}
	return matches, gaps, conflicts, gateHits
}

// prefFieldsFor returns the JD signal fields a preference of the given type
// should be compared against.
//
// This is the single matcher for every preference, gate or not. There used to
// be two: a broad signalFields for scoring and this routed one for the gate.
// The split was invisible and consequential — scoring never saw the skills
// arrays, so a weighted negative naming a technology could not fire no matter
// what the JD required, and because an unmatched negative earns its weight it
// silently paid out a bonus on every evaluation instead. Two matchers over one
// set of rows is what let that hide; there is now one.
//
// Routing by type is what stops a label colliding with an unrelated field.
// Matching every preference against every field meant "IT consulting / staff
// augmentation model" — a business-model exclude — was compared against the
// seniority field, and since "staff" is a whole word inside that label, every
// staff-level JD tripped it. Word-boundary matching does not help here: the
// overlap is a genuine whole word, just one that means something different in
// each place. Only routing fixes it.
//
// Seniority is deliberately absent from every branch. No preference type
// describes a seniority level, so nothing should be matched against it. Add a
// case if that ever changes.
//
// # core_practice vs dealbreaker
//
// These two branches ask different questions of the same JD, and the split is
// what closes #68. dealbreaker asks "does the JD demand this at all", which is
// the right question for a bare label like "C# / .NET" — presence is the whole
// claim. core_practice asks "is the role practised in this", which is the right
// question for a label that makes a claim about prominence: "Python as a
// primary language", "Angular as co-equal frontend requirement".
//
// Routing the second kind at required_skills is what produced the defect. The
// qualifier in such a label has no counterpart in required_skills, so it was
// inert text, and matchesSignal's field-inside-label direction matched the bare
// token instead: containsPhrase("Python", "expert Python as primary
// requirement") is true, because "python" is a whole word in the label. A
// posting whose must-haves read "Proficiency in Java and/or Python" therefore
// tripped a hard gate and was narrated as gating on expert Python. Every
// posting naming Python as a required skill did. (The trip also capped the
// preference score, back when there was one — the misrouting is the defect,
// and it outlived the arithmetic that made it loud.)
//
// Bidirectional matching is safe again in the core_practice branch precisely
// because the field is already filtered to prominence upstream. Routing does
// the work, the same way it did for the staff/seniority collision.
//
// preferred_skills is deliberately absent too. A preference is a statement
// about what the job actually demands, so it should fire on a genuine
// requirement and not on an optional mention. The Angular exclude — "Angular
// as co-equal frontend requirement" — tripped on a JD whose only Angular
// reference was a nice-to-have bullet ("exposure to front-end technologies
// such as React or Angular"), and the narrative then described Angular as a
// co-equal requirement the JD never made it out to be. preferred_skills still
// feeds ScoreCapabilityFit normally; it is only preference matching that
// ignores it.
func prefFieldsFor(prefType string, signals JDSignals) []string {
	switch prefType {
	case "domain":
		// screening_summary.industry is here and nowhere else. It used to sit
		// alongside signals.Domain, a closed enum answering the same question
		// badly: a freight-visibility platform extracted as "saas" because the
		// enum had no logistics value, while industry read "freight logistics
		// visibility platform". Reading the enum reported "logistics" — the
		// strongest positive in the profile — as an unmet gap on the JD it
		// matched best. Migration 021 deleted the enum; this is the field now.
		// The rest of screening_summary (location, clearance, other_flags) is
		// still read by nothing; that is #48 and stays open.
		//
		// core_competencies carries the rest. A domain preference names a
		// problem space, and a JD states its problem space in prose far more
		// often than it picks a classification: "observability" was an unmet
		// gap on a posting that required observability work outright.
		return append(
			[]string{signals.ScreeningSummary.Industry},
			signals.CoreCompetencies...,
		)
	case "role_shape":
		// Named work_type until migration 021, against a signal field also
		// named work_type that meant something else entirely — arrangement,
		// not kind of work. No role-shape label can appear in a four-value
		// arrangement enum, so "small team, high ownership", "product
		// engineering", and "greenfield development" were not merely unlikely
		// to match: they could not match any JD, ever, which was 23 weight of
		// guaranteed gap on every evaluation (#53). Role shape lives in
		// culture_signals ("small team", "high ownership") and
		// core_competencies (what the person must be able to do).
		return append(
			append([]string{}, signals.CultureSignals...),
			signals.CoreCompetencies...,
		)
	case "culture":
		// screening_summary.work_arrangement is included here, and only looks
		// like a type confusion. Arrangement is a culture question, and this
		// is the only field remoteness is recorded in — a "remote-first"
		// culture preference that could not read it would score zero against a
		// genuinely remote JD. It reads better than the enum it replaced:
		// "fully remote with occasional office visits or offsites as agreed
		// with manager" is the whole answer, where "remote" was a quarter of
		// it.
		//
		// core_competencies is included for the same reason it is on domain: a
		// posting states "mentoring senior engineers" as a job requirement more
		// often than it lists "mentoring" as a culture signal. The extraction
		// prompt keeps the two fields from duplicating each other, so reading
		// both widens coverage rather than double-counting.
		return append(
			append([]string{signals.ScreeningSummary.WorkArrangement}, signals.CultureSignals...),
			signals.CoreCompetencies...,
		)
	case "core_practice":
		// Deliberately NOT split into alternatives, which is the one place this
		// branch differs from dealbreaker below and the whole point of the
		// type. An alternatives group means the JD offers substitutes; something
		// you can be excused from is not what the role is practised in, so an
		// exclude naming only one member of a group must not fire. Splitting
		// "Java | Python" back apart is exactly how the Python gate tripped on a
		// posting asking for "Java and/or Python" (#68).
		return collapseSubsumed(signals.CorePractice)
	default:
		// dealbreaker is the catch-all type and has no single corresponding
		// signal field. It keeps required_skills, and is the only branch that
		// does — a domain or culture preference has no business matching
		// against a list of tools.
		//
		// What lives here after #68 is the bare-label excludes, where presence
		// IS the whole claim: "C# / .NET", "Windows / Microsoft ecosystem",
		// "crypto / blockchain". A row whose label instead claims prominence
		// belongs under core_practice; leaving it here is what made every
		// posting that merely mentioned Python trip a gate about expert Python.
		fields := []string{
			signals.ScreeningSummary.Industry,
			signals.ScreeningSummary.WorkArrangement,
		}
		fields = append(fields, signals.CultureSignals...)
		// Alternatives are split back out so an exclude still matches a single
		// option buried in a group — "Ruby" against "Java | Ruby | Python".
		// Correct here and wrong for core_practice, for the reason spelled out
		// in that branch: this asks whether the JD demands the thing at all,
		// and it demands each alternative in the sense that matters to a bare
		// exclude.
		for _, req := range signals.RequiredSkills {
			fields = append(fields, splitAlternatives(req)...)
		}
		return fields
	}
}

// preferenceAnswered reports whether a preference row is answered by the JD,
// routing it at the fields its type is about and applying the matching rule
// that type carries.
//
// Every type but core_practice is answered by any one field. core_practice is
// answered only by a whole entry — see corePracticeAnswered.
func preferenceAnswered(p db.Preference, signals JDSignals) bool {
	fields := prefFieldsFor(p.PreferenceType, signals)
	if p.PreferenceType == "core_practice" {
		return corePracticeAnswered(p, fields)
	}
	return preferenceMatches(p, fields)
}

// corePracticeAnswered reports whether a preference answers a core_practice
// entry outright: it must answer EVERY alternative in the entry, not merely
// appear somewhere inside it.
//
// # Declining to split a group was never protection on its own
//
// prefFieldsFor hands core_practice entries over ungrouped, on the rule that a
// technology you can be excused from is not what the role is built on — that
// is #68, and "Proficiency in Java and/or Python" is the posting that made it
// concrete. But containsPhrase matches a token run ANYWHERE inside the field,
// so leaving "Java | Python" whole stops nothing by itself: the run ["python"]
// sits inside it either way.
//
// What held the rule up until now was label length, which is not a mechanism.
// Every gated label happened to be prose — "Python as a primary language" is
// five tokens and cannot sit inside a two-token group — so the group survived
// on the accident that no label was short enough to defeat it. Two things
// defeat it the moment they appear, and both already exist:
//
//   - A short label. The clinical profile carries core_practice "Epic", and a
//     posting whose core practice reads "Epic | Cerner" matched it.
//   - Any alias. Vocabulary on these rows is a single technology name by
//     nature, which is exactly the shape that reaches inside a group. This is
//     why database/seed/025_preference_vocabulary.sql left all seven
//     core_practice rows without vocabulary while the other 19 rows got theirs.
//
// The rule stated positively: a group means the posting offers substitutes, so
// it is answered only when the row refuses ALL of them. A row refusing both
// halves of "TypeScript | Node.js" is not being offered an out, and still
// fires. A row refusing only Python is, and does not.
//
// The row answers each alternative as a whole — label or any alias — rather
// than one term having to cover the entire group, so a group may be refused
// jointly: the label answering one half and an alias the other is still a
// posting with no acceptable option in it.
//
// An entry with no alternatives is skipped rather than answered. Ranging over
// an empty list would satisfy "every alternative" vacuously and match
// everything, which is how splitAlternatives' own malformed-entry guard reads
// as unsatisfied rather than universal.
func corePracticeAnswered(p db.Preference, entries []string) bool {
	for _, entry := range entries {
		alts := splitAlternatives(entry)
		if len(alts) == 0 {
			continue
		}
		answered := true
		for _, alt := range alts {
			if !preferenceMatches(p, []string{alt}) {
				answered = false
				break
			}
		}
		if answered {
			return true
		}
	}
	return false
}

// preferenceMatches reports whether a preference row answers any of the fields
// it is routed at, by its label or by any of its aliases.
//
// Aliases are matching vocabulary only. They never reach the narrative — the
// projection hands the model label and type and nothing else, so a row that
// fired through "programmatic advertising" is still described to the reader as
// "adtech", in the user's own words. That separation is deliberate: the label
// is what the user said, the aliases are how a posting might say it, and
// showing the second would read as the system putting words in their mouth.
func preferenceMatches(p db.Preference, fields []string) bool {
	if matchesSignal(p.Label, fields) {
		return true
	}
	for _, alias := range p.Aliases {
		if strings.TrimSpace(alias) == "" {
			continue
		}
		if matchesSignal(alias, fields) {
			return true
		}
	}
	return false
}

// matchesSignal reports whether label matches any of fields on word
// boundaries, in either direction — label containing field, or field
// containing label. Fields are typically short canonical tokens (e.g.
// "defense") while labels are often broader descriptive phrases (e.g.
// "defense / aerospace"), so a single fixed direction would miss legitimate
// matches.
//
// Bidirectional matching is only safe because callers control which fields a
// given label is compared against. Comparing every label against every field
// is what produced the staff-seniority collision — see prefFieldsFor.
func matchesSignal(label string, fields []string) bool {
	for _, f := range fields {
		if f == "" {
			continue
		}
		if containsPhrase(f, label) || containsPhrase(label, f) {
			return true
		}
	}
	return false
}

// tokenize splits s into lowercase word tokens, discarding punctuation and
// separators, and folds regular plurals. "TypeScript / Node.js" becomes
// ["typescript", "node", "js"]; "patient assessments" becomes ["patient",
// "assessment"].
func tokenize(s string) []string {
	raw := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		out = append(out, foldPlural(t))
	}
	return out
}

// foldPlural strips one trailing "s" from a regular plural. It is not a
// stemmer and must not become one.
//
// The gap it closes is that the same idea is routinely written both ways, and
// a bare plural used to clear all three matching layers: "API" against "APIs",
// "patient assessment" against "patient assessments", "case management"
// against "cases managed". On technology names that was a documented
// annoyance; on the vocabulary of a career built out of ordinary English
// nouns it is most of the corpus.
//
// Two properties make this safe rather than clever:
//
//   - It is applied to BOTH sides, inside tokenize, so it cannot introduce an
//     asymmetry. Any pair that matches today still folds to the same tokens
//     and still matches — the fold only ever widens, never breaks.
//   - It declines where a trailing "s" is reliably not a plural: tokens under
//     four characters (AWS, OS, iOS, CSS, DDS, DIS — the acronyms where the S
//     is part of the name) and tokens ending in "ss" (Harness, business,
//     process). Both keep their "s" on both sides.
//
// It deliberately does NOT try to be right about the rest. "Redis" folds to
// "redi" and "Kubernetes" to "kubernete", which look wrong and are harmless:
// both sides fold identically, so each still matches itself, and neither
// collides with another term. Distinguishing "APIs" (a plural) from "Redis"
// (not one) needs a vocabulary, not a longer suffix list — and a vocabulary is
// what tags.aliases already is. The blast radius of over-folding is two
// different words folding to the same token, which
// TestPluralFoldDoesNotCollapseUnrelatedTerms is the guard for.
//
// English irregulars are out of scope for the same reason:
// "criteria"/"criterion" and "analyses"/"analysis" are alias data.
func foldPlural(token string) string {
	if len(token) < 4 || !strings.HasSuffix(token, "s") {
		return token
	}
	if strings.HasSuffix(token, "ss") {
		return token
	}
	return token[:len(token)-1]
}

// containsPhrase reports whether needle appears as a contiguous run of whole
// words within haystack. Unlike a raw substring check it will not match "go"
// inside "mongodb" or "ai" inside "maintain" — the overlap has to fall on
// word boundaries.
func containsPhrase(needle, haystack string) bool {
	n := tokenize(needle)
	h := tokenize(haystack)
	if len(n) == 0 || len(n) > len(h) {
		return false
	}
	for i := 0; i+len(n) <= len(h); i++ {
		if slices.Equal(h[i:i+len(n)], n) {
			return true
		}
	}
	return false
}

// matchesTerm reports whether any of terms answers the JD term tag, on
// whole-word boundaries. It runs in ONE direction: a term answers tag when the
// term's tokens sit as a contiguous run inside tag, so the skill "Kafka"
// answers a requirement of "Apache Kafka" and the skill "GitHub Actions" does
// not answer a requirement of "GitHub". That asymmetry is deliberate and is
// what the alias layer is for — 'github' on the GitHub Actions tag is the same
// move as 'sql' on PostgreSQL, described below.
//
// This is not preference matching, which compares in both directions
// (matchesSignal) because a preference label is broader than the field it is
// weighed against, not narrower.
//
// # There used to be a raw-substring direction, and it is gone
//
// The skill's own name was matched as a case-insensitive substring of the JD
// term, justified by one case: a JD asking for "SQL" is answered by
// "PostgreSQL". That direction bought that case and paid for it three ways.
//
// It over-reached. "API" was answered by Anthropic API and FastAPI, "systems"
// by Distributed systems, and — measured against the eval corpus before this
// change — the only requirement it actually reached that whole-word matching
// did not was a JD term "Go" answered by the skill "ArgoCD". One real match,
// zero of them correct.
//
// It shadowed more precise vocabulary. Because the direct layer outranks the
// alias layer, a term sitting inside another skill's name could never be given
// an alias: there was no way to say "this term means REST" for a term that
// happens to be a substring of something else.
//
// And it did not survive a career it was not written for. On generic
// vocabulary the same rule makes a JD requiring "art" match a stored
// "charting", "care" match "Medicare", and "rad" match "gradation" — the exact
// nonsense containsPhrase was written to prevent, reintroduced by the one
// direction that skipped it. That is #75.
//
// What replaces it is data, which is what CLAUDE.md already says the answer is
// here: PostgreSQL and MySQL carry 'sql' in tags.aliases, so the case that
// justified the substring direction is answered by the alias layer, on the
// specific skill rather than by a letter coincidence.
//
// The terms slice is whatever the caller wants weighed against tag: a single
// skill name, one skill's aliases, or a category name plus its competency
// vocabulary. Keeping the primitive dumb is deliberate — the ranking of those
// three lives in satisfies, where it can be read in one place.
func matchesTerm(terms []string, tag string) bool {
	for _, term := range terms {
		if term == "" {
			continue
		}
		if containsPhrase(term, tag) {
			return true
		}
	}
	return false
}

func clampScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// LevelScale orders the depth vocabulary skills.proficiency and
// jd_signals.skill_levels share. It is built from the user's own
// proficiency_levels rows, so a licence tier, a union classification, or a
// CEFR band ranks the same way novice/proficient/expert used to — this was a
// switch statement over three hardcoded values, which meant every scale that
// is not software's ranked at zero and satisfied nothing.
//
// Keys are lowercased values and aliases together. Aliases matter on this side
// because the JD half of the comparison is extraction's reading of a posting's
// wording ("deep expertise in Kafka"), not a value chosen from a list.
type LevelScale map[string]int

// NewLevelScale builds a scale from a user's proficiency vocabulary.
//
// A value beats an alias: an alias on one band must never outrank another
// band's own name, the same precedence the skill matcher uses.
func NewLevelScale(rows []db.ProficiencyLevel) LevelScale {
	scale := make(LevelScale, len(rows)*3)
	for _, r := range rows {
		for _, alias := range r.Aliases {
			if key := strings.ToLower(strings.TrimSpace(alias)); key != "" {
				scale[key] = int(r.Rank)
			}
		}
	}
	for _, r := range rows {
		if key := strings.ToLower(strings.TrimSpace(r.Value)); key != "" {
			scale[key] = int(r.Rank)
		}
	}
	return scale
}

// defaultLevelScale is the shipped neutral scale, used when an account has no
// proficiency vocabulary of its own. Signup installs one, so reaching this
// means the account was created another way.
func defaultLevelScale() LevelScale {
	rows := vocabulary.DefaultProficiencyLevels()
	scale := make(LevelScale, len(rows)*3)
	for _, r := range rows {
		for _, alias := range r.Aliases {
			scale[strings.ToLower(alias)] = int(r.Rank)
		}
	}
	for _, r := range rows {
		scale[strings.ToLower(r.Value)] = int(r.Rank)
	}
	return scale
}

// Rank returns the depth ordinal for a level. Anything unrecognised ranks 0,
// below every band, so a typo in either source degrades to "no level
// established" rather than silently satisfying an expert requirement.
func (s LevelScale) Rank(level string) int {
	if len(s) == 0 {
		s = defaultLevelScale()
	}
	return s[strings.ToLower(strings.TrimSpace(level))]
}

// requiredLevelFor returns the depth the JD asked for on this requirement, and
// the JD language behind it.
//
// The lookup is by exact requirement string, including any " | " group, and is
// case-insensitive on the whole entry only — extraction is asked to copy the
// entry verbatim, so a near-miss is a prompt failure rather than something to
// paper over with fuzzy matching here. A miss returns an empty level, which
// every caller reads as "the JD stated no depth for this", and scoring then
// proceeds exactly as it did before skill levels existed.
func requiredLevelFor(entry string, stated []generation.SkillLevel, levels LevelScale) (level, signal string) {
	for _, sl := range stated {
		if strings.EqualFold(strings.TrimSpace(sl.Requirement), strings.TrimSpace(entry)) {
			if levels.Rank(sl.Level) == 0 {
				// A level nobody can rank cannot be compared against, so it is
				// not a requirement — treat it as unstated rather than as
				// unmet. Reporting a partial here would blame the profile for
				// an extraction defect.
				return "", ""
			}
			return sl.Level, sl.Signal
		}
	}
	return "", ""
}

// strongestProficiency returns the highest proficiency among the skills behind
// a match.
//
// Highest rather than lowest because the evidence list is a set of skills that
// each independently answer the requirement — any one of them satisfying the
// depth asked for means the person can do the work. This reads differently for
// a category match, where the evidence is every active skill in the category:
// one expert anywhere in "Observability" carries the whole category over an
// expert bar. That is a known consequence of category evidence being broad,
// not an oversight, and it is visible in the report because Kind records that
// the match was a category match in the first place.
func strongestProficiency(evidence []SkillTerm, levels LevelScale) string {
	best := ""
	for _, s := range evidence {
		if levels.Rank(s.Proficiency) > levels.Rank(best) {
			best = s.Proficiency
		}
	}
	return best
}
