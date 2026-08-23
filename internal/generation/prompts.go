package generation

import (
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"fmt"
	"strings"
	"text/template"
)

// pipelineVersion identifies the overall generation pipeline shape -- how many
// LLM calls run, in what order, and what each is responsible for. Bumped when
// that sequence changes (e.g. the 2a body / 2b summary split), NOT when the
// text of an individual prompt changes.
//
// This is the one prompt-related constant still maintained by hand, because it
// names something no single file's content captures. Individual prompt versions
// are content-addressed instead -- see promptFingerprint.
const pipelineVersion = "v2"

//go:embed prompts/*.tmpl
var promptFS embed.FS

//go:embed prompts/*.txt
var rawPromptFS embed.FS

var templates = template.Must(
	template.ParseFS(promptFS, "prompts/*.tmpl"),
)

// Prompt template filenames. These are deliberately unversioned: a prompt's
// identity is the content hash recorded in generation_params, and its history
// is the git log of the file. Renaming per revision would reintroduce two
// sources of truth for one fact, which is what this scheme exists to avoid.
const (
	jdExtractionPrompt  = "jd_extraction.tmpl"
	careerExtractPrompt = "stage0_career_extraction.tmpl"
	resumeBodyPrompt    = "resume_body.tmpl"
	resumeSummaryPrompt = "resume_summary.tmpl"
)

// renderPrompt executes a named template with the given data.
func renderPrompt(name string, data any) (string, error) {
	var buf strings.Builder
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render prompt %q: %w", name, err)
	}
	return buf.String(), nil
}

// CareerExtractionPromptData is what stage0_career_extraction.tmpl renders
// against. ProficiencyValues comes from the user's own proficiency_levels rows,
// for the same reason the JD extraction prompt's seniority list does: the scale
// the extractor is told to choose from and the scale the fit gate ranks against
// have to be one scale.
type CareerExtractionPromptData struct {
	CareerText        string
	ProficiencyValues string
}

// RenderCareerExtractionPrompt renders the Stage 0 career extraction prompt.
func RenderCareerExtractionPrompt(data CareerExtractionPromptData) (string, error) {
	return renderPrompt(careerExtractPrompt, data)
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

// promptFingerprint returns the git blob SHA of an embedded prompt file. This
// is the prompt's version: it changes exactly when the content changes, and it
// is identical to the hash git computes for the same bytes, so a fingerprint
// recorded in generation_params round-trips against the repository:
//
//	git cat-file -p <sha>           -- the exact prompt text
//	git log --find-object=<sha>     -- the commit that introduced it
//	git diff <shaA> <shaB>          -- what changed between two generations
//
// Caveat: those lookups only resolve once the content is committed. A prompt
// edited but not yet committed still fingerprints correctly and stably, but
// the blob exists nowhere retrievable -- see `make check-prompts`.
//
// SHA-1 here is git's object format, not a security boundary.
func promptFingerprint(name string) (string, error) {
	b, err := promptFS.ReadFile("prompts/" + name)
	if err != nil {
		return "", fmt.Errorf("fingerprint prompt %q: %w", name, err)
	}

	// Git hashes a blob as sha1("blob <bytelen>\x00" + content).
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// promptRef identifies one prompt used during a generation run: the file it
// came from, and the content hash that pins the exact text.
type promptRef struct {
	File string `json:"file"`
	Blob string `json:"blob"`
}

func newPromptRef(name string) (promptRef, error) {
	blob, err := promptFingerprint(name)
	if err != nil {
		return promptRef{}, err
	}
	return promptRef{File: name, Blob: blob}, nil
}
