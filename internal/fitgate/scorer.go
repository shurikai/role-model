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
)

// JDSignals is the structured output of Stage 1 extraction.
type JDSignals = generation.JDSignals

// RunAntiPatternGate checks JD signals against the user's hard-exclude
// preferences. Each preference is matched, on word boundaries, only against
// the signal fields its preference_type names — see gateFieldsFor. The first
// match fails the gate.
//
// The result is advisory: RunFitEvaluation records it but does not act on it.
func RunAntiPatternGate(prefs []db.Preference, signals JDSignals) (passed bool, hits []db.Preference) {
	for _, p := range prefs {
		if p.Sentiment != "hard_exclude" {
			continue
		}
		if matchesSignal(p.Label, gateFieldsFor(p.PreferenceType, signals)) {
			return false, []db.Preference{p}
		}
	}
	return true, nil
}

// ScoreTechnicalFit scores how well skillNames (the user's resolved skill
// tag names) cover the JD's required and preferred skills. Required matches
// are worth twice a preferred match. Gaps list required skills with no
// matching entry in skillNames.
func ScoreTechnicalFit(skillNames []string, signals JDSignals) (score float64, gaps []string) {
	pointsPossible := float64(len(signals.RequiredSkills)*2 + len(signals.PreferredSkills))
	if pointsPossible == 0 {
		return 100.0, nil
	}

	var pointsEarned float64
	for _, req := range signals.RequiredSkills {
		if matchesAny(skillNames, req) {
			pointsEarned += 2
		} else {
			gaps = append(gaps, req)
		}
	}
	for _, pref := range signals.PreferredSkills {
		if matchesAny(skillNames, pref) {
			pointsEarned += 1
		}
	}

	return clampScore(pointsEarned / pointsPossible * 100), gaps
}

// ScorePreferenceFit scores JD alignment against the user's non-hard-exclude
// preferences (hard excludes are handled by RunAntiPatternGate). A matched
// positive preference earns its weight; a matched negative preference costs
// its weight.
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
func ScorePreferenceFit(prefs []db.Preference, signals JDSignals) (score float64, gaps []string, conflicts []string) {
	fields := signalFields(signals)

	var earned, possible float64
	var counted bool
	for _, p := range prefs {
		if p.Sentiment == "hard_exclude" {
			continue
		}
		counted = true

		weight := 1.0
		if p.Weight != nil {
			weight = float64(*p.Weight)
		}
		possible += weight

		matched := matchesSignal(p.Label, fields)
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

	if !counted || possible == 0 {
		return 100.0, nil, nil
	}
	return clampScore(earned / possible * 100), gaps, conflicts
}

// signalFields collects the JD's free-text signal fields for matching
// against preference labels.
func signalFields(signals JDSignals) []string {
	return append([]string{signals.Domain, signals.WorkType, signals.Seniority}, signals.CultureSignals...)
}

// gateFieldsFor returns the JD signal fields a hard-exclude preference of the
// given type should be compared against.
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
func gateFieldsFor(prefType string, signals JDSignals) []string {
	switch prefType {
	case "domain":
		return []string{signals.Domain}
	case "work_type":
		return []string{signals.WorkType}
	case "culture":
		return signals.CultureSignals
	default:
		// anti_pattern is the catch-all type and has no single corresponding
		// signal field. It is also where every skills-shaped exclude lives
		// ("expert Python as primary requirement" and friends), which is why
		// this is the only branch that sees the skills arrays — a domain or
		// culture exclude has no business matching against a tech stack.
		fields := []string{signals.Domain, signals.WorkType}
		fields = append(fields, signals.CultureSignals...)
		fields = append(fields, signals.RequiredSkills...)
		fields = append(fields, signals.PreferredSkills...)
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
// is what produced the staff-seniority collision — see gateFieldsFor.
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

// matchesAny reports whether tag appears as a case-insensitive substring of
// any name in skillNames.
func matchesAny(skillNames []string, tag string) bool {
	needle := strings.ToLower(tag)
	for _, name := range skillNames {
		if strings.Contains(strings.ToLower(name), needle) {
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
