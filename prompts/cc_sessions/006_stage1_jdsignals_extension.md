# CC Session: Stage 1 JDSignals Struct Extension

## Context

Role Model is a self-hostable Go REST API for AI-powered resume generation. Stage 1
is an LLM extraction pass that reads a raw job description and produces structured
signals stored as JSONB in `applications.jd_signals`. Stage 2 (resume synthesis)
reads those signals to tailor output.

The current `JDSignals` struct in `internal/generation/extractor.go` is:

```go
type JDSignals struct {
    PrioritySkills   []string `json:"priority_skills"`
    Seniority        string   `json:"seniority"`
    DomainVocabulary []string `json:"domain_vocabulary"`
}
```

This struct is too thin to support the fit gate and scoring pipeline currently being
built. It lacks a required/preferred skills distinction, domain classification,
work type, and culture signals. This session extends the struct and the Stage 1
extraction prompt to extract richer signal.

**Do NOT implement the fit gate in this session.** That is a separate CC session
that depends on this one completing first.

---

## Target: Extended JDSignals Struct

Update `internal/generation/extractor.go`. Replace the existing `JDSignals` struct
with:

```go
type JDSignals struct {
    // Skills
    RequiredSkills   []string `json:"required_skills"`
    PreferredSkills  []string `json:"preferred_skills"`

    // Role classification
    Seniority        string   `json:"seniority"`
    Domain           string   `json:"domain"`
    WorkType         string   `json:"work_type"`

    // Culture and preference signals
    CultureSignals   []string `json:"culture_signals"`

    // Deprecated: retained for backward compatibility with existing jd_signals rows.
    // Do not use in new code. Will be removed in a future cleanup.
    PrioritySkills   []string `json:"priority_skills,omitempty"`
    DomainVocabulary []string `json:"domain_vocabulary,omitempty"`
}
```

**Field definitions:**
- `required_skills`: technologies, tools, or skills the JD states are required,
  must-have, or core to the role
- `preferred_skills`: technologies or skills the JD states are preferred, nice-to-
  have, a plus, or desired but not required
- `seniority`: one of "junior", "mid", "senior", "staff", "principal", "lead", or
  "unknown" if not determinable
- `domain`: short domain label, e.g. "fintech", "observability", "healthcare",
  "e-commerce", "defense", "saas", "platform", "consumer", or "unknown"
- `work_type`: one of "remote", "hybrid", "onsite", or "unknown"
- `culture_signals`: free-text array of culture and work style signals extracted
  from the JD, e.g. "async-first", "small team", "high ownership", "fast-paced",
  "on-call required", "consulting culture"
- `priority_skills`, `domain_vocabulary`: retained with `omitempty` for backward
  compatibility; not populated by the updated extraction prompt

---

## Target: Updated Extraction Prompt

Update the Stage 1 extraction prompt file at
`internal/generation/prompts/stage1_extraction.txt` (or wherever it currently
lives — check the embed path in extractor.go).

Replace the existing prompt with:

```
You are a structured job description analysis assistant. Extract signals from the
provided job description and return them as a single JSON object.

Return ONLY a valid JSON object with these fields:

required_skills: array of strings — technologies, tools, languages, frameworks, or
  platforms the JD states are required, must-have, or core to the role. Use the
  canonical name as it appears in the JD (e.g. "Go", "Kubernetes", "PostgreSQL").
  Empty array if none stated.

preferred_skills: array of strings — technologies or skills the JD states are
  preferred, nice-to-have, a plus, or desired but not required. Empty array if none.

seniority: string — one of: "junior", "mid", "senior", "staff", "principal",
  "lead", "unknown". Infer from title and requirements if not explicit.

domain: string — one of: "fintech", "observability", "healthcare", "e-commerce",
  "defense", "saas", "platform", "consumer", "edtech", "govtech", "unknown".
  Choose the closest match. Use "unknown" if domain is not determinable.

work_type: string — one of: "remote", "hybrid", "onsite", "unknown".

culture_signals: array of strings — short phrases describing work culture, team
  structure, or work style signals present in the JD. Examples: "async-first",
  "small team", "high ownership", "fast-paced startup", "on-call required",
  "consulting culture", "large enterprise", "cross-functional collaboration".
  Empty array if no clear signals.

Rules:
- Extract only what is present in the JD. Do not infer required skills from context
  if they are not stated.
- Keep skill names concise and canonical — "Go" not "Golang", "Kubernetes" not "K8s".
- Do not include soft skills (communication, teamwork) in required_skills or
  preferred_skills — only technical skills and tools.
- Return ONLY the JSON object. No preamble, no explanation, no markdown fences.

Example output shape:
{
  "required_skills": ["Go", "Kubernetes", "PostgreSQL"],
  "preferred_skills": ["Kafka", "Prometheus"],
  "seniority": "senior",
  "domain": "observability",
  "work_type": "remote",
  "culture_signals": ["small team", "high ownership", "async-first"]
}
```

---

## Target: No Migration Required

`applications.jd_signals` is JSONB. The struct change is backward compatible:
- Existing rows with `priority_skills` and `domain_vocabulary` will deserialize
  correctly due to `omitempty` on those fields
- New extractions will populate the new fields
- No migration needed

Do NOT create a migration for this change.

---

## Target: Verify Existing Stage 2 Still Works

Stage 2 (resume synthesis) reads `jd_signals` to assemble generation context.
Verify that the Stage 2 context assembly code in `internal/generation/` (likely
`assembler.go` or similar) compiles cleanly with the updated struct. If Stage 2
references `PrioritySkills` or `DomainVocabulary` directly, update those references
to use the new fields:
- `PrioritySkills` → use `RequiredSkills` (append `PreferredSkills` if needed for
  backward parity)
- `DomainVocabulary` → use `CultureSignals` or `Domain` as appropriate to context

Do not change Stage 2 generation behavior — only update field references to compile.

---

## Verification Steps

```bash
# 1. Build clean
go build ./...

# 2. Existing tests pass
go test ./...

# 3. Smoke test Stage 1 extraction with updated prompt
TOKEN="<your_jwt>"

APP=$(curl -s -X POST http://localhost:8080/applications \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "company_name": "Acme Observability",
    "role_title": "Senior Backend Engineer",
    "jd_text": "We are looking for a senior backend engineer with strong Go and Kubernetes experience. PostgreSQL required. Kafka and Prometheus a plus. Remote-first, small autonomous teams, high ownership culture."
  }')
APP_ID=$(echo $APP | jq -r '.id')

curl -s -X POST http://localhost:8080/applications/$APP_ID/extract \
  -H "Authorization: Bearer $TOKEN" | jq .

# Expected shape:
# {
#   "required_skills": ["Go", "Kubernetes", "PostgreSQL"],
#   "preferred_skills": ["Kafka", "Prometheus"],
#   "seniority": "senior",
#   "domain": "observability",
#   "work_type": "remote",
#   "culture_signals": ["small team", "high ownership", "remote-first"]
# }

# 4. Verify Stage 2 still generates cleanly against the same application
curl -s -X POST http://localhost:8080/applications/$APP_ID/generate \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Both extraction and generation should return 200 with expected shapes. The
extraction response should contain the new fields. Generation should be unaffected.

