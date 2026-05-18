#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/draft-changelog.sh VERSION [options]

Generate an LLM-ready changelog prompt from git log/diff and, when an LLM
command is provided, write a release notes draft to .release/notes-VERSION.md.

Options:
  --llm-cmd CMD          Command that reads the prompt from stdin and writes
                         Markdown release notes to stdout.
  --no-llm               Only write the context and prompt files.
  --target-ref REF       Build notes for changes up to REF instead of HEAD.
                         Useful for backfilling notes for an existing tag.
  --max-diff-bytes N     Max bytes per diff section in the prompt.
                         Default: $GLOWED_CHANGELOG_MAX_DIFF_BYTES or 200000.
  -h, --help             Show this help.

Environment:
  GLOWED_CHANGELOG_LLM             Same as --llm-cmd.
  GLOWED_CHANGELOG_MAX_DIFF_BYTES  Same as --max-diff-bytes.

Examples:
  scripts/draft-changelog.sh v0.4.0 --llm-cmd "codex exec --sandbox read-only -"
  GLOWED_CHANGELOG_LLM="codex exec --sandbox read-only -" scripts/draft-changelog.sh v0.4.0
  scripts/draft-changelog.sh v0.1.0 --target-ref v0.1.0 --no-llm
USAGE
}

version=""
llm_cmd="${GLOWED_CHANGELOG_LLM:-}"
no_llm=0
target_ref="HEAD"
max_diff_bytes="${GLOWED_CHANGELOG_MAX_DIFF_BYTES:-200000}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --llm-cmd)
      if [ "$#" -lt 2 ]; then
        echo "error: --llm-cmd requires a command" >&2
        exit 2
      fi
      llm_cmd="$2"
      shift 2
      ;;
    --no-llm)
      no_llm=1
      shift
      ;;
    --target-ref)
      if [ "$#" -lt 2 ]; then
        echo "error: --target-ref requires a ref" >&2
        exit 2
      fi
      target_ref="$2"
      shift 2
      ;;
    --max-diff-bytes)
      if [ "$#" -lt 2 ]; then
        echo "error: --max-diff-bytes requires a number" >&2
        exit 2
      fi
      max_diff_bytes="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      if [ -n "$version" ]; then
        echo "error: unexpected argument: $1" >&2
        usage >&2
        exit 2
      fi
      version="$1"
      shift
      ;;
  esac
done

if [ -z "$version" ]; then
  echo "error: VERSION is required" >&2
  usage >&2
  exit 2
fi

case "$version" in
  v*) ;;
  *) version="v$version" ;;
esac

case "$max_diff_bytes" in
  ''|*[!0-9]*)
    echo "error: --max-diff-bytes must be a positive integer" >&2
    exit 2
    ;;
esac
if [ "$max_diff_bytes" -le 0 ]; then
  echo "error: --max-diff-bytes must be a positive integer" >&2
  exit 2
fi

root=$(git rev-parse --show-toplevel)
cd "$root"

target_commit=$(git rev-parse --verify "${target_ref}^{commit}" 2>/dev/null) || {
  echo "error: target ref not found: $target_ref" >&2
  exit 1
}

mkdir -p .release
context_file=".release/changelog-context-${version}.md"
prompt_file=".release/changelog-prompt-${version}.md"
notes_file=".release/notes-${version}.md"
llm_log_file=".release/llm-${version}.log"
date_utc=$(date -u +%F)
last_tag=$(git tag --merged "$target_commit" --list 'v*' --sort=-version:refname | grep -v "^${version}$" | head -n 1 || true)

if [ -n "$last_tag" ]; then
  range_label="${last_tag}..${target_ref}"
  log_args=("${last_tag}..${target_commit}")
  diff_args=("${last_tag}..${target_commit}")
else
  range_label="initial release history up to ${target_ref}"
  log_args=("$target_commit")
  empty_tree=$(git hash-object -t tree /dev/null)
  diff_args=("$empty_tree" "$target_commit")
fi

