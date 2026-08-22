package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/google/uuid"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/vocabulary"
	resumeschema "github.com/shurikai/role-model/schema"
)

// ErrSignalsRequired is returned when jd_signals have not been extracted yet.
var ErrSignalsRequired = errors.New("jd_signals must be extracted before generating a resume")

// bodyMaxTokens caps the 2a response. It was 4096, which fit comfortably until
// projects started reaching the document: a staff-length resume plus a project
// section landed close enough to the ceiling to truncate intermittently, and a
// truncated body is invalid JSON. 8192 leaves real headroom above the largest
// document observed (~12KB, roughly 3,000 tokens).
//
// The limit is not the length control — <length_budget> is. This exists so
// that hitting it is an obvious, reportable failure rather than a corrupt
// document.
const bodyMaxTokens = 8192

// ValidationError wraps a JSON schema validation failure from the generation pipeline.
type ValidationError struct {
	Detail string
}

func (e *ValidationError) Error() string {
	return "schema validation failed: " + e.Detail
}

type resumeBodyPromptData struct {
	CompanyName     string
	RoleTitle       string
	JDSignals       string
	SkillsChecklist string
	LengthBudget    string
	FramingGuidance string
	Identity        string
	Experience      string
	Skills          string
	Projects        string
	Education       string
	Credentials     string
	ResumeSchema    string
	PriorFeedback   string
}

// seniorityLevers returns the two levers a posting's seniority drives: how
// much gets written, and at what altitude.
//
// They are returned together because they are siblings — one career_levels row
// carries both columns, so a third lever is added as a third column rather
// than as a third lookup somewhere else. They used to be two switch statements
// over a hardcoded software ladder, and the pair had already drifted: 'mid'
// took the short budget in one and the default branch in the other.
//
// Both values are the user's own text now. The account's ladder is what says
// whether "attending", "sous", or "staff" is the top of it.
func (s *Service) seniorityLevers(ctx context.Context, userID uuid.UUID, raw *json.RawMessage) (lengthBudget, framingGuidance string, err error) {
	seniority := ""
	if raw != nil {
		var signals JDSignals
		if err := json.Unmarshal(*raw, &signals); err != nil {
			return "", "", fmt.Errorf("parse jd_signals for seniority levers: %w", err)
		}
		seniority = signals.Seniority
	}

	levels, err := s.q.ListCareerLevelsByUser(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("list career levels: %w", err)
	}

	level := pickCareerLevel(levels, seniority)
	if level == nil {
		// The account has no ladder, or one with no fallback row. Signup
		// installs both, so this means the account was made another way. The
		// shipped neutral set keeps the prompt well-formed — an empty
		// <length_budget> would leave two of the 2a rules pointing at nothing.
		level = pickCareerLevel(defaultCareerLevelRows(), seniority)
	}
	if level == nil {
		return "", "", fmt.Errorf("no career level for seniority %q and no fallback", seniority)
	}

	return level.LengthBudget, level.FramingGuidance, nil
}

// pickCareerLevel resolves a posting's stated seniority against a ladder.
//
// Own name before alias, the same precedence the fit-gate matcher uses and for
// the same reason: an alias on one rung must never outrank another rung's own
// name. A miss lands on the row flagged is_fallback, which is where "unknown"
// — the value extraction emits when it cannot tell — is meant to land.
//
// The fallback is a flagged row rather than the median rank because the median
// is not the neutral choice it looks like. On the ten-rung software ladder the
// middle rung is staff, so deriving it would hand every unreadable posting the
// ownership framing that framing guidance spends two rules warning against
// reaching for. An unrecognised seniority is not evidence of a senior role.
func pickCareerLevel(levels []db.CareerLevel, seniority string) *db.CareerLevel {
	var fallback *db.CareerLevel
	for i := range levels {
		if levels[i].IsFallback {
			fallback = &levels[i]
			break
		}
	}

	want := strings.ToLower(strings.TrimSpace(seniority))
	if want == "" {
		return fallback
	}

	for i := range levels {
		if strings.ToLower(strings.TrimSpace(levels[i].Value)) == want {
			return &levels[i]
		}
	}
	for i := range levels {
		for _, alias := range levels[i].Aliases {
			if strings.ToLower(strings.TrimSpace(alias)) == want {
				return &levels[i]
			}
		}
	}

	return fallback
}

// defaultCareerLevelRows renders the shipped neutral ladder in the row shape
// pickCareerLevel reads, for the one path that runs without a stored ladder.
func defaultCareerLevelRows() []db.CareerLevel {
	defaults := vocabulary.DefaultCareerLevels()
	rows := make([]db.CareerLevel, 0, len(defaults))
	for _, l := range defaults {
		rows = append(rows, db.CareerLevel{
			Value:           l.Value,
			Label:           l.Label,
			Rank:            l.Rank,
			Aliases:         l.Aliases,
			LengthBudget:    l.LengthBudget,
			FramingGuidance: l.FramingGuidance,
			IsFallback:      l.IsFallback,
		})
	}
	return rows
}

