#!/usr/bin/env bash
set -e

# Rebuilds the docket binary and deploys it to the plugin install location.
# Usage: bash dev-build.sh

SOURCE_DIR="$(cd "$(dirname "$0")" && pwd)"
PLUGIN_INSTALL="$HOME/.claude/plugins/marketplaces/local/docket"

echo "Building docket..."
cd "$SOURCE_DIR"
go build -ldflags="-s -w" -o plugin/docket.exe ./cmd/docket/
echo "Done. $(./plugin/docket.exe version)"

# Kill running docket MCP server so Claude Code restarts it with the new binary
if tasklist 2>/dev/null | grep -qi "docket.exe"; then
  taskkill //F //IM docket.exe >>/dev/null 2>&1 && echo "Killed running docket.exe" || true
fi

# Copy binary to all known install locations
for dest in \
  "$PLUGIN_INSTALL/docket.exe" \
  "$HOME/.claude/plugins/cache/local/docket/0.1.0/docket.exe" \
  "$HOME/.local/share/docket/docket.exe"; do
  if [ -d "$(dirname "$dest")" ]; then
    cp plugin/docket.exe "$dest" && echo "Deployed to $dest"
  fi
done

echo "Run /reload-plugins to restart the MCP server."
