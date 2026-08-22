package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
)

// JDSignals is the structured output of signal extraction.
type JDSignals struct {
	// Skills
	RequiredSkills  []string `json:"required_skills"`
	PreferredSkills []string `json:"preferred_skills"`

	// CoreCompetencies holds the capability-level requirements a JD states in
	// prose instead of as named technology — "decomposing a legacy service",
	// "production ownership of services", "setting technical direction".
	//
	// It exists because senior and staff postings routinely name no concrete
	// technology at all, leaving RequiredSkills and PreferredSkills correctly
	// but uselessly empty. Every consumer then degrades at once and silently:
	// the 2a requirement checklist renders "(none listed)" twice, so the
	// prompt's whole skill-relevance apparatus has nothing to filter against
	// and falls back to emitting the tag inventory; and ScoreCapabilityFit
	// scores a vacuous 100 against an empty requirement list.
	//
	// Deliberately NOT part of the document projection. Adding a field here
	// must not change what the document emits — see documentJDSignals.
	CoreCompetencies []string `json:"core_competencies"`

	// CorePractice holds what the posting frames the role as being PRACTISED
	// IN day to day — the tools, methods, or materials the person will
	// actually work with — as opposed to RequiredSkills, which records only
	// that something is asked for somewhere.
	//
	// It was named PrimaryStack, and a stack is software's word for it. The
	// distinction it carries is not software-specific: "Spanish fluency as a
	// core requirement" and "Epic as the charting system" are prominence
	// claims in exactly the way "Python as a primary language" is.
	//
	// The distinction is not cosmetic. Several preference rows make a claim
	// about prominence rather than presence ("Python as a primary language",
	// "Angular as co-equal frontend requirement"), and until this field existed
	// there was nothing for that claim to be checked against. The fit gate
	// matched them against RequiredSkills, where the qualifier was inert text:
	// a posting whose must-haves read "Proficiency in Java and/or Python"
	// tripped the Python gate on the bare token, capped the preference score at
	// hardGateCeiling, and recommended a pass on a role that never asked for
	// expert Python at all (#68).
	//
	// Interchangeable alternatives keep the " | " grouping RequiredSkills uses,
	// and here the grouping is load-bearing rather than a convenience: a
	// technology the posting offers a substitute for is, by definition, not
	// what the role is built on. internal/fitgate deliberately does not split
	// these apart — see prefFieldsFor.
	//
	// Deliberately NOT part of the document projection, same as
	// CoreCompetencies. Adding a field here must not change what the document
	// emits — see documentJDSignals.
	CorePractice []string `json:"core_practice"`

	// SkillLevels records a depth requirement for individual entries in
	// RequiredSkills or PreferredSkills, and only where the posting states one.
	//
	// It is a sparse side table rather than an encoding inside the skill
	// strings because those strings are consumed directly by resume generation
	// and the frontend, which want the plain technology name. Requirement is a
	// lookup key back into those arrays, matching an entry verbatim including
	// any " | " group.
	//
	// Most requirements have no entry here and that is the correct case, not a
	// gap in extraction. A posting that says "Kafka" is asking for Kafka; only
	// one that says "expert-level Kafka" or "5+ years of Kafka" is asking for
	// depth, and inventing a level for the rest would turn every unqualified
	// requirement into a judgment the JD never made.
	//
	// The scale is the same three values as skills.proficiency, so the fit
	// gate compares ordinals with no translation layer in between.
	//
	// Deliberately NOT part of the document projection, same as
	// CoreCompetencies and CorePractice. Adding a field here must not change
	// what the document emits — see documentJDSignals.
	SkillLevels []SkillLevel `json:"skill_levels"`

	// Role classification.
	//
	// Seniority is the only closed-ish field left here, and it is closed
	// against the user's own career_levels rows rather than a fixed list.
	//
	// Domain and WorkType used to sit beside it as enums --
	// fintech|observability|healthcare|... and remote|hybrid|onsite|unknown --
	// and both were deleted rather than renamed. Each had a free-text field in
	// ScreeningSummary answering the same question without the truncation, and
	// the enum was losing information to it in every stored row: four postings
	// in four different industries all extracted as domain "saas" while their
	// industry strings read "B2B business intelligence", "sales data", "web
	// marketing platform", and "AI-powered marketing cloud". "platform" and
	// "observability" were never industries at all. work_type fared no better:
	// "remote" against an arrangement of "fully remote with occasional office
	// visits or offsites as agreed with manager".
	//
	// Read ScreeningSummary.Industry and ScreeningSummary.WorkArrangement
	// instead. Do not reintroduce a classification enum beside a free-text
	// field that already answers the question.
	Seniority string `json:"seniority"`

	// Culture and preference signals
	CultureSignals []string `json:"culture_signals"`

	// Screening facts a human scans for before considering a role at all.
	ScreeningSummary ScreeningSummary `json:"screening_summary"`

	// Deprecated: retained for backward compatibility with existing jd_signals rows.
	// Do not use in new code. Will be removed in a future cleanup.
	PrioritySkills   []string `json:"priority_skills,omitempty"`
	DomainVocabulary []string `json:"domain_vocabulary,omitempty"`
}

