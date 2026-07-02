// Package fitgate implements the deterministic fit gate and scoring pass
// that runs between Stage 1 (JD signal extraction) and Stage 2 (resume
// generation). Nothing in this file talks to the database or an LLM — it is
// pure scoring logic over data the caller assembles.
package fitgate

import (
	"strings"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

// JDSignals is the structured output of Stage 1 extraction.
type JDSignals = generation.JDSignals

// RunAntiPatternGate checks JD signals against the user's hard-exclude
// preferences. Matching is a case-insensitive substring check of the
// preference label against the JD's domain, work type, seniority, and
// culture signal fields. The first match fails the gate.
func RunAntiPatternGate(prefs []db.Preference, signals JDSignals) (passed bool, hits []db.Preference) {
	fields := signalFields(signals)
	for _, p := range prefs {
		if p.Sentiment != "hard_exclude" {
			continue
		}
		if matchesSignal(p.Label, fields) {
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
// its weight. An unmatched positive preference is reported as a gap; an
// unmatched negative preference earns its weight, since avoiding a stated
// dislike is the ideal outcome.
func ScorePreferenceFit(prefs []db.Preference, signals JDSignals) (score float64, gaps []string) {
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
			} else {
				earned += weight
			}
		}
	}

	if !counted || possible == 0 {
		return 100.0, nil
	}
	return clampScore(earned / possible * 100), gaps
}

// signalFields collects the JD's free-text signal fields for matching
// against preference labels.
func signalFields(signals JDSignals) []string {
	return append([]string{signals.Domain, signals.WorkType, signals.Seniority}, signals.CultureSignals...)
}

// matchesSignal reports whether label matches any of fields via a
// case-insensitive substring check in either direction — label containing
// field, or field containing label. Fields are typically short canonical
// tokens (e.g. "defense") while labels are often broader descriptive
// phrases (e.g. "defense / aerospace"), so a single fixed direction would
// miss legitimate matches.
func matchesSignal(label string, fields []string) bool {
	needle := strings.ToLower(label)
	for _, f := range fields {
		if f == "" {
			continue
		}
		hay := strings.ToLower(f)
		if strings.Contains(hay, needle) || strings.Contains(needle, hay) {
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
