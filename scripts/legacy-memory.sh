#!/usr/bin/env bash
set -euo pipefail

# Legacy compatibility wrapper for pre-ENGRAM memory scripts.
# Routes writes/search through engramctl when engramd is healthy.
# Falls back to LEGACY_MEMORY_SCRIPT when provided.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_PATH="${ENGRAM_CONFIG:-$ROOT_DIR/configs/example.yaml}"
ENGRAMCTL="${ENGRAMCTL_BIN:-$ROOT_DIR/bin/engramctl}"
LEGACY_SCRIPT="${LEGACY_MEMORY_SCRIPT:-}"

if [[ $# -lt 1 ]]; then
  echo "usage: legacy-memory.sh <search|curate|get|history|correct|deprecate|ingest> [args...]" >&2
  exit 2
fi

command="$1"
shift

if [[ -x "$ENGRAMCTL" ]] && "$ENGRAMCTL" health --config "$CONFIG_PATH" >/dev/null 2>&1; then
  exec "$ENGRAMCTL" "$command" --config "$CONFIG_PATH" "$@"
fi

if [[ -n "$LEGACY_SCRIPT" ]]; then
  exec "$LEGACY_SCRIPT" "$command" "$@"
fi

echo "engramd unavailable and no LEGACY_MEMORY_SCRIPT configured" >&2
exit 1