// SkillLevel is one requirement's stated depth, as the posting phrased it.
//
// Signal carries the JD language the level was inferred from. The level itself
// is a judgment made at extraction time, and a judgment that cannot be checked
// against its source is one nobody can audit later — the same reason a fit
// report records the evidence behind a match instead of asserting a score.
type SkillLevel struct {
	// Requirement matches an entry in RequiredSkills or PreferredSkills
	// verbatim, " | " group included. A string that does not match one is
	// dead data: the lookup simply finds nothing, and the requirement scores
	// exactly as it would have without any level stated.
	Requirement string `json:"requirement"`

	// Level is "novice", "proficient", or "expert".
	Level string `json:"level"`

	// Signal is the JD language that produced the inference — "expert-level
	// Kafka", "5+ years of Go", "exposure to Terraform".
	Signal string `json:"signal"`
}

// documentJDSignals is the jd_signals projection embedded in the resume
// document's meta block. It mirrors $defs.jd_signals in schema/resume.v2.json
// exactly — those six fields, no others.
//
// It exists because the document schema is a strict contract
// (additionalProperties: false) while JDSignals is owned by extraction and
// evolves on its own schedule. Assigning the stored jd_signals blob straight
// into meta coupled the two, and broke in both directions at once: 15 stored
// applications carry the deprecated priority_skills/domain_vocabulary and 5
// carry screening_summary, none of which the schema allows. 20 of 31
// applications could not generate at all, each failing on a field the
// document never needed.
//
// The invariant this type buys: adding a field to JDSignals cannot change
// what the document emits. If the document should carry something new, the
// schema declares it first and this struct follows — never the reverse.
type documentJDSignals struct {
	RequiredSkills  []string `json:"required_skills"`
	PreferredSkills []string `json:"preferred_skills"`
	Seniority       string   `json:"seniority"`

	// Industry and WorkArrangement replace the domain and work_type enums
	// one for one, and are read out of ScreeningSummary because that is where
	// the free-text versions already lived. The document carries the same two
	// facts it always did; it no longer carries them truncated to a fixed
	// list. This is a v2-only shape — see schema/resume.v2.json.
	Industry        string `json:"industry"`
	WorkArrangement string `json:"work_arrangement"`

	CultureSignals []string `json:"culture_signals"`
}

// forDocument projects the signals down to what the document schema declares.
func (s JDSignals) forDocument() documentJDSignals {
	// The same fallback buildSkillsChecklist applies. Pre-migration rows carry
	// the requirement list under priority_skills, so reading RequiredSkills
	// alone would hand 15 stored applications an empty signal block — a
	// silent downgrade, where the data is merely named differently.
	required := s.RequiredSkills
	if len(required) == 0 {
		required = s.PrioritySkills
	}

	return documentJDSignals{
		RequiredSkills:  nonNilStrings(required),
		PreferredSkills: nonNilStrings(s.PreferredSkills),
		Seniority:       s.Seniority,
		Industry:        s.ScreeningSummary.Industry,
		WorkArrangement: s.ScreeningSummary.WorkArrangement,
		CultureSignals:  nonNilStrings(s.CultureSignals),
	}
}

