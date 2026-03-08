#!/usr/bin/env bash
set -euo pipefail

APP="ifmproxy"
OUT="dist"

rm -rf "$OUT"
mkdir -p "$OUT"

platforms=(
  "windows/amd64/.exe"
  "darwin/amd64/"
  "darwin/arm64/"
  "linux/amd64/"
  "linux/arm64/"
)

for entry in "${platforms[@]}"; do
  IFS='/' read -r os arch ext <<< "$entry"
  output="${OUT}/${APP}_${os}_${arch}${ext}"
  echo "Building ${output}..."
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -o "$output" .
done

echo ""
echo "Done. Builds:"
for f in "$OUT"/*; do
  printf "  %-40s %s\n" "$(basename "$f")" "$(du -h "$f" | cut -f1)"
done
