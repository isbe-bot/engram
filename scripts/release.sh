#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "$version" ]]; then
  echo "usage: scripts/release.sh <version>" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

export PATH="/home/rico/.local/go/bin:/home/rico/go/bin:$PATH"

test -z "$(gofmt -l .)"
go mod tidy
git diff --exit-code -- go.mod go.sum
git diff --check
go test ./...
go vet ./...
staticcheck ./...
govulncheck ./...
go test -race ./...
make clean build

out="dist/$version"
rm -rf "$out"
mkdir -p "$out"
cp bin/engramd bin/engramctl "$out/"
cp README.md LICENSE DEVELOPMENT_PLAN.md "$out/"
cp -R configs deploy scripts "$out/"
(
  cd "$out"
  sha256sum engramd engramctl > SHA256SUMS
)

echo "release artifacts ready: $out"
