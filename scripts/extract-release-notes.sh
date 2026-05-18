#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/extract-release-notes.sh VERSION [OUTPUT_FILE]

Extract the VERSION section from CHANGELOG.md. If OUTPUT_FILE is omitted, write
to stdout. The extracted Markdown is suitable for GitHub Release --notes-file.
USAGE
}

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  usage >&2
  exit 2
fi

case "$1" in
  -h|--help)
    usage
    exit 0
    ;;
esac

version="$1"
case "$version" in
  v*) ;;
  *) version="v$version" ;;
esac
output="${2:-}"
root=$(git rev-parse --show-toplevel)
cd "$root"

if [ ! -f CHANGELOG.md ]; then
  echo "error: CHANGELOG.md not found" >&2
  exit 1
fi

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

awk -v version="$version" '
  $0 ~ "^##[[:space:]]+" version "([[:space:]-]|$)" { in_section=1; next }
  /^##[[:space:]]+(Unreleased|v[0-9])/ && in_section { exit }
  in_section { print }
' CHANGELOG.md > "$tmp"

# Trim leading/trailing blank lines.
awk '
  NF { seen=1 }
  seen { lines[++n]=$0 }
  END {
    while (n > 0 && lines[n] ~ /^[[:space:]]*$/) n--
    for (i = 1; i <= n; i++) print lines[i]
  }
' "$tmp" > "$tmp.trimmed"
mv "$tmp.trimmed" "$tmp"

if [ ! -s "$tmp" ]; then
  echo "error: no CHANGELOG.md section found for $version" >&2
  exit 1
fi

if [ -n "$output" ]; then
  cp "$tmp" "$output"
  echo "Wrote release notes for $version to $output"
else
  cat "$tmp"
fi
