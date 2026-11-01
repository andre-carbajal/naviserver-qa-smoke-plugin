#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
version=1.0.0
name="qa-smoke_${version}_linux_amd64"
out="$root/dist"
mkdir -p "$out"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
GOOS=linux GOARCH=amd64 go build -trimpath -o "$tmp/plugin" .
go run . --write-manifest "$tmp/manifest.json"
(cd "$tmp" && zip -q -j "$out/$name.zip" manifest.json plugin)
(cd "$out" && sha256sum "$name.zip" > "$name.zip.sha256")
printf 'Built %s and checksum\n' "$out/$name.zip"
