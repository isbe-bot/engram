#!/usr/bin/env bash
set -euo pipefail

PREFIX="${PREFIX:-/usr/local}"
CONFIG_DIR="${CONFIG_DIR:-/etc/engram}"
DATA_DIR="${DATA_DIR:-/var/lib/engram}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
DRY_RUN="${DRY_RUN:-0}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    printf '[dry-run] %q ' "$@"
    printf '\n'
  else
    "$@"
  fi
}

if [[ ! -x "$repo_root/bin/engramd" || ! -x "$repo_root/bin/engramctl" ]]; then
  echo "bin/engramd and bin/engramctl must exist; run make build first" >&2
  exit 1
fi

run install -d -m 0755 "$PREFIX/bin" "$CONFIG_DIR" "$DATA_DIR" "$SYSTEMD_DIR"
run install -m 0755 "$repo_root/bin/engramd" "$PREFIX/bin/engramd"
run install -m 0755 "$repo_root/bin/engramctl" "$PREFIX/bin/engramctl"

if [[ ! -f "$CONFIG_DIR/engram.yaml" ]]; then
  run install -m 0644 "$repo_root/configs/example.yaml" "$CONFIG_DIR/engram.yaml"
fi

run install -m 0644 "$repo_root/deploy/systemd/engramd.service" "$SYSTEMD_DIR/engramd.service"

echo "ENGRAM installed. Review $CONFIG_DIR/engram.yaml, then run:"
echo "  systemctl daemon-reload"
echo "  systemctl enable --now engramd"
