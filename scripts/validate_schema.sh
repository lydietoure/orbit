#!/usr/bin/env bash
# validate_schema.sh
#
# Verifies that applying migrations 0000..VERSION produces the same schema
# as a reference SQL file.
#
# Usage:
#   ./scripts/validate_schema.sh VERSION REFERENCE_SQL
#
#   VERSION        migration version to apply up to, inclusive (e.g. 0001 or 1)
#   REFERENCE_SQL  path to a SQL file expected to produce the same schema
#
# Examples:
#   ./scripts/validate_schema.sh 0001 ./internal/db/schema.sql
#   ./scripts/validate_schema.sh 0000 /tmp/schema_v010_released.sql
#
# Exit code 0 → schemas match. Non-zero → mismatch or error (details on stderr).

set -euo pipefail

if [[ $# -ne 2 ]]; then
    echo "usage: $0 VERSION REFERENCE_SQL" >&2
    echo "example: $0 0001 ./internal/db/schema.sql" >&2
    exit 1
fi

VERSION_ARG="$1"
REFERENCE_SQL="$2"

# Force base-10 so leading zeros don't trip up the comparison.
TARGET_VERSION=$(( 10#${VERSION_ARG} ))

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

if [[ ! -f "$REFERENCE_SQL" ]]; then
    echo "error: reference SQL file not found: $REFERENCE_SQL" >&2
    exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

REFERENCE_NORM="$TMP/reference_norm.sql"
FROM_MIGRATIONS="$TMP/from_migrations.sql"
GENSCHEMA="$TMP/genschema"

echo "Building genschema..."
go build -tags ci -o "$GENSCHEMA" ./cmd/genschema/

# Collect every migration file whose numeric prefix is <= TARGET_VERSION.
MIGRATIONS_DIR="internal/db/migrations"
SELECTED=()

shopt -s nullglob
for f in "$MIGRATIONS_DIR"/*.sql; do
    base="$(basename "$f")"
    prefix="${base%%_*}"
    file_version=$(( 10#${prefix} ))
    if [[ $file_version -le $TARGET_VERSION ]]; then
        SELECTED+=("$base")
    fi
done

# Ensure numeric ordering even if filenames aren't zero-padded.
if [[ ${#SELECTED[@]} -gt 0 ]]; then
    mapfile -t SELECTED < <(printf '%s\n' "${SELECTED[@]}" | sort -t_ -k1,1n)
fi

if [[ ${#SELECTED[@]} -eq 0 ]]; then
    echo "error: no migration files with version <= $TARGET_VERSION found in $MIGRATIONS_DIR" >&2
    exit 1
fi

echo "Applying migrations: ${SELECTED[*]}"
MIGRATIONS_ARG="$(IFS=,; echo "${SELECTED[*]}")"
"$GENSCHEMA" -migrations "$MIGRATIONS_ARG" -out "$FROM_MIGRATIONS"

echo "Normalising reference: $REFERENCE_SQL"
"$GENSCHEMA" -sql "$REFERENCE_SQL" -out "$REFERENCE_NORM"

echo "Comparing..."
if diff "$REFERENCE_NORM" "$FROM_MIGRATIONS"; then
    echo "OK: migrations 0000..$(printf '%04d' "$TARGET_VERSION") match $REFERENCE_SQL"
else
    echo "FAIL: schemas differ. See diff above." >&2
    exit 1
fi
