#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
ethdir="$workspace/src/github.com/InsighterInc"
if [ ! -L "$ethdir/bxmp" ]; then
    mkdir -p "$ethdir"
    cd "$ethdir"
    ln -s ../../../../../. bxmp
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"
export GOPATH

# Run the command inside the workspace.
cd "$ethdir/bxmp"
PWD="$ethdir/bxmp"

# Launch the arguments with the configured environment.
exec "$@"