// buildSkillsChecklist renders the JD's requirements as an explicit checklist
// for the 2a prompt, rather than leaving cross-referencing to implicit recall
// from the <jd_signals> blob. Falls back to the deprecated priority_skills
// field for older stored jd_signals rows that predate
// required_skills/preferred_skills.
//
// Core competencies are rendered as their own section rather than folded in
// with the skills. They satisfy differently: a required skill can be answered
// by a Skills entry, while a competency is a capability that only a bullet can
// evidence — resume_body.tmpl relies on the sections staying distinct to keep
// "setting technical direction" out of the Skills list.
//
// The competency section is also what keeps this checklist non-empty for a
// staff-level JD that names no technology at all. Both skill lists are
// correctly empty for such a posting, and a checklist reading "(none listed)"
// twice silently disables every relevance rule in the 2a prompt.
func buildSkillsChecklist(raw *json.RawMessage) (string, error) {
	if raw == nil {
		return "(no jd_signals available)", nil
	}

	var signals JDSignals
	if err := json.Unmarshal(*raw, &signals); err != nil {
		return "", fmt.Errorf("parse jd_signals for checklist: %w", err)
	}

	required := signals.RequiredSkills
	if len(required) == 0 {
		required = signals.PrioritySkills
	}
	preferred := signals.PreferredSkills

	var b strings.Builder
	writeSection(&b, "Required skills", required)
	writeSection(&b, "Preferred skills", preferred)
	writeSection(&b, "Core competencies", signals.CoreCompetencies)

	return b.String(), nil
}

// writeSection renders one labelled checklist section. An empty section still
// prints its heading, so the prompt sees the same three-section shape every
// time and "(none listed)" reads as a fact about this JD rather than as a
// missing block.
func writeSection(b *strings.Builder, label string, entries []string) {
	fmt.Fprintf(b, "%s:\n", label)
	if len(entries) == 0 {
		b.WriteString("(none listed)\n")
		return
	}
	for _, e := range entries {
		fmt.Fprintf(b, "- %s\n", e)
	}
}

type resumeSummaryPromptData struct {
	CompanyName     string
	RoleTitle       string
	JDSignals       string
	HeaderTitle     string
	YearsExperience string
	Body            string
}

// buildYearsExperience derives total years of experience from the earliest
// started_on in the 2a body, formatted for the 2b prompt to quote verbatim.
// Returns "" when no usable date is present, which the prompt reads as
// "do not mention years at all".
//
// This is threaded in for the same reason HeaderTitle is: it is a fact both
// passes could derive independently, and when 2b was left to derive it the
// same career produced "27 years" for one application and "over 26 years" for
// another generated three hours earlier. The rule was deterministic; the
// arithmetic was not.
//
// Only started_on is read. An ended_on gap is not subtracted — the figure is
// career span, the ordinary meaning of "N years of experience" on a resume,
// and inferring unemployment from a date gap is not something to do silently.
func buildYearsExperience(experience json.RawMessage, now time.Time) (string, error) {
	if len(experience) == 0 {
		return "", nil
	}

	var employers []struct {
		Positions []struct {
			StartedOn string `json:"started_on"`
		} `json:"positions"`
	}
	if err := json.Unmarshal(experience, &employers); err != nil {
		return "", fmt.Errorf("parse experience for years: %w", err)
	}

	var earliest time.Time
	for _, e := range employers {
		for _, p := range e.Positions {
			started, err := time.Parse("2006-01", p.StartedOn)
			if err != nil {
				// The schema constrains this field, but 2a's output has not
				// been validated yet at this point. A single unparseable
				// date should not cost the whole figure.
				continue
			}
			if earliest.IsZero() || started.Before(earliest) {
				earliest = started
			}
		}
	}
	if earliest.IsZero() {
		return "", nil
	}

	years := int(now.Sub(earliest).Hours() / 24 / 365.25)
	if years < 1 {
		return "", nil
	}
	return fmt.Sprintf("%d years", years), nil
}

