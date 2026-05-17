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
6. Keep ignored local files such as `.TODO.md` and `bin/` out of commits.
7. If a step fails, stop and report the failed command and recovery steps.

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
  --generate-notes
```

If release notes need manual content, use `--notes-file <file>` instead of `--generate-notes`.

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
