# glowed release process

This document defines the release process for `glowed` so releases can be performed consistently from any local machine.

## Principles

- The public source of truth for release contents is `CHANGELOG.md`.
- GitHub Release notes must be extracted from the matching `CHANGELOG.md` version section.
- `.release/` is a local draft workspace and must not be committed.
- Prompt files, context files, LLM logs, and draft notes under `.release/` are intermediate release-preparation artifacts.
- A release performed from another machine should be reproducible from `CHANGELOG.md`, git tags, GitHub Releases, and the Homebrew formula.

## Prerequisites

Required tools:

- `git`
- `go`
- `gh`
- `curl`
- `shasum`
- Optional: `brew`

Check GitHub CLI authentication:

```bash
gh auth status
```

The working tree must not contain tracked changes. Ignored local files such as `.TODO.md`, `.release/`, and `bin/` must stay out of commits.

```bash
git status --short --ignored
```

Run tests before releasing:

```bash
go test ./...
```

## Version selection

Inspect existing release tags:

```bash
git tag --list 'v*' --sort=-version:refname | head -10
```

Version rules:

- If the user provides `vX.Y.Z`, use that exact version.
- If the user provides `X.Y.Z`, normalize it to `vX.Y.Z`.
- If no version is provided, increment the latest tag's patch version.
- Never overwrite an existing tag without explicit approval.

## Generate changelog draft context

Generate prompt/context files without invoking an LLM:

```bash
scripts/draft-changelog.sh vX.Y.Z --no-llm
```

Optionally provide an LLM command. The command must read the prompt from stdin and write Markdown to stdout:

```bash
scripts/draft-changelog.sh vX.Y.Z --llm-cmd "codex exec --sandbox read-only -"
```

Generated `.release/` files are local intermediate artifacts:

```text
.release/changelog-context-vX.Y.Z.md
.release/changelog-prompt-vX.Y.Z.md
.release/notes-vX.Y.Z.md
.release/llm-vX.Y.Z.log
```

## Review notes

Review and edit the generated notes:

```bash
${EDITOR:-vi} .release/notes-vX.Y.Z.md
```

If you do not want to use `.release/`, write the reviewed notes to a temporary path instead:

```bash
${EDITOR:-vi} /tmp/glowed-notes-vX.Y.Z.md
```

## Update CHANGELOG.md

Insert the reviewed notes into `CHANGELOG.md`:

```bash
scripts/update-changelog.sh vX.Y.Z .release/notes-vX.Y.Z.md
```

Or, without using `.release/`:

```bash
scripts/update-changelog.sh vX.Y.Z /tmp/glowed-notes-vX.Y.Z.md
```

Commit the changelog update and rerun tests:

```bash
git add CHANGELOG.md
git commit -m "Update changelog for vX.Y.Z"
go test ./...
```

## Extract GitHub Release notes

GitHub Release notes are extracted from `CHANGELOG.md`. This step does not require `.release/`.

```bash
scripts/extract-release-notes.sh vX.Y.Z /tmp/glowed-release-notes-vX.Y.Z.md
```

To verify that an already-published release can be reproduced without `.release/`, compare notes extracted from `CHANGELOG.md` with the GitHub Release body:

```bash
scripts/extract-release-notes.sh vX.Y.Z /tmp/glowed-release-notes-from-changelog.md
gh release view vX.Y.Z --repo khw1031/glowed --json body --jq .body > /tmp/glowed-release-notes-from-github.md
diff -u /tmp/glowed-release-notes-from-changelog.md /tmp/glowed-release-notes-from-github.md
```

No `diff` output means the GitHub Release notes can be regenerated from `CHANGELOG.md` without `.release/`.

## Create tag and GitHub Release

Confirm that the tag does not already exist:

```bash
git rev-parse -q --verify refs/tags/vX.Y.Z
```

Create and push the tag:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin main
git push origin vX.Y.Z
```

Create the GitHub Release:

```bash
gh release create vX.Y.Z \
  --repo khw1031/glowed \
  --title "vX.Y.Z" \
  --notes-file /tmp/glowed-release-notes-vX.Y.Z.md
```

## Update the Homebrew tap

Compute the tag tarball SHA256:

```bash
VERSION=vX.Y.Z
TMP=$(mktemp)
curl -L "https://github.com/khw1031/glowed/archive/refs/tags/${VERSION}.tar.gz" -o "$TMP"
shasum -a 256 "$TMP"
rm -f "$TMP"
```

Prepare the `khw1031/homebrew-tap` repository:

```bash
gh repo clone khw1031/homebrew-tap /tmp/homebrew-tap
```

Update `/tmp/homebrew-tap/Formula/glowed.rb` with the new `url` and `sha256`:

```ruby
url "https://github.com/khw1031/glowed/archive/refs/tags/vX.Y.Z.tar.gz"
sha256 "SHA256_HERE"
```

Commit and push the formula update:

```bash
cd /tmp/homebrew-tap
git add Formula/glowed.rb
git commit -m "glowed vX.Y.Z"
git push origin main
```

Optionally run local Homebrew checks:

```bash
brew audit --strict --online Formula/glowed.rb
brew install --build-from-source Formula/glowed.rb
brew test glowed
brew uninstall glowed
```

## Final report format

After releasing, report:

```text
Released vX.Y.Z.

- GitHub: https://github.com/khw1031/glowed/releases/tag/vX.Y.Z
- Tag: vX.Y.Z
- Homebrew tap: https://github.com/khw1031/homebrew-tap
- Install: brew install khw1031/tap/glowed
- Tests: go test ./... passed
- Caveats: skipped checks or environment notes
```
