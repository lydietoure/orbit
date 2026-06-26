#!/usr/bin/env bash
# create_v0_1_0_app.sh
#
# Seeds a v0.1.0 orbit DB with sample data for adoption/upgrade testing.
#
# Usage:
#   ./scripts/create_v0_1_0_app.sh /path/to/orbit-v0.1.0 [orbit_home]
#
#   orbit_home  defaults to internal/db/testdata/v0_1_0_app/home

set -euo pipefail

if [[ $# -lt 1 || $# -gt 3 ]]; then
    echo "usage: $0 /path/to/orbit_executable [orbit_home] [orbit_dock]" >&2
    exit 1
fi

ORBIT_EXE="$1"

ROOT_DIR="$(git rev-parse --show-toplevel)"
cd "$ROOT_DIR"
DEFAULT_APP_ROOT="$ROOT_DIR/internal/db/testdata/v0_1_0_app"

ORBIT_HOME="${2:-$DEFAULT_APP_ROOT/home}"
ORBIT_DOCK="${3:-$DEFAULT_APP_ROOT/dock}"

# if [[ -z "$HOME_DIR" ]]; then
#     $ORBIT_HOME = "$ROOT_DIR/internal/db/testdata/v0_1_0_app/home"
# fi
# if [[ -z "$ORBIT_DOCK" ]]; then
#     $ORBIT_DOCK = "$ROOT_DIR/internal/db/testdata/v0_1_0_app/dock"
# fi

echo "Creating a new Orbit app using $ORBIT_EXE..."
echo "Setting the home directory to $ORBIT_HOME ..."
export ORBIT_HOME

echo "Setting the verbosity to DEBUG ..."
export ORBIT_VERBOSE=1

# Initialize a new Orbit app with the v0.1.0 schema.
echo "Initializing a new Orbit 0.1.0 app..."
"$ORBIT_EXE" init

# Set the dock directory to the specified path.
echo "Setting the dock directory to $ORBIT_DOCK ..."
"$ORBIT_EXE" config dock set "$ORBIT_DOCK"

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

echo "Done. App data for v0.1.0 has been created in $ORBIT_HOME/orbit.db"


# TODO: This script should just create a database. So it should accept a output path for the database file