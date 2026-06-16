package generation

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed prompts/*.tmpl
var promptFS embed.FS

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
