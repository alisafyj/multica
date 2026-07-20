#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_DIR="$ROOT_DIR/apps/figma-plugin"
OUTPUT_PATH="${1:-$PLUGIN_DIR/dist/multica-figma-plugin.zip}"
PACKAGE_NAME="Multica-Figma-Plugin"
STAGING_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$STAGING_DIR"
}
trap cleanup EXIT

mkdir -p "$STAGING_DIR/$PACKAGE_NAME" "$(dirname "$OUTPUT_PATH")"
cp \
  "$PLUGIN_DIR/manifest.json" \
  "$PLUGIN_DIR/code.js" \
  "$PLUGIN_DIR/ui.html" \
  "$PLUGIN_DIR/README.md" \
  "$PLUGIN_DIR/INSTALL.md" \
  "$STAGING_DIR/$PACKAGE_NAME/"

rm -f "$OUTPUT_PATH"
(
  cd "$STAGING_DIR"
  zip -q -r "$OUTPUT_PATH" "$PACKAGE_NAME"
)

printf '%s\n' "$OUTPUT_PATH"
