// Package intake turns approved import drafts into rows.
//
// It exists because Stage 0 could draft contributions and nothing else, and
// ApproveDraft required a position_id that already existed. An account with no
// employers and no positions — every new account — could not use the import at
// all, which made the low-friction path unreachable for exactly the person it
// was for.
package intake

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// PreferenceCollision reports that a drafted preference label matches an
// existing one closely enough that both will fire on the same job description.
type PreferenceCollision struct {
	Existing string
	Reason   string
}

func (c PreferenceCollision) String() string {
	return fmt.Sprintf("%s: %q", c.Reason, c.Existing)
}

// CheckPreferenceLabel reports collisions between a drafted label and the
// labels already on the account.
//
// The failure it catches is recorded in migration 015 and is not hypothetical.
// The seeded pair "product over platform / internal tooling" (positive) and
// "platform / internal tooling over product" (negative) tokenize to the same
// bag of words, and matchesSignal compares token runs — it cannot tell "X over
// Y" from "Y over X". Both matched the same posting, so the profile earned and
// conflicted on one signal at once.
//
// A person writes preference labels a handful at a time and notices. A
// conversational intake proposing twenty in one pass will produce these far
// more readily, and nothing downstream complains — the rows simply both fire,
// quietly, forever.
//
// This is a REVIEW FLAG, not a rejection. Two labels sharing vocabulary can be
// legitimate ("remote-first" and "remote"), and the person is better placed to
// say which. The flag lands on the draft and the human decides.
func CheckPreferenceLabel(label string, existing []string) []PreferenceCollision {
	bag := tokenBag(label)
	if len(bag) == 0 {
		return nil
	}

	var out []PreferenceCollision
	for _, other := range existing {
		otherBag := tokenBag(other)
		if len(otherBag) == 0 {
			continue
		}

		// Same words in the same order is a duplicate, whatever the
		// punctuation and case: "remote-first" and "Remote First" are one
		// preference written twice, and the UNIQUE constraint on
		// (user_id, preference_type, label) will not catch it because the
		// strings differ.
		if equalRuns(bag, otherBag) {
			out = append(out, PreferenceCollision{
				Existing: other,
				Reason:   "duplicate label, differing only in case or punctuation",
			})
			continue
		}

		if equalBags(bag, otherBag) {
			out = append(out, PreferenceCollision{
				Existing: other,
				Reason: "same words in a different order, so both labels match the same posting — " +
					"matching compares token runs and cannot tell \"X over Y\" from \"Y over X\"",
			})
			continue
		}

		// One label containing the other as a whole-word run means the shorter
		// fires wherever the longer does. That is not always wrong, but it is
		// always worth a look: it is how a broad label silently swallows a
		// specific one.
		if containsRun(bag, otherBag) || containsRun(otherBag, bag) {
			out = append(out, PreferenceCollision{
				Existing: other,
				Reason:   "one label contains the other, so the broader one fires wherever the narrower does",
			})
		}
	}
	return out
}

// tokenBag lowercases and splits a label the way the fit gate's matcher does,
// so a collision found here is a collision there. It deliberately does not fold
// plurals: this is about labels a human wrote, and reporting "service" and
// "services" as a collision would be noise.
func tokenBag(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// equalRuns reports whether two token sequences are identical in order.
func equalRuns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalBags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// containsRun reports whether needle appears as a contiguous run inside
// haystack, which is the same test containsPhrase makes in the fit gate.
func containsRun(haystack, needle []string) bool {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
