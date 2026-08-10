# Session 030 — ROADMAP.md State Refresh

## Objective

Update ROADMAP.md to match the actual current state of the repo. This is a
mechanical pass — fix what's factually wrong, don't change architectural intent
or add new sections. A separate session will handle architectural updates.

## Pre-flight

```bash
git pull
gh issue list --state open
```

Confirm you're on `main` and note the current open issue count.

## Instructions

Read `ROADMAP.md` in full. Then check each section below against the actual
repo state and fix any drift.

### 1. Companion docs path

The ROADMAP says `docs/discovery-design.md`. The actual file is at
`notes/discovery-design.md`. Fix the reference. Also add `CLAUDE.md` to the
companion docs list if it's not already there.

### 2. Seed data file count and inventory

The ROADMAP lists 12 seed files (001-012). The actual seed repo has 16 files
(001-016). Update the inventory to include all 16 files with accurate
one-line descriptions. Use the existing files as the source of truth for
what each one does.

The two "Pending seed tasks" listed after the inventory are both resolved:
- DynamoDB tag/contribution text was confirmed and closed in 012
- Groovy tag verification was confirmed and closed in 012

Remove or mark these as completed. If there are any actually-pending seed
tasks (stale TODO in 013, duplicate insert across 013/015), note those
instead, but mark them as low-priority cleanup.

### 3. Migration count

The ROADMAP says "9 migrations applied." Count the actual migration files in
`migrations/` and update the number.

### 4. Schema table count

The ROADMAP says "22 tables + 1 view." Run a count against the actual schema
(either from migration files or by querying the database if running) and
update if the number has changed.

### 5. Open issues count

Don't put a count in the ROADMAP (it changes too often), but check that any
issues referenced by number in the ROADMAP still exist and are in the
correct state (open vs. closed).

### 6. Endpoint inventory

Scan `internal/` for route registrations and compare against the endpoint
list in the ROADMAP. Add any missing endpoints. Remove any that no longer
exist. Don't change the format — keep the same `GET/POST/PUT/DELETE` style.

### 7. Known Skill Gaps table

Check against the current state:
- The "React" row should note the Role Model frontend as active work, not
  future tense if frontend code already exists
- The "Go at production API depth" row same — Role Model backend is
  substantially built
- The "MongoDB/NoSQL" row should be clear that DynamoDB (not MongoDB) is the
  confirmed NoSQL experience, specifically Global Tables at Edward Jones
- If Angular is listed, make sure the framing is accurate: real but thin
  exposure, not zero signal

### 8. Hard-Pass Filters table

Current state (do NOT add the FDE exception or other architectural changes
in this pass — that's the next session):
- The "defense / aerospace" line should say "defense-coded / clearance-required"
  not "defense / aerospace" (commercial aerospace is not excluded)
- Add "Crypto/blockchain product companies" if not present
- Add "Onsite outside Orlando, FL" if not present
- Add "Ruby/Rails as primary language" if not present
- Add "C# / .NET as primary stack" if not present

### 9. Renderer section

The ROADMAP's Phase 2 renderer entry should reflect the built state. Check
that `docx-renderer/` exists in the repo and that the description matches
what's actually there (FastAPI + python-docx, POST /render endpoint).

### 10. Open Questions

Review each numbered item. If any have been resolved by shipped code, mark
them resolved with a one-line note of what landed. Don't remove them — mark
them so the history is visible.

## Validation

After making changes, do a final read-through of the updated ROADMAP.md and
confirm:
- No section claims something is "pending" or "TBD" that is actually built
- No file counts, table counts, or migration counts are wrong
- No paths reference files that don't exist at that path

## Commit

```bash
git add ROADMAP.md
git commit -m "docs: refresh ROADMAP.md state to match current repo"
```
