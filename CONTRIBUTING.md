# Contributing to glowed

Thanks for your interest in glowed.

This repository is the maintainer's version of glowed. The preferred model is that users freely maintain and use their own modified taps/builds, then share them through distribution registration if they want others to discover them.

## Contribution model

- External pull requests are not the default contribution path for this repository.
- Bug reports, compatibility notes, release feedback, and distribution registrations are welcome as issues.
- You are encouraged to customize glowed for your own workflow and distribute your own tap/build freely, as allowed by the project license.
- If you publish a modified distribution, please consider opening a **Distribution registration** issue.
- The maintainer may review registered distributions and, if desired, independently prepare and merge related changes into this repository.
- A change being useful in another distribution does not automatically mean it will be merged here.
- Terminal behavior is environment-sensitive, so please include your OS, terminal, shell, and architecture when reporting TUI issues.

## Before sharing a change or distribution

Please run:

```bash
go test ./...
```

If your change affects scanning or rendering performance, also run:

```bash
go test -bench=. -benchmem ./internal/docs ./internal/render
```

If your change affects terminal input/output, please describe the environment where you tested it, for example:

```text
OS: macOS 15
Terminal: Ghostty
Arch: arm64
Shell: zsh
Notes: tested mouse drag, wheel, ctrl+g prefix, edit cursor movement
```

## Custom distribution model

Homebrew tap namespaces are the recommended way to distinguish modified builds.

The same formula name can exist in different taps:

```bash
brew install khw1031/tap/glowed
brew install someone/tap/glowed
brew install teamname/tap/glowed
```

All three can use the formula name `glowed`; the full tap path identifies whose distribution it is.

Recommended naming:

- Homebrew formula: usually `glowed`
- Install command: always document the full tap path, e.g. `brew install someone/tap/glowed`
- Binary name: use `glowed` for drop-in replacement, or `glowed-<name>` if your build should coexist with other builds
- Version tag example: `v0.1.0-yourname.1`

## Registering a public distribution

If you publish a modified tap or build, please consider opening a **Distribution registration** issue.

This is not an approval request. It is a lightweight way to share:

- tap or package name
- repository URL
- install command
- binary name
- main differences from this repository
- AI agent/model/method used for customization, if any
- tested environment
- maintainer/contact

Known distributions may be listed in [`DISTRIBUTIONS.md`](DISTRIBUTIONS.md).

## Publishing a custom Homebrew tap

A custom distribution can use its own tap while keeping the formula name `glowed`.

Example formula:

```ruby
class Glowed < Formula
  desc "Custom glowed build"
  homepage "https://github.com/SOMEONE/glowed"
  url "https://github.com/SOMEONE/glowed/archive/refs/tags/v0.1.0-someone.1.tar.gz"
  sha256 "..."

  depends_on "go" => :build

  def install
    system "go", "build", "-o", bin/"glowed", "./cmd/glowed"
  end
end
```

Users should install it with the full tap path:

```bash
brew install SOMEONE/tap/glowed
```

If you want your build to coexist with another `glowed`, install a different binary name:

```ruby
def install
  system "go", "build", "-o", bin/"glowed-someone", "./cmd/glowed"
end
```

## Security

Please do not open public issues for sensitive security problems. Until a dedicated `SECURITY.md` exists, contact the maintainer privately if possible.

Security-sensitive areas include:

- launching external commands
- opening terminal sessions
- clipboard/context handling
- path traversal or root guard behavior
