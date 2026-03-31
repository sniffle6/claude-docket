#!/usr/bin/env bash
set -e

# Rebuilds the docket binary into plugin/ for symlink-based dev workflow.
# After running: the live plugin binary is updated (no install step needed).
# Usage: bash dev-build.sh

SOURCE_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Building docket into plugin/docket.exe..."
cd "$SOURCE_DIR"
go build -ldflags="-s -w" -o plugin/docket.exe ./cmd/docket/
echo "Done. $(./plugin/docket.exe version)"
echo "Restart Claude Code or start a new session to pick up binary changes."
