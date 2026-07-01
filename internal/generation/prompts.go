package generation

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
)

const promptVersion = "v1"

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
