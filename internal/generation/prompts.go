package generation

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
)

// promptVersion identifies the overall generation pipeline shape. Bumped when
// the sequence/contract of prompt calls changes (e.g. the 2a/2b summary split).
const promptVersion = "v2"

// Individual prompt template versions, recorded in generation_params for
// per-call traceability now that generation is more than one LLM call.
const (
	bodyPromptVersion    = "resume_body.v1"
	summaryPromptVersion = "resume_summary.v1"
)

//go:embed prompts/*.tmpl
var promptFS embed.FS

//go:embed prompts/*.txt
var rawPromptFS embed.FS

var templates = template.Must(
	template.ParseFS(promptFS, "prompts/*.tmpl"),
)

// renderPrompt executes a named template with the given data.
func renderPrompt(name string, data any) (string, error) {
	var buf strings.Builder
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render prompt %q: %w", name, err)
	}
	return buf.String(), nil
}

// RawPrompt returns the contents of a static (non-templated) prompt file by
// name, e.g. "stage0a_extraction.txt". Used by pipelines that don't need
// text/template substitution.
func RawPrompt(name string) (string, error) {
	b, err := rawPromptFS.ReadFile("prompts/" + name)
	if err != nil {
		return "", fmt.Errorf("read prompt %q: %w", name, err)
	}
	return string(b), nil
}
