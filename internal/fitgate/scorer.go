// Package fitgate implements the deterministic fit gate and scoring pass
// that runs between Stage 1 (JD signal extraction) and Stage 2 (resume
// generation). Nothing in this file talks to the database or an LLM — it is
// pure scoring logic over data the caller assembles.
package fitgate

import (
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
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

// SkillMatch is one satisfied requirement and the evidence behind it.
type SkillMatch struct {
	Requirement string    `json:"requirement"`
	Kind        MatchKind `json:"kind"`
	Category    string    `json:"category,omitempty"`
	Evidence    []string  `json:"evidence"`
}

// satisfies reports how skills cover the requirement in entry, and returns
// the evidence for it. An entry may offer several interchangeable
// alternatives; matching any one satisfies the whole entry.
//
// The three kinds are tried in order of strength across every alternative,
// rather than settling for the first alternative that matches somehow. A JD
// entry of "Kubernetes | container orchestration" should report the stored
// Kubernetes skill, not a category match on the phrase beside it.
func satisfies(skills []SkillTerm, entry string) (kind MatchKind, category string, evidence []string, ok bool) {
	alts := splitAlternatives(entry)

	// Strongest: the skill's own name answers the requirement.
	for _, alt := range alts {
		for _, s := range skills {
			if matchesAny([]string{s.Name}, alt) {
				evidence = appendUnique(evidence, s.Name)
			}
		}
	}
	if len(evidence) > 0 {
		return MatchDirect, "", evidence, true
	}

	// Next: a recorded synonym of the skill answers it.
	for _, alt := range alts {
		for _, s := range skills {
			if matchesPhrase(s.Aliases, alt) {
				evidence = appendUnique(evidence, s.Name)
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
			if !matchesPhrase(append([]string{s.Category}, s.CategoryAliases...), alt) {
				continue
			}
			category = s.Category
			for _, peer := range skills {
				if peer.Category == category {
					evidence = appendUnique(evidence, peer.Name)
				}
			}
			return MatchCategory, category, evidence, true
		}
	}

	return "", "", nil, false
}

// appendUnique appends v to out if it isn't already there, keeping evidence
// lists stable and free of repeats when several alternatives hit the same
// skill.
func appendUnique(out []string, v string) []string {
	if slices.Contains(out, v) {
		return out
	}
	return append(out, v)
}

// ScoreTechnicalFit scores how well the user's skills cover the JD's required
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
// where a reader can weigh it, rather than baked into the number. Weighting
// credit by actual depth is a separate problem — skills.proficiency and
// years_experience are still dropped before scoring ever sees them.
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
// Note that Stage 1 now also extracts core_competencies for exactly the
// postings that trip this, so the unscored case should be rare — but a JD can
// always fail to yield anything extractable, and the guard is independent of
// how good extraction is on any given day.
func ScoreTechnicalFit(skills []SkillTerm, signals JDSignals) TechnicalFit {
	pointsPossible := float64(len(signals.RequiredSkills)*2 + len(signals.PreferredSkills))
	if pointsPossible == 0 {
		return TechnicalFit{Scored: false}
	}

	var (
		pointsEarned float64
		gaps         []string
		matches      []SkillMatch
	)
	for _, req := range signals.RequiredSkills {
		if kind, category, evidence, ok := satisfies(skills, req); ok {
			pointsEarned += 2
			matches = append(matches, SkillMatch{
				Requirement: req, Kind: kind, Category: category, Evidence: evidence,
			})
		} else {
			gaps = append(gaps, req)
		}
	}
	for _, pref := range signals.PreferredSkills {
		if kind, category, evidence, ok := satisfies(skills, pref); ok {
			pointsEarned += 1
			matches = append(matches, SkillMatch{
				Requirement: pref, Kind: kind, Category: category, Evidence: evidence,
			})
		}
	}

	return TechnicalFit{
		Score:   clampScore(pointsEarned / pointsPossible * 100),
		Scored:  true,
		Gaps:    gaps,
		Matches: matches,
	}
}

// TechnicalFit is the outcome of technical scoring.
//
// Scored is false when the JD stated no technical requirements at all. Score
// carries no meaning in that case and callers must not display, store, or
// narrate it — see the ScoreTechnicalFit doc comment for why this is a field
// rather than a magic value.
type TechnicalFit struct {
	Score   float64
	Scored  bool
	Gaps    []string
	Matches []SkillMatch
}

// hardGateCeiling is the highest preference score a JD can hold once it has
// matched a hard-gate preference. It exists because weight subtraction alone
// cannot express "hard" under a normalized average: the denominator grows with
// the penalty, so no single row can ever dominate, and a tripped hard exclude
// would read as a ten-point dip — a minor quibble rather than a disqualifier.
//
// It is deliberately not zero. A role that is excluded for one reason and
// otherwise excellent should still be distinguishable from one that is
// excluded and also a poor fit on every other axis.
const hardGateCeiling = 25.0

// ScorePreferenceFit scores JD alignment against the user's preferences.
// A matched positive preference earns its weight; a matched negative
// preference costs its weight.
//
// The two ways a preference can fail to earn points are semantically
// different and are reported separately rather than lumped into one list:
//   - An unmatched positive preference means the JD simply doesn't mention
//     something Jason wants — a gap, not a conflict.
//   - A matched negative preference means the JD actively signals something
//     Jason dislikes — a genuine conflict.
//
// An unmatched negative preference earns its weight, since avoiding a
// stated dislike is the ideal outcome, and isn't reported at all (there's
// nothing notable to say about an absence of an absence).
//
// # Hard gates
//
// Rows carrying is_hard_gate are scored differently, and deliberately so.
// They are never terms in the normalized average — feeding them into both
// earned and possible would let a profile full of unmatched excludes inflate
// every clean JD toward 100, which is the same pathology that made unreachable
// negatives a free bonus. Instead a matched hard gate subtracts its weight as
// raw points and caps the result at hardGateCeiling.
//
// Every matched hard gate is returned in gateHits so the caller can record the
// complete set, and is also reported as a conflict so the narrative can name
// it. A hard gate that goes unmatched costs and earns nothing at all.
//
// The 100.0 short-circuit for a user with no scorable preferences governs only
// the average. Penalty and ceiling still apply on top of it: a profile made up
// entirely of hard gates has an empty average but must not therefore score a
// perfect 100 on a JD that trips one.
func ScorePreferenceFit(prefs []db.Preference, signals JDSignals) (
	score float64, gaps []string, conflicts []string, gateHits []db.Preference,
) {
	var earned, possible, penalty float64
	var counted bool

	for _, p := range prefs {
		weight := float64(p.Weight)
		matched := matchesSignal(p.Label, prefFieldsFor(p.PreferenceType, signals))

		if p.IsHardGate {
			if matched {
				penalty += weight
				gateHits = append(gateHits, p)
				conflicts = append(conflicts, p.Label)
			}
			continue
		}

		counted = true
		possible += weight

		switch p.Sentiment {
		case "positive":
			if matched {
				earned += weight
			} else {
				gaps = append(gaps, p.Label)
			}
		case "negative":
			if matched {
				earned -= weight
				conflicts = append(conflicts, p.Label)
			} else {
				earned += weight
			}
		}
	}

	base := 100.0
	if counted && possible > 0 {
		base = earned / possible * 100
	}

	score = base - penalty
	if len(gateHits) > 0 {
		// An upper bound, never an override. Setting the score *to* the
		// ceiling would raise a JD that already scored below it.
		score = math.Min(score, hardGateCeiling)
	}
	return clampScore(score), gaps, conflicts, gateHits
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
// preferred_skills is deliberately absent too. A preference is a statement
// about what the job actually demands, so it should fire on a genuine
// requirement and not on an optional mention. The Angular exclude — "Angular
// as co-equal frontend requirement" — tripped on a JD whose only Angular
// reference was a nice-to-have bullet ("exposure to front-end technologies
// such as React or Angular"), and the narrative then described Angular as a
// co-equal requirement the JD never made it out to be. preferred_skills still
// feeds ScoreTechnicalFit normally; it is only preference matching that
// ignores it.
func prefFieldsFor(prefType string, signals JDSignals) []string {
	switch prefType {
	case "domain":
		return []string{signals.Domain}
	case "work_type":
		return []string{signals.WorkType}
	case "culture":
		// work_type is included here, and only looks like a type confusion.
		// The enum is remote | hybrid | onsite | unknown — work *arrangement*,
		// not work type — and arrangement is a culture question. It is also
		// the only field remoteness is ever recorded in, so a "remote-first"
		// culture preference that could not read it would score zero against
		// a genuinely remote JD. Adopting the gate's routing wholesale did
		// exactly that; the gate never noticed because its one culture row
		// ("Big Four consulting culture") really does live in culture_signals.
		return append([]string{signals.WorkType}, signals.CultureSignals...)
	default:
		// anti_pattern is the catch-all type and has no single corresponding
		// signal field. It is also where every skills-shaped preference lives
		// ("expert Python as primary requirement" and friends), which is why
		// this is the only branch that sees the skills arrays — a domain or
		// culture preference has no business matching against a tech stack.
		fields := []string{signals.Domain, signals.WorkType}
		fields = append(fields, signals.CultureSignals...)
		// Alternatives are split back out so an exclude still matches a single
		// option buried in a group — "Ruby" against "Java | Ruby | Python".
		for _, req := range signals.RequiredSkills {
			fields = append(fields, splitAlternatives(req)...)
		}
		return fields
	}
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
// separators. "TypeScript / Node.js" becomes ["typescript", "node", "js"].
func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
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

// matchesAny reports whether any name in skillNames answers the JD term tag.
// Two directions count, and they are deliberately asymmetric:
//
//   - tag as a case-insensitive substring of the skill name, the original
//     behavior: a JD asking for "SQL" is answered by "PostgreSQL".
//   - the skill name as a whole-word phrase inside tag: a JD asking for
//     "REST APIs" is answered by the stored skill "REST".
//
// Only one direction may be a raw substring. Reversing "SQL" ⊂ "PostgreSQL"
// unguarded is what makes a stored "Go" satisfy a JD's "Google Cloud" or
// "MongoDB", so the second direction is word-boundary matched instead. That
// still bridges the case that matters — a short canonical skill name sitting
// inside a longer JD phrase — without inventing matches from shared letters.
//
// Neither direction bridges a morphological difference on its own: "REST" and
// "RESTful APIs" share no whole word and neither contains the other. That
// gap is now closed by data rather than by string logic — the caller passes
// each skill's recorded aliases through here too, and the REST tag carries
// 'restful'. Canonicalization upstream in jd_extraction.tmpl is still the
// first line of defence; aliases are what keep a stored ten-year skill from
// reading as a gap when the JD spells it differently anyway.
//
// The terms slice is whatever the caller wants weighed against tag: a single
// skill name, one skill's aliases, or a category name plus its competency
// vocabulary. Keeping the primitive dumb is deliberate — the ranking of those
// three lives in satisfies, where it can be read in one place.
func matchesAny(terms []string, tag string) bool {
	needle := strings.ToLower(tag)
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(strings.ToLower(term), needle) {
			return true
		}
		if containsPhrase(term, tag) {
			return true
		}
	}
	return false
}

// matchesPhrase is matchesAny without the raw-substring direction: a term
// counts only where it appears as a run of whole words inside the JD phrase.
//
// It exists because the substring direction does not survive contact with
// multi-word terms. It is safe for a skill name — a JD asking for "SQL" is
// genuinely answered by "PostgreSQL" — but a category alias is a sentence
// fragment, and letting a JD term match any letters inside one produced
// exactly the nonsense you would predict: a JD requiring "RAG" matched the
// Testing category, because "rag" sits inside "test cove(rag)e", and reported
// TDD and JUnit as evidence of retrieval-augmented generation.
//
// Aliases and category vocabulary go through here. Only the skill's own name
// keeps both directions.
func matchesPhrase(terms []string, tag string) bool {
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
