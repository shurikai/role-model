package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type resumePromptData struct {
	CompanyName   string
	RoleTitle     string
	JDSignals     string
	Identity      string
	Experience    string
	Projects      string
	Education     string
	Credentials   string
	ResumeSchema  string
	PriorFeedback string
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
	identityJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal identity: %w", err)
	}
	experienceJSON, err := json.MarshalIndent(resumeCtx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate: marshal experience: %w", err)
	}

	prompt, err := renderPrompt("resume_generation.v1.tmpl", resumePromptData{
		CompanyName:   app.CompanyName,
		RoleTitle:     app.RoleTitle,
		JDSignals:     string(signalsJSON),
		Identity:      string(identityJSON),
		Experience:    string(experienceJSON),
		Projects:      "[]",
		Education:     "[]",
		Credentials:   "[]",
		ResumeSchema:  string(resumeschema.ResumeV1JSON),
		PriorFeedback: "",
	})
	if err != nil {
		return nil, fmt.Errorf("generate: render prompt: %w", err)
	}

	msg, err := s.client.api.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 4096,
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

	cleaned := stripCodeFence(raw)

	// Parse into a key-preserving map so untouched fields pass through byte-for-byte.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &doc); err != nil {
		return nil, fmt.Errorf("generate: parse resume json: %w (raw: %s)", err, cleaned)
	}

	// Overwrite provenance fields the model must not own. The model generates
	// resume *content*; the generator stamps the *facts* about generation.
	meta := map[string]any{
		"schema_version":   "1.0",
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
		"application_id":   applicationID.String(),
		"target_company":   app.CompanyName,
		"target_role":      app.RoleTitle,
		"jd_signals":       app.JdSignals,
		"generation_model": s.client.ModelName(),
		"prompt_version":   promptVersion,
	}
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

	genParamsJSON, err := json.Marshal(struct {
		Model         string `json:"model"`
		PromptVersion string `json:"prompt_version"`
	}{
		Model:         s.client.ModelName(),
		PromptVersion: promptVersion,
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
