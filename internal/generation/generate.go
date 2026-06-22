package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	var resumeDoc json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &resumeDoc); err != nil {
		return nil, fmt.Errorf("generate: parse resume json: %w (raw: %s)", err, cleaned)
	}

	if err := validateResume(resumeDoc); err != nil {
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
		Model:         string(s.client.model),
		PromptVersion: "v1",
	})
	if err != nil {
		return nil, fmt.Errorf("generate: marshal gen params: %w", err)
	}
	genParamsRaw := json.RawMessage(genParamsJSON)

	rv, err := s.q.CreateResumeVersion(ctx, db.CreateResumeVersionParams{
		UserID:           userID,
		ApplicationID:    applicationID,
		VersionNumber:    versionNum,
		GenerationParams: &genParamsRaw,
		StructuredOutput: &resumeDoc,
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
