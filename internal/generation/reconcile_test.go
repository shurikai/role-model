package generation

import (
	"encoding/json"
	"testing"
)

func TestNamedIn(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		term string
		want bool
	}{
		{name: "plain word", text: "Built services in Java 17", term: "Java", want: true},
		{name: "case insensitive", text: "built with SPRING BOOT", term: "Spring Boot", want: true},
		{name: "trailing comma", text: "Used Kafka, Redis and more", term: "Kafka", want: true},

		// The reason this is not a substring match. Each of these would put a
		// skill on a resume because a bullet mentioned a different one.
		{name: "Go inside Golang", text: "Wrote Golang services", term: "Go", want: false},
		{name: "Go inside Google", text: "Deployed to Google Cloud", term: "Go", want: false},
		{name: "Java inside JavaScript", text: "Wrote JavaScript for the UI", term: "Java", want: false},
		{name: "REST inside RESTful", text: "Designed RESTful endpoints", term: "REST", want: false},
		{name: "Git inside GitHub", text: "Ran GitHub Actions", term: "Git", want: false},

		// Punctuated names, where a regexp \b would put the boundary in the
		// wrong place.
		{name: "C++ surrounded by spaces", text: "Integrated via C++ plugins", term: "C++", want: true},
		{name: "C# before a slash", text: "Built in C#/.NET on the team", term: "C#", want: true},
		{name: "dot-NET after a slash", text: "Built in C#/.NET on the team", term: ".NET", want: true},
		{name: "C# at end of text", text: "The service was written in C#", term: "C#", want: true},
		{name: "CI/CD", text: "Owned CI/CD pipelines", term: "CI/CD", want: true},

		// A later occurrence must still be found after an earlier rejection.
		{name: "rejected then accepted", text: "Golang is not Go", term: "Go", want: true},

		{name: "absent", text: "Built services in Java", term: "Rust", want: false},
		{name: "empty term", text: "anything", term: "", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := namedIn(tc.text, tc.term); got != tc.want {
				t.Errorf("namedIn(%q, %q) = %v, want %v", tc.text, tc.term, got, tc.want)
			}
		})
	}
}

func skillsOf(t *testing.T, doc map[string]json.RawMessage) map[string][]string {
	t.Helper()
	var got map[string][]string
	if err := json.Unmarshal(doc["skills"], &got); err != nil {
		t.Fatalf("unmarshal skills: %v", err)
	}
	return got
}

func TestReconcileSkillsReAddsBulletCitedSkills(t *testing.T) {
	doc := map[string]json.RawMessage{
		"skills": json.RawMessage(`{"Languages":["Java (25 yrs, expert)"]}`),
		"experience": json.RawMessage(`[{"positions":[{"bullets":[
			{"text":"Maintained 80%+ test coverage with JUnit and Mockito"},
			{"text":"Led 7 engineers in C# on .NET 4"}
		]}]}]`),
	}
	claimed := []SkillView{
		{Name: "Java", Category: "Languages", Proficiency: "expert"},
		{Name: "C#", Category: "Languages", Proficiency: "novice"},
		{Name: "JUnit", Category: "Testing", Proficiency: "expert"},
		{Name: "Mockito", Category: "Testing", Proficiency: "proficient"},
		{Name: ".NET", Category: "Frameworks & Libraries", Proficiency: "proficient"},
		{Name: "Rust", Category: "Languages", Proficiency: "novice"},
	}

	if err := reconcileSkills(doc, claimed); err != nil {
		t.Fatalf("reconcileSkills: %v", err)
	}
	got := skillsOf(t, doc)

	// Java is already listed under an annotation and must not be duplicated.
	if want := []string{"Java (25 yrs, expert)", "C#"}; !equalStrings(got["Languages"], want) {
		t.Errorf("Languages = %q, want %q", got["Languages"], want)
	}
	// A category absent from the emitted object is created, not dropped.
	if want := []string{"JUnit", "Mockito"}; !equalStrings(got["Testing"], want) {
		t.Errorf("Testing = %q, want %q", got["Testing"], want)
	}
	if want := []string{".NET"}; !equalStrings(got["Frameworks & Libraries"], want) {
		t.Errorf("Frameworks = %q, want %q", got["Frameworks & Libraries"], want)
	}
	// Claimed but named by no bullet: not re-added.
	for _, s := range got["Languages"] {
		if s == "Rust" {
			t.Error("Rust was added without any bullet naming it")
		}
	}
}