// nonNilStrings keeps an absent list marshalling as [] rather than null. The
// schema types these as arrays without listing them as required, so an
// omitted key validates but an explicit null does not.
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// ScreeningSummary holds plain-language facts a human would scan a JD for
// before seriously considering it — criteria that don't relate to skills
// match but often decide whether a role is worth pursuing at all. This is
// deliberately descriptive, not classificatory: no fixed enums, because the
// set of things worth flagging is open-ended and person-specific.
//
// It exists because the classificatory fields above could not represent most
// real exclude criteria — audience, timezone, travel, clearance — and every
// new category needed a new field. Describing the role and letting a human
// judge it scales where an exclude taxonomy does not.
type ScreeningSummary struct {
	// e.g. "Providence, RI", "Remote, US", "not stated"
	Location string `json:"location"`
	// e.g. "fully remote", "hybrid, 2 days/week Minneapolis", "onsite"
	WorkArrangement string `json:"work_arrangement"`
	// e.g. "occasional customer site visits", "not mentioned"
	Travel string `json:"travel"`
	// Plain-language description, not constrained to a fixed list — e.g.
	// "defense/autonomous systems", "themed entertainment",
	// "developer telemetry SaaS".
	Industry string `json:"industry"`
	// e.g. "U.S. citizenship + active clearance required", "not mentioned"
	ClearanceCitizenship string `json:"clearance_citizenship"`
	// Free text: anything else identifying or discriminating that isn't a
	// skills match — military-coded language, FedRAMP mentions, "serves
	// internal engineering teams" framing, anonymous postings, unusual comp
	// structure, notable red flags. Empty array if nothing notable.
	OtherFlags []string `json:"other_flags"`
}

type extractPromptData struct {
	JobDescription string

	// SeniorityLevels is the rendered enum the prompt offers for `seniority`,
	// built from the user's own career_levels rows. It was a hardcoded
	// software ladder, which is how three of its values — manager, director,
	// vp — came to be storable in the database and unreachable from a job
	// description: the prompt's copy of the list never listed them.
	SeniorityLevels string
}

// seniorityEnum renders the account's ladder as the quoted list the extraction
// prompt offers. Empty when the account has no ladder, which the template
// falls back on.
func seniorityEnum(levels []db.CareerLevel) string {
	if len(levels) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(levels))
	for _, l := range levels {
		quoted = append(quoted, `"`+l.Value+`"`)
	}
	return strings.Join(quoted, ", ")
}

// ExtractSignals runs JD signal extraction against the given job description text.
func (s *Service) ExtractSignals(ctx context.Context, userID uuid.UUID, jdText string) (*JDSignals, error) {
	levels, err := s.q.ListCareerLevelsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list career levels: %w", err)
	}
	if len(levels) == 0 {
		levels = defaultCareerLevelRows()
	}

	prompt, err := renderPrompt(jdExtractionPrompt, extractPromptData{
		JobDescription:  jdText,
		SeniorityLevels: seniorityEnum(levels),
	})
	if err != nil {
		return nil, err
	}

	msg, err := s.client.api.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 1024,
		Model:     s.client.model,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic messages: %w", err)
	}

	raw, err := extractText(msg)
	if err != nil {
		return nil, err
	}

	var signals JDSignals
	if err := json.Unmarshal([]byte(stripCodeFence(raw)), &signals); err != nil {
		return nil, fmt.Errorf("parse jd signals: %w (raw: %s)", err, raw)
	}

	return &signals, nil
}

// extractText pulls the concatenated text from a message response.
func extractText(msg *anthropic.Message) (string, error) {
	var b strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("no text content in response")
	}
	return b.String(), nil
}

// stripCodeFence removes markdown code fences if the model wrapped its JSON.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
