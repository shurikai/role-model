package generation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// reconcileSkills re-adds claimed skills that an emitted bullet names but the
// Skills section left out.
//
// resume_body.tmpl already states this as its first skill-inclusion criterion
// ("demonstrated by a bullet that actually appears in this output"), and 2a
// does not reliably honour it: a generation citing "80%+ test coverage with
// JUnit and Mockito" and "7 engineers ... in C# on .NET 4" emitted neither
// JUnit, Mockito, C#, nor .NET in its Skills object, dropping the whole
// Testing category while a bullet advertised it.
//
// The check is fully deterministic, so it belongs here rather than in more
// prompt wording. The generator already stamps facts the model must not own —
// meta, generated_at, resume_version_id — and a bullet/skills consistency
// invariant is the same kind of fact.
//
// Deliberately re-add only, never drop. A skill can be legitimately claimed
// without a dedicated bullet — that is what the prompt's other two inclusion
// criteria are for, and dropping on that basis would delete a JD-relevant
// skill for want of space in the bullet budget.
//
// Only skills in `claimed` are eligible. A bullet naming WAGO, GIFT, or NGTS
// must not manufacture a skill the user never claimed; that rule is the whole
// reason the Skills section is sourced from the skills table.
func reconcileSkills(doc map[string]json.RawMessage, claimed []SkillView) error {
	raw, ok := doc["skills"]
	if !ok || len(raw) == 0 {
		return nil
	}

	order, byCategory, err := decodeOrderedSkills(raw)
	if err != nil {
		return err
	}

	bullets, err := collectBulletText(doc)
	if err != nil {
		return err
	}

	var added bool
	for _, skill := range claimed {
		if skillAlreadyListed(byCategory, skill.Name) {
			continue
		}
		if !namedInAny(bullets, skill.Name) {
			continue
		}
		if _, exists := byCategory[skill.Category]; !exists {
			order = append(order, skill.Category)
		}
		byCategory[skill.Category] = append(byCategory[skill.Category], skill.Name)
		added = true
	}
	if !added {
		return nil
	}

	encoded, err := encodeOrderedSkills(order, byCategory)
	if err != nil {
		return err
	}
	doc["skills"] = encoded
	return nil
}

// decodeOrderedSkills reads the skills object preserving category order.
//
// Order matters and a plain map would destroy it: the renderer prints
// categories in the order the document lists them, so decoding to a map and
// re-marshalling would silently re-alphabetize the resume's Skills section as
// a side effect of adding one entry to it.
func decodeOrderedSkills(raw json.RawMessage) ([]string, map[string][]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))

	tok, err := dec.Token()
	if err != nil {
		return nil, nil, fmt.Errorf("reconcile skills: read skills object: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, nil, fmt.Errorf("reconcile skills: skills is not an object")
	}

	var order []string
	byCategory := map[string][]string{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, nil, fmt.Errorf("reconcile skills: read category name: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, nil, fmt.Errorf("reconcile skills: category name is not a string")
		}

		var values []string
		if err := dec.Decode(&values); err != nil {
			return nil, nil, fmt.Errorf("reconcile skills: read category %q: %w", key, err)
		}

		order = append(order, key)
		byCategory[key] = values
	}

	return order, byCategory, nil
}

func encodeOrderedSkills(order []string, byCategory map[string][]string) (json.RawMessage, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, category := range order {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(category)
		if err != nil {
			return nil, fmt.Errorf("reconcile skills: encode category %q: %w", category, err)
		}
		b.Write(key)
		b.WriteByte(':')
		values, err := json.Marshal(byCategory[category])
		if err != nil {
			return nil, fmt.Errorf("reconcile skills: encode category %q entries: %w", category, err)
		}
		b.Write(values)
	}
	b.WriteByte('}')
	return json.RawMessage(b.Bytes()), nil
}

// collectBulletText gathers every bullet the document emits, across both
// experience and projects.
func collectBulletText(doc map[string]json.RawMessage) ([]string, error) {
	var texts []string

	if raw, ok := doc["experience"]; ok && len(raw) > 0 {
		var employers []struct {
			Positions []struct {
				Bullets []struct {
					Text string `json:"text"`
				} `json:"bullets"`
			} `json:"positions"`
		}
		if err := json.Unmarshal(raw, &employers); err != nil {
			return nil, fmt.Errorf("reconcile skills: parse experience: %w", err)
		}
		for _, e := range employers {
			for _, p := range e.Positions {
				for _, b := range p.Bullets {
					texts = append(texts, b.Text)
				}
			}
		}
	}

	if raw, ok := doc["projects"]; ok && len(raw) > 0 {
		var projects []struct {
			Bullets []struct {
				Text string `json:"text"`
			} `json:"bullets"`
		}
		if err := json.Unmarshal(raw, &projects); err != nil {
			return nil, fmt.Errorf("reconcile skills: parse projects: %w", err)
		}
		for _, p := range projects {
			for _, b := range p.Bullets {
				texts = append(texts, b.Text)
			}
		}
	}

	return texts, nil
}

// skillAlreadyListed reports whether any category already carries this skill.
//
// It matches on the name inside the entry rather than on equality, because an
// entry may carry a depth annotation: "Java (25 yrs, expert)" already lists
// Java, and an equality check would re-add a bare duplicate beside it.
func skillAlreadyListed(byCategory map[string][]string, name string) bool {
	for _, entries := range byCategory {
		if namedInAny(entries, name) {
			return true
		}
	}
	return false
}

func namedInAny(texts []string, name string) bool {
	for _, text := range texts {
		if namedIn(text, name) {
			return true
		}
	}
	return false
}

// namedIn reports whether text names the skill as a whole word.
//
// Whole-word, never substring — the same rule the fit-gate matcher documents,
// for the same reason. Substring matching makes "Go" a hit inside "Golang"
// and "Google", and "Java" a hit inside "JavaScript", which here would put a
// skill on a resume because a bullet mentioned a different one.
//
// The boundary test is "not a letter or digit" rather than a regexp \b,
// because real skill names carry punctuation that \b treats as a boundary in
// the wrong places: C++, C#, .NET, CI/CD. Their surrounding characters —
// spaces, commas, slashes — are all non-alphanumeric, so the test holds for
// them while still rejecting Golang.
func namedIn(text, name string) bool {
	if name == "" {
		return false
	}

	lowerText := strings.ToLower(text)
	lowerName := strings.ToLower(name)

	for offset := 0; ; {
		idx := strings.Index(lowerText[offset:], lowerName)
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len(lowerName)

		beforeOK := start == 0 || !isWordChar(rune(lowerText[start-1]))
		afterOK := end == len(lowerText) || !isWordChar(rune(lowerText[end]))
		if beforeOK && afterOK {
			return true
		}

		offset = start + 1
		if offset >= len(lowerText) {
			return false
		}
	}
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
