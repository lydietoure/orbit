#!/usr/bin/env bash
# create_v0_1_0_app_data.sh
#
# Creates a v0.1.0 orbit database seeded with sample data.
# Useful for regenerating the adoption/upgrade test fixture in internal/db/migrate_test.go
#
# Usage:
#   ./scripts/create_v0_1_0_app_data.sh /path/to/orbit-v0.1.0 /path/to/output.db

set -euo pipefail

if [[ $# -ne 2 ]]; then
    echo "usage: $0 /path/to/orbit_executable /path/to/output.db" >&2
    exit 1
fi

ORBIT_EXE="$1"
OUTPUT_DB="$2"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Creating a new Orbit app using $ORBIT_EXE..."
export ORBIT_HOME="$TMP/home"
echo "Setting the home directory to $ORBIT_HOME ..."
mkdir -p "$ORBIT_HOME"

# Initialize a new Orbit app with the v0.1.0 schema.
echo "Initializing a new Orbit 0.1.0 app..."
"$ORBIT_EXE" init

# Creating some sample data in the new app
echo "Creating a new work entry..."
"$ORBIT_EXE" work new "Sample work entry for v0.1.0 app"
"$ORBIT_EXE" work tag "test"
"$ORBIT_EXE" work tag "v0.1.0"
"$ORBIT_EXE" work show

echo "Creating a second work entry..."
"$ORBIT_EXE" work new "Dictionary notes" --no-select

echo "Listing work entries"
"$ORBIT_EXE" work list

cp "$ORBIT_HOME/orbit.db" "$OUTPUT_DB"
echo "Done. App data for v0.1.0 has been created in $OUTPUT_DB"