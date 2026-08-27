#!/usr/bin/env bash
#
# Refuse a migration numbered at or below one that already exists on the base
# branch.
#
# golang-migrate applies only versions ABOVE the database's current version. A
# file numbered below it is skipped forever, `migrate up` reports "no change"
# rather than failing, and the schema silently diverges from the migration
# directory. It has already happened here once —
# migrations/018_restore_tag_category_aliases.up.sql exists because 012's data
# statements were skipped on every database that had passed 012 already (#92,
# and #74 for what it cost).
#
# Two branches each adding "025" is the ordinary way in: both merge, both look
# applied, and whichever lands second is skipped on every database that ran the
# first.
#
# Usage:
#   scripts/check-migration-order.sh [base-ref]      # default: origin/main
set -euo pipefail

BASE="${1:-origin/main}"
MIGRATIONS_DIR="migrations"

# Highest numeric prefix among the .up.sql files in a given ref, or 0 when the
# ref has no migrations at all.
max_version_in_ref() {
    local ref="$1"
    git ls-tree -r --name-only "$ref" -- "$MIGRATIONS_DIR" 2>/dev/null \
        | grep -E '\.up\.sql$' \
        | sed -E 's|.*/([0-9]+)_.*|\1|' \
        | sed 's/^0*//' \
        | sort -n | tail -1 || true
}

if ! git rev-parse --verify --quiet "$BASE" >/dev/null; then
    echo "check-migration-order: base ref '$BASE' not found; skipping."
    echo "  (fetch it first, e.g. git fetch origin main)"
    exit 0
fi

base_max="$(max_version_in_ref "$BASE")"
base_max="${base_max:-0}"

# Files present now that the base does not have.
#
# Compared against the WORKING TREE rather than HEAD, so this catches a bad
# migration before it is committed as well as in CI, where the two are the
# same thing. A check that only fires after you commit is a check you run
# after you have already pushed.
base_files="$(git ls-tree -r --name-only "$BASE" -- "$MIGRATIONS_DIR" 2>/dev/null \
              | grep -E '\.up\.sql$' | sort || true)"
head_files="$(ls "$MIGRATIONS_DIR"/*.up.sql 2>/dev/null | sort || true)"
added="$(comm -13 <(printf '%s\n' "$base_files") <(printf '%s\n' "$head_files"))"

if [ -z "$added" ]; then
    echo "check-migration-order: no new migrations (base $BASE is at $base_max)."
    exit 0
fi

status=0
while IFS= read -r file; do
    [ -n "$file" ] || continue
    version="$(basename "$file" | sed -E 's|^0*([0-9]+)_.*|\1|')"
    if [ "$version" -le "$base_max" ]; then
        echo "ERROR: $file is version $version, but $BASE already has $base_max."
        echo "       golang-migrate applies only versions above the current one, so this"
        echo "       file would be SKIPPED FOREVER on any database already past $base_max —"
        echo "       and 'migrate up' would report 'no change' rather than failing."
        echo "       Renumber it above $base_max."
        status=1
    else
        echo "ok: $file (version $version > base $base_max)"
    fi
done <<< "$added"

# Two new migrations sharing a number is the same failure one step earlier:
# whichever applies first wins and the other is skipped.
dupes="$(printf '%s\n' "$added" | xargs -r -n1 basename \
         | sed -E 's|^([0-9]+)_.*|\1|' | sort | uniq -d || true)"
if [ -n "$dupes" ]; then
    echo "ERROR: more than one new migration shares a version number:"
    printf '  %s\n' $dupes
    status=1
fi

exit $status