append_limited() {
  local title="$1"
  shift
  local tmp
  tmp=$(mktemp)
  {
    printf '## %s\n\n' "$title"
    printf '```text\n'
  } >> "$context_file"
  if "$@" > "$tmp" 2>&1; then
    :
  else
    printf 'command failed: %s\n' "$*" > "$tmp"
  fi
  local bytes
  bytes=$(wc -c < "$tmp" | tr -d '[:space:]')
  if [ "$bytes" -gt "$max_diff_bytes" ]; then
    head -c "$max_diff_bytes" "$tmp" >> "$context_file"
    printf '\n\n[truncated after %s bytes; original %s bytes]\n' "$max_diff_bytes" "$bytes" >> "$context_file"
  else
    cat "$tmp" >> "$context_file"
  fi
  printf '\n```\n\n' >> "$context_file"
  rm -f "$tmp"
}

cat > "$context_file" <<EOF
# glowed changelog context

Version: ${version}
Date: ${date_utc}
Range: ${range_label}
Target ref: ${target_ref}
Target commit: ${target_commit}
Repository: $(git config --get remote.origin.url || basename "$root")

EOF

append_limited "Target commit" git show --no-patch --oneline "$target_commit"
if [ "$target_commit" = "$(git rev-parse HEAD)" ]; then
  append_limited "Git status" git status --short
fi
append_limited "Commits" git log --pretty=format:'- %s (%h)' "${log_args[@]}"
append_limited "Diff stat" git diff --stat --find-renames "${diff_args[@]}"
append_limited "Committed diff" git diff --find-renames "${diff_args[@]}"
if [ "$target_commit" = "$(git rev-parse HEAD)" ]; then
  append_limited "Staged working tree diff" git diff --cached --find-renames
  append_limited "Unstaged working tree diff" git diff --find-renames
fi

cat > "$prompt_file" <<EOF
You are writing release notes for glowed, a Ghostty-oriented terminal TUI Markdown browser/editor.

Version: ${version}
Date: ${date_utc}

Use the git log and diff context below to write a concise, user-facing changelog draft in Markdown.

Rules:
- Output Markdown only.
- Do not include a top-level title or version heading; the release script will add it.
- Group changes under third-level headings: `### Added`, `### Changed`, `### Fixed`, `### Removed`, `### Breaking Changes`, `### Documentation`, `### Internal`.
- Do not use `#` or `##` headings inside the notes.
- Prefer user-facing behavior over implementation details.
- Mention breaking configuration or behavior changes clearly.
- Include migration notes when a setting or file format changes.
- Do not invent changes that are not present in the context.
- Keep wording suitable for both CHANGELOG.md and GitHub Release notes.

EOF
cat "$context_file" >> "$prompt_file"

if [ "$no_llm" -eq 1 ]; then
  echo "Wrote context: $context_file"
  echo "Wrote prompt:  $prompt_file"
  echo "Skipped LLM because --no-llm was provided."
  exit 0
fi

if [ -z "$llm_cmd" ]; then
  echo "Wrote context: $context_file"
  echo "Wrote prompt:  $prompt_file"
  echo "No LLM command provided; set GLOWED_CHANGELOG_LLM or pass --llm-cmd." >&2
  exit 0
fi

if sh -c "$llm_cmd" < "$prompt_file" > "$notes_file" 2> "$llm_log_file"; then
  if [ ! -s "$notes_file" ]; then
    echo "error: LLM command produced an empty notes file: $notes_file" >&2
    if [ -s "$llm_log_file" ]; then
      echo "LLM log tail:" >&2
      tail -40 "$llm_log_file" >&2
    fi
    exit 1
  fi
else
  status=$?
  echo "error: LLM command failed with status $status: $llm_cmd" >&2
  if [ -s "$llm_log_file" ]; then
    echo "LLM log tail:" >&2
    tail -40 "$llm_log_file" >&2
  fi
  exit "$status"
fi

echo "Wrote context: $context_file"
echo "Wrote prompt:  $prompt_file"
echo "Wrote notes:   $notes_file"
if [ -s "$llm_log_file" ]; then
  echo "Wrote LLM log: $llm_log_file"
fi
echo "Review the notes, then run: scripts/update-changelog.sh $version $notes_file"