// The Skills section may only ever carry claimed skills. A bullet naming a
// product, customer, or framework the user has no skills row for must not
// manufacture one.
func TestReconcileSkillsIgnoresUnclaimedTechnologies(t *testing.T) {
	doc := map[string]json.RawMessage{
		"skills": json.RawMessage(`{"Languages":["Java"]}`),
		"experience": json.RawMessage(`[{"positions":[{"bullets":[
			{"text":"Integrated off-the-shelf WAGO hardware alongside NGTS"}
		]}]}]`),
	}

	if err := reconcileSkills(doc, []SkillView{{Name: "Java", Category: "Languages"}}); err != nil {
		t.Fatalf("reconcileSkills: %v", err)
	}
	got := skillsOf(t, doc)

	if len(got) != 1 || !equalStrings(got["Languages"], []string{"Java"}) {
		t.Errorf("skills = %v, want only the claimed Java", got)
	}
}

// The renderer prints categories in document order, so adding one entry must
// not re-alphabetize the section as a side effect.
func TestReconcileSkillsPreservesCategoryOrder(t *testing.T) {
	doc := map[string]json.RawMessage{
		"skills": json.RawMessage(`{"Languages":["Java"],"Cloud & Infrastructure":["AWS"],"Databases":["PostgreSQL"]}`),
		"experience": json.RawMessage(`[{"positions":[{"bullets":[
			{"text":"Shipped with Docker on ECS"}
		]}]}]`),
	}
	claimed := []SkillView{{Name: "Docker", Category: "Cloud & Infrastructure"}}

	if err := reconcileSkills(doc, claimed); err != nil {
		t.Fatalf("reconcileSkills: %v", err)
	}

	order, _, err := decodeOrderedSkills(doc["skills"])
	if err != nil {
		t.Fatalf("decodeOrderedSkills: %v", err)
	}
	want := []string{"Languages", "Cloud & Infrastructure", "Databases"}
	if !equalStrings(order, want) {
		t.Errorf("category order = %q, want %q", order, want)
	}
}

// Nothing to add must leave the document byte-for-byte untouched, so an
// unchanged run cannot be told apart from one that never ran.
func TestReconcileSkillsNoopLeavesDocumentUnchanged(t *testing.T) {
	original := json.RawMessage(`{"Languages":["Java (25 yrs, expert)","Go"]}`)
	doc := map[string]json.RawMessage{
		"skills":     original,
		"experience": json.RawMessage(`[{"positions":[{"bullets":[{"text":"Wrote Java and Go services"}]}]}]`),
	}

	if err := reconcileSkills(doc, []SkillView{
		{Name: "Java", Category: "Languages"},
		{Name: "Go", Category: "Languages"},
	}); err != nil {
		t.Fatalf("reconcileSkills: %v", err)
	}

	if string(doc["skills"]) != string(original) {
		t.Errorf("skills rewritten with no change: %s", doc["skills"])
	}
}

func TestReconcileSkillsHandlesMissingBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  map[string]json.RawMessage
	}{
		{name: "no skills key", doc: map[string]json.RawMessage{}},
		{name: "no bullets anywhere", doc: map[string]json.RawMessage{
			"skills": json.RawMessage(`{"Languages":["Java"]}`),
		}},
		{name: "projects only", doc: map[string]json.RawMessage{
			"skills":   json.RawMessage(`{"Languages":["Java"]}`),
			"projects": json.RawMessage(`[{"bullets":[{"text":"Built a CLI in Go"}]}]`),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := reconcileSkills(tc.doc, []SkillView{{Name: "Go", Category: "Languages"}})
			if err != nil {
				t.Fatalf("reconcileSkills: %v", err)
			}
		})
	}
}

// A project bullet is a bullet. A skill demonstrated only there still counts.
func TestReconcileSkillsReadsProjectBullets(t *testing.T) {
	doc := map[string]json.RawMessage{
		"skills":   json.RawMessage(`{"Languages":["Java"]}`),
		"projects": json.RawMessage(`[{"bullets":[{"text":"Built a photo deduplication CLI in Go"}]}]`),
	}

	if err := reconcileSkills(doc, []SkillView{{Name: "Go", Category: "Languages"}}); err != nil {
		t.Fatalf("reconcileSkills: %v", err)
	}
	got := skillsOf(t, doc)

	if want := []string{"Java", "Go"}; !equalStrings(got["Languages"], want) {
		t.Errorf("Languages = %q, want %q", got["Languages"], want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
