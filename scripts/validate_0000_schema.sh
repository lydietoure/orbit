#!/usr/bin/env bash
# validate_0000_schema.sh
#
# Verifies that migrations/0000_v0.1.0.sql faithfully reproduces the schema
# that was shipped in the v0.1.0 release.
#
# The ground truth is fetched directly from git at the v0.1.0 tag, so this
# script must be run from inside the repository and requires git.
#
# Usage:
#   ./scripts/validate_0000_schema.sh
#
# Exit code 0 → schemas match. Non-zero → mismatch or error (details on stderr).

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

RELEASED_RAW="$TMP/schema_v010_released_raw.sql"

# Ground truth: the schema.sql that was actually in the v0.1.0 binary.
echo "Fetching v0.1.0:internal/db/schema.sql from git..."
git show v0.1.0:internal/db/schema.sql > "$RELEASED_RAW"

bash ./scripts/validate_schema.sh 0000 "$RELEASED_RAW"
