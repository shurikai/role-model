package generation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"

	resumeschema "github.com/shurikai/role-model/schema"
)

// The tracked pipeline fixtures must validate against the document schema.
//
// tests/fixtures/*.json are shared: the Go side treats them as examples of what
// generation produces, and docx-renderer/tests/conftest.py loads the same files
// as examples of what the renderer consumes. Nothing checked they were valid
// documents. The renderer's Pydantic models ignore unknown fields and default
// missing optional ones, so a fixture could drift away from the schema in both
// directions and stay green on the Python side forever — which is how they came
// to carry a jd_signals block in the deprecated priority_skills shape that
// `additionalProperties: false` forbids.
//
// This is the cheap half of the contract the renderer cannot enforce.
func TestTrackedFixturesValidateAgainstSchema(t *testing.T) {
	c := jsonschema.NewCompiler()
	if err := c.AddResource("resume.v2.json", bytes.NewReader(resumeschema.ResumeV2JSON)); err != nil {
		t.Fatalf("load schema: %v", err)
	}
	sch, err := c.Compile("resume.v2.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join("..", "..", "tests", "fixtures", "sample_*.json"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no tracked document fixtures found")
	}

	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var v any
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if err := sch.Validate(v); err != nil {
				t.Errorf("does not validate against resume.v2.json: %v", err)
			}
		})
	}
}
