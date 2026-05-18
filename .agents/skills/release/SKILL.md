---
name: release
description: Release glowed to GitHub and the khw1031 Homebrew tap. Use when asked to publish a new version, create a GitHub Release, update the Homebrew formula, or run the /release workflow.
compatibility: Requires git, gh, Go, curl, shasum, and optionally brew. Assumes repository github.com/khw1031/glowed and tap repository github.com/khw1031/homebrew-tap.
---

# glowed release skill

This skill releases `glowed` from the main repository and updates the Homebrew tap.

## Default inputs

- Version argument: optional.
- If the user provides `vX.Y.Z`, use that exact tag.
- If the user provides `X.Y.Z`, normalize it to `vX.Y.Z`.
- If no version is provided, inspect existing tags and propose the next patch version. If no tags exist, use `v0.1.0`.

## Release safety rules

1. Do not release from a dirty working tree unless the user explicitly asked to include and commit the current changes.
2. Never overwrite an existing git tag without explicit user approval.
3. Run `go test ./...` before tagging.
4. Confirm `gh auth status` works before creating GitHub resources.
5. Use the full tap path in user-facing instructions: `brew install khw1031/tap/glowed`.
6. Keep ignored local files such as `.TODO.md`, `.release/`, and `bin/` out of commits.
7. Generate and review LLM-drafted changelog notes before tagging.
8. Treat `CHANGELOG.md` as the public source of truth for release contents; GitHub Release notes should be extracted from it.
9. If a step fails, stop and report the failed command and recovery steps.

## Main repository release checklist

From the repository root:

```bash
git status --short --ignored
go test ./...
git log -1 --oneline
```

Determine the release tag:

```bash
git tag --list 'v*' --sort=-version:refname | head -10
```

Generate LLM-drafted changelog notes before tagging. The LLM command must read the prompt from stdin and write Markdown to stdout. If no LLM command is configured, the script writes `.release/changelog-prompt-vX.Y.Z.md`; ask the user to run their preferred LLM and save the result as `.release/notes-vX.Y.Z.md`.

```bash
# Example with Codex CLI:
scripts/draft-changelog.sh vX.Y.Z --llm-cmd "codex exec --sandbox read-only -"

# Or with an environment-specific command:
GLOWED_CHANGELOG_LLM="YOUR_LLM_COMMAND" scripts/draft-changelog.sh vX.Y.Z
```

Review and edit the generated notes:

```bash
${EDITOR:-vi} .release/notes-vX.Y.Z.md
```

Update and commit `CHANGELOG.md`. The reviewed release contents become public in this file; `.release/` remains a local draft workspace only.

```bash
scripts/update-changelog.sh vX.Y.Z .release/notes-vX.Y.Z.md
git add CHANGELOG.md
git commit -m "Update changelog for vX.Y.Z"
go test ./...
```

Extract GitHub Release notes from the committed changelog section:

```bash
scripts/extract-release-notes.sh vX.Y.Z /tmp/glowed-release-notes-vX.Y.Z.md
```

Create and push the tag:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin main
git push origin vX.Y.Z
```

Create the GitHub Release using notes extracted from the public `CHANGELOG.md` section:

```bash
gh release create vX.Y.Z \
  --repo khw1031/glowed \
  --title "vX.Y.Z" \
  --notes-file /tmp/glowed-release-notes-vX.Y.Z.md
```

Use `--generate-notes` only if the changelog step was intentionally skipped.

## Homebrew tap checklist

The tap repository is `khw1031/homebrew-tap`. Formula path is `Formula/glowed.rb`.

Compute source tarball SHA256 after the tag exists on GitHub:

```bash
VERSION=vX.Y.Z
TMP=$(mktemp)
curl -L "https://github.com/khw1031/glowed/archive/refs/tags/${VERSION}.tar.gz" -o "$TMP"
shasum -a 256 "$TMP"
rm -f "$TMP"
```

Create or update the tap repository:

```bash
# If the repo does not exist yet:
gh repo create khw1031/homebrew-tap --public --clone /tmp/homebrew-tap

# If it already exists:
gh repo clone khw1031/homebrew-tap /tmp/homebrew-tap
```

Formula template:

```ruby
class Glowed < Formula
  desc "Ghostty-oriented terminal TUI Markdown browser/editor"
  homepage "https://github.com/khw1031/glowed"
  url "https://github.com/khw1031/glowed/archive/refs/tags/vX.Y.Z.tar.gz"
  sha256 "SHA256_HERE"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags=-s -w -X main.version=#{version}", "-o", bin/"glowed", "./cmd/glowed"
  end

  test do
    system "#{bin}/glowed", "--version"
  end
end
```

Commit and push the tap update:

```bash
cd /tmp/homebrew-tap
git add Formula/glowed.rb
git commit -m "glowed vX.Y.Z"
git push origin main
```

Optional local formula checks on macOS with Homebrew:

```bash
brew audit --strict --online Formula/glowed.rb
brew install --build-from-source Formula/glowed.rb
brew test glowed
brew uninstall glowed
```

## Final response format

Report:

- GitHub Release URL
- Tag
- Homebrew tap URL
- Install command
- Test result
- Any skipped checks or caveats

Example:

```text
Released vX.Y.Z.

- GitHub: https://github.com/khw1031/glowed/releases/tag/vX.Y.Z
- Homebrew tap: https://github.com/khw1031/homebrew-tap
- Install: brew install khw1031/tap/glowed
- Tests: go test ./... passed
```