// buildMetaBlock assembles the document's meta block.
//
// It takes the full JDSignals and projects internally rather than accepting an
// already-projected value, so there is no call site at which the raw stored
// blob could be substituted. That was the original defect: meta was assembled
// inline and `"jd_signals": app.JdSignals` looked entirely reasonable there,
// while the schema forbids additional properties. Making the projection
// unbypassable is worth more than a test asserting nobody bypassed it.
func buildMetaBlock(applicationID uuid.UUID, companyName, roleTitle, generationModel string, signals JDSignals) map[string]any {
	return map[string]any{
		"schema_version":   "2.0",
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
		"application_id":   applicationID.String(),
		"target_company":   companyName,
		"target_role":      roleTitle,
		"jd_signals":       signals.forDocument(),
		"generation_model": generationModel,
		// schema/resume.v2.json requires this field and forbids additional
		// ones, so the portable document carries the coarse pipeline version
		// only. Per-prompt content hashes live in generation_params on the
		// resume_versions row, which is unconstrained JSONB.
		"prompt_version": pipelineVersion,
	}
}

// Generate runs the full resume generation pipeline for an application.
func (s *Service) Generate(ctx context.Context, applicationID, userID uuid.UUID) (*db.ResumeVersion, error) {
	app, err := s.q.GetApplication(ctx, db.GetApplicationParams{
		ID:     applicationID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: get application: %w", err)
	}
	if app.JdSignals == nil {
		return nil, ErrSignalsRequired
	}

	resumeCtx, err := s.AssembleContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	user, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("generate: get user: %w", err)
	}

	signalsJSON, err := json.MarshalIndent(app.JdSignals, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal signals: %w", err)
	}
	// The prompts get the full stored signals; the document gets only what its
	// schema declares. See documentJDSignals and buildMetaBlock.
	var parsedSignals JDSignals
	if err := json.Unmarshal(*app.JdSignals, &parsedSignals); err != nil {
		return nil, fmt.Errorf("generate: parse jd_signals for document: %w", err)
	}
	skillsChecklist, err := buildSkillsChecklist(app.JdSignals)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	lengthBudget, framingGuidance, err := s.seniorityLevers(ctx, userID, app.JdSignals)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	identityJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal identity: %w", err)
	}
	experienceJSON, err := json.MarshalIndent(resumeCtx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal experience: %w", err)
	}

	skills, err := s.assembleSkills(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	projects, err := s.assembleProjects(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	education, err := s.assembleEducation(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	credentials, err := s.assembleCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	skillsJSON, err := json.MarshalIndent(skills, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal skills: %w", err)
	}
	projectsJSON, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal projects: %w", err)
	}
	educationJSON, err := json.MarshalIndent(education, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal education: %w", err)
	}
	credentialsJSON, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal credentials: %w", err)
	}

	// Pass 2a: experience, skills, projects, education, and credentials.
	// Summary is deliberately excluded — see 2b below.
	bodyPrompt, err := renderPrompt(resumeBodyPrompt, resumeBodyPromptData{
		CompanyName:     app.CompanyName,
		RoleTitle:       app.RoleTitle,
		JDSignals:       string(signalsJSON),
		SkillsChecklist: skillsChecklist,
		LengthBudget:    lengthBudget,
		FramingGuidance: framingGuidance,
		Identity:        string(identityJSON),
		Experience:      string(experienceJSON),
		Skills:          string(skillsJSON),
		Projects:        string(projectsJSON),
		Education:       string(educationJSON),
		Credentials:     string(credentialsJSON),
		ResumeSchema:    string(resumeschema.ResumeV2JSON),
		PriorFeedback:   "",
	})
	if err != nil {
		return nil, fmt.Errorf("generate: render body prompt: %w", err)
	}

	bodyMsg, err := s.client.api.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: bodyMaxTokens,
		Model:     s.client.model,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(bodyPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic messages (body): %w", err)
	}

	// Truncation has to be reported as truncation. A body cut off at the token
	// limit is invalid JSON, and the parse error that follows points at
	// whatever field happened to be under the knife — the first occurrence
	// blamed "generation_model", which is stamped by the generator and could
	// not have been at fault.
	if bodyMsg.StopReason == anthropic.StopReasonMaxTokens {
		return nil, fmt.Errorf(
			"generate: body response hit the %d token limit and was truncated; "+
				"the resume is too long for one response — reduce the length budget "+
				"or raise bodyMaxTokens", bodyMaxTokens)
	}

	bodyRaw, err := extractText(bodyMsg)
	if err != nil {
		return nil, err
	}

	bodyCleaned := stripCodeFence(bodyRaw)

	// Parse into a key-preserving map so untouched fields pass through byte-for-byte.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(bodyCleaned), &doc); err != nil {
		return nil, fmt.Errorf("generate: parse resume body json: %w (raw: %s)", err, bodyCleaned)
	}

	// Enforce the bullet/skills invariant 2a states but does not reliably
	// honour, before 2b sees the body — the summary should be written against
	// the final Skills section, not a version still missing entries.
	if err := reconcileSkills(doc, skills); err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	// Pass 2b: summary, grounded ONLY in 2a's output — never the raw
	// background corpus. This makes claims unsupported by any generated
	// bullet structurally unreachable rather than relying on a prompt
	// instruction to self-police.
	bodyForSummary, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("generate: marshal body for summary pass: %w", err)
	}

	// Header title (identity.headline) is authoritative personal-branding
	// input, not something 2b should independently derive or restate at a
	// different level from the JD's own leveling language.
	headerTitle := ""
	if user.Headline != nil {
		headerTitle = *user.Headline
	}

	// Likewise computed here rather than left to 2b's arithmetic.
	yearsExperience, err := buildYearsExperience(doc["experience"], time.Now())
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	summaryPrompt, err := renderPrompt(resumeSummaryPrompt, resumeSummaryPromptData{
		CompanyName:     app.CompanyName,
		RoleTitle:       app.RoleTitle,
		JDSignals:       string(signalsJSON),
		HeaderTitle:     headerTitle,
		YearsExperience: yearsExperience,
		Body:            string(bodyForSummary),
	})
	if err != nil {
		return nil, fmt.Errorf("generate: render summary prompt: %w", err)
	}

	summaryMsg, err := s.client.api.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 512,
		Model:     s.client.model,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(summaryPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic messages (summary): %w", err)
	}

	summaryRaw, err := extractText(summaryMsg)
	if err != nil {
		return nil, err
	}

	summaryCleaned := stripCodeFence(summaryRaw)

	var summaryOut struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(summaryCleaned), &summaryOut); err != nil {
		return nil, fmt.Errorf("generate: parse summary json: %w (raw: %s)", err, summaryCleaned)
	}

	summaryJSON, err := json.Marshal(summaryOut.Summary)
	if err != nil {
		return nil, fmt.Errorf("generate: marshal summary: %w", err)
	}
	doc["summary"] = summaryJSON

	// Overwrite provenance fields the model must not own. The model generates
	// resume *content*; the generator stamps the *facts* about generation.
	meta := buildMetaBlock(applicationID, app.CompanyName, app.RoleTitle,
		s.client.ModelName(), parsedSignals)
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("generate: marshal meta: %w", err)
	}
	doc["meta"] = metaJSON

	versionID := uuid.New()

	vid, _ := json.Marshal(versionID.String())
	doc["resume_version_id"] = vid

	// Stamp top-level provenance fields too (these live outside meta in the schema).
	genAt, _ := json.Marshal(time.Now().UTC().Format(time.RFC3339))
	doc["generated_at"] = genAt
	appID, _ := json.Marshal(applicationID.String())
	doc["application_id"] = appID

	// Re-marshal the corrected document — this is what we validate AND store.
	corrected, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("generate: re-marshal corrected document: %w", err)
	}
	correctedRaw := json.RawMessage(corrected)

	// Validate the corrected document, not the model's raw output.
	if err := validateResume(correctedRaw); err != nil {
		return nil, fmt.Errorf("generate: schema validation: %w", err)
	}

	versionNum, err := s.q.NextResumeVersionNumber(ctx, db.NextResumeVersionNumberParams{
		ApplicationID: applicationID,
		UserID:        userID,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: next version number: %w", err)
	}

	// Prompt provenance is content-addressed: each ref pins the exact template
	// text by git blob hash, recoverable with `git cat-file -p <blob>`. See
	// promptFingerprint. pipelineVersion covers the call sequence, which no
	// individual file's content describes.
	bodyRef, err := newPromptRef(resumeBodyPrompt)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	summaryRef, err := newPromptRef(resumeSummaryPrompt)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	genParamsJSON, err := json.Marshal(struct {
		Model    string               `json:"model"`
		Pipeline string               `json:"pipeline_version"`
		Prompts  map[string]promptRef `json:"prompts"`
	}{
		Model:    s.client.ModelName(),
		Pipeline: pipelineVersion,
		Prompts: map[string]promptRef{
			"body":    bodyRef,
			"summary": summaryRef,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("generate: marshal gen params: %w", err)
	}
	genParamsRaw := json.RawMessage(genParamsJSON)

	rv, err := s.q.CreateResumeVersion(ctx, db.CreateResumeVersionParams{
		ID:               versionID,
		UserID:           userID,
		ApplicationID:    applicationID,
		VersionNumber:    versionNum,
		GenerationParams: &genParamsRaw,
		StructuredOutput: &correctedRaw,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: store resume version: %w", err)
	}

	return &rv, nil
}

func validateResume(doc json.RawMessage) error {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("resume.v2.json", bytes.NewReader(resumeschema.ResumeV2JSON)); err != nil {
		return fmt.Errorf("load schema: %w", err)
	}
	sch, err := compiler.Compile("resume.v2.json")
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return fmt.Errorf("unmarshal for validation: %w", err)
	}
	if err := sch.Validate(v); err != nil {
		return &ValidationError{Detail: err.Error()}
	}
	return nil
}
