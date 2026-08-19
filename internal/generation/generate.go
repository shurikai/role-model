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
	resumeschema "github.com/shurikai/role-model/schema"
)

// ErrSignalsRequired is returned when jd_signals have not been extracted yet.
var ErrSignalsRequired = errors.New("jd_signals must be extracted before generating a resume")

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
	Identity        string
	Experience      string
	Skills          string
	Projects        string
	Education       string
	Credentials     string
	ResumeSchema    string
	PriorFeedback   string
}

// buildLengthBudget renders a seniority-informed target length as soft
// guidance for the 2a prompt. Output length previously scaled unbounded with
// however much background data existed per role; this gives the model an
// explicit target to allocate bullets against by relevance instead.
func buildLengthBudget(raw *json.RawMessage) (string, error) {
	seniority := ""
	if raw != nil {
		var signals JDSignals
		if err := json.Unmarshal(*raw, &signals); err != nil {
			return "", fmt.Errorf("parse jd_signals for length budget: %w", err)
		}
		seniority = signals.Seniority
	}

	switch seniority {
	case "junior", "mid":
		return "Target 1 page (~8-10 bullets total across ALL positions and projects combined).", nil
	case "staff", "principal", "lead":
		return "Target 2 pages (~15-18 bullets total across ALL positions and projects combined).", nil
	default:
		// senior, manager, director, vp, unknown, or anything unrecognized:
		// senior IC length is the safest general default.
		return "Target 1-2 pages (~12-15 bullets total across ALL positions and projects combined).", nil
	}
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
	CompanyName string
	RoleTitle   string
	JDSignals   string
	HeaderTitle string
	Body        string
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
		"schema_version":   "1.0",
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
		"application_id":   applicationID.String(),
		"target_company":   companyName,
		"target_role":      roleTitle,
		"jd_signals":       signals.forDocument(),
		"generation_model": generationModel,
		// schema/resume.v1.json requires this field and forbids additional
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
	lengthBudget, err := buildLengthBudget(app.JdSignals)
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
		Identity:        string(identityJSON),
		Experience:      string(experienceJSON),
		Skills:          string(skillsJSON),
		Projects:        string(projectsJSON),
		Education:       string(educationJSON),
		Credentials:     string(credentialsJSON),
		ResumeSchema:    string(resumeschema.ResumeV1JSON),
		PriorFeedback:   "",
	})
	if err != nil {
		return nil, fmt.Errorf("generate: render body prompt: %w", err)
	}

	bodyMsg, err := s.client.api.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 4096,
		Model:     s.client.model,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(bodyPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic messages (body): %w", err)
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

	summaryPrompt, err := renderPrompt(resumeSummaryPrompt, resumeSummaryPromptData{
		CompanyName: app.CompanyName,
		RoleTitle:   app.RoleTitle,
		JDSignals:   string(signalsJSON),
		HeaderTitle: headerTitle,
		Body:        string(bodyForSummary),
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
	if err := compiler.AddResource("resume.v1.json", bytes.NewReader(resumeschema.ResumeV1JSON)); err != nil {
		return fmt.Errorf("load schema: %w", err)
	}
	sch, err := compiler.Compile("resume.v1.json")
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
