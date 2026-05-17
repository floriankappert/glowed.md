# glowed

**glowed** 是一个以 Ghostty 为主要目标环境设计的终端 TUI Markdown 浏览器/编辑器。

它会把启动时所在的目录作为 project root，扫描 Markdown 文件，并支持搜索、预览、raw Markdown 编辑、带路径 metadata 的选择复制，以及用当前文档 context 打开外部 LLM CLI 会话。

语言: [English](README.md) · [한국어](README.ko.md) · [日本語](README.jp.md)

## 项目状态

本项目仍处于早期 MVP 阶段。

当前实现是 Go TUI，使用：

- Bubble Tea
- Lipgloss
- Glamour
- Go 标准工具链

glowed 目前实现为一个基于 Go 的终端应用。

当前实现使用 Codex GPT-5.5、local `TODO.md` planning file 和 pi agent coding harness 完成。

## 功能

- 扫描 project root 下的 `.md` 文件
- 支持常见 ignore directory 和基础 `.gitignore` 规则
- 按文件名、frontmatter、`tag:` / `tags:` metadata 搜索
- sidebar 文档列表
- 基于 Glamour 的 Markdown preview
- raw Markdown edit mode
- 带 backup 的 atomic save
- edit mode 中的 undo/redo
- 基于 mouse click、wheel、drag 的 app-managed selection
- 用 source selection mode 精确复制原始 Markdown
- 可点击的 footer action bar
- 可配置 keymap 和 footer actions
- 用于启动任意已配置 CLI command 的 external LLM session launcher

## 安装

### Source build

```bash
git clone https://github.com/khw1031/glowed.git
cd glowed
go build -o ./bin/glowed ./cmd/glowed
./bin/glowed
```

或者安装到 `PATH` 中的某个位置：

```bash
go build -o glowed ./cmd/glowed
install -m 0755 glowed ~/.local/bin/glowed
```

### `go install`

```bash
go install github.com/khw1031/glowed/cmd/glowed@latest
```

### Homebrew tap

发布优先使用 custom Homebrew tap，而不是 Homebrew core。

```bash
brew tap khw1031/tap
brew install glowed
```

或者：

```bash
brew install khw1031/tap/glowed
```

fork 或 custom variant 也可以发布自己的 tap，例如：

```bash
brew install SOMEONE/tap/glowed
```

## 使用方法

把当前目录作为 project root 打开：

```bash
glowed
```

打开指定 project root：

```bash
glowed /path/to/project
```

打开指定 Markdown 文件：

```bash
glowed /path/to/project/notes/file.md
```

帮助：

```bash
glowed --help
```

## 默认按键

- `q`: 退出
- `/`: 聚焦搜索
- `tab`: 循环切换 focus
- `enter`: 打开选中的文档 / 聚焦 preview
- `e`: 编辑当前文档
- `v`: source selection mode
- `c`: 打开 external LLM session
- `ctrl+s`: edit mode 中保存
- `ctrl+z`: edit mode 中 undo
- `ctrl+y`: edit mode 中 redo
- `esc`: 根据 context 取消搜索/编辑/source mode
- `r`: 重新扫描 project root
- `ctrl+g b`: toggle sidebar
- `ctrl+g l`: 打开 external LLM session
- `ctrl+g r`: 重新扫描
- `ctrl+g q`: 退出

## 配置

配置文件按以下顺序加载：

1. `~/.config/glowed/config.json`
2. `<project-root>/.glowed.json`

project-local 配置会覆盖 global 配置。

参考：

- [`glowed.schema.json`](glowed.schema.json)
- [`.glowed.example.json`](.glowed.example.json)

## External LLM session

`glowed` 不会直接处理 OAuth、password 或 API key。

它会启动 `llm.command` 中配置的外部 CLI command，并通过临时 context file 和 clipboard prompt 传递当前 Markdown context。用户需要先安装该 CLI，并在自己的终端环境中完成登录。

`claude` 和 `codex` 只是示例，并不是唯一支持的 command。只要是 `PATH` 中可执行的 interactive CLI，都可以配置为 `llm.command`，包括其他 LLM CLI 或自己的 wrapper script。

预期行为：

- 启动 `llm.command` 中配置的 CLI
- 示例：`claude`, `codex`, `aider`, custom wrapper script
- 对 Claude/Codex 等已知 command，在需要时应用少量 command-specific launch default
- 如果可能，打开 Ghostty split
- 包含当前文件 absolute path、relative path、mode、可选 selection、以及可选 raw Markdown context

## 重要限制

在使用或分发本项目之前，请先阅读本节。

### Ghostty-first，并不是通用终端支持

`glowed` 目前围绕 Ghostty 设计。它可能能在其他终端中运行，但其他终端环境还不是 first-class target。

风险较高的部分：

- mouse tracking
- drag selection
- wheel event
- `Cmd+Left` / `Cmd+Right` 等 platform-specific key sequence
- cursor shape
- alternate screen
- OSC52 clipboard fallback
- external LLM session 的 terminal split 启动

### 多环境测试不足

本项目还没有在大量环境中充分测试。

以下环境目前不保证可用：

- iTerm2
- Terminal.app
- Alacritty
- Kitty
- WezTerm
- VS Code integrated terminal
- SSH session
- tmux/screen
- Linux desktop terminal
- Windows Terminal / WSL

目前最推荐、最有把握的环境是 macOS + Ghostty。

### External LLM launch 依赖环境

external LLM launcher 假设所选 CLI command 已经安装并且已经登录。

Ghostty split support 主要针对 Ghostty。其他终端可能需要不同 command，或者只能打开单独 window。

### Preview selection 不是 source mapping

Preview selection 复制的是 rendered preview text 和 metadata。它不会把终端中渲染后的坐标精确反向映射到 Markdown source range。

如果需要精确复制原始 Markdown，请使用：

- edit/raw mode selection
- `sourceSelect` mode (`v`)

### 早期 editor 注意事项

editor 会执行 backup + atomic save，但它仍然是早期软件。重要文档请务必使用 version control。

## Custom distributions

区分修改版 build 的推荐方式是使用 Homebrew tap namespace。

- 相同的 formula 名称 `glowed` 可以存在于不同 tap 中。
- 例如，`khw1031/tap/glowed` 和 `someone/tap/glowed` 都可以发布。
- 为了避免歧义，用户最好使用 full tap path 安装，例如 `brew install someone/tap/glowed`。
- 鼓励你根据自己的 workflow 自由维护和使用自己的 tap/build。
- 修改版 build 如果用于 drop-in，可以把 binary 安装为 `glowed`；如果需要和其他 build 共存，也可以安装为 `glowed-<name>`。
- 本 repository 不把外部 pull request 作为默认 contribution path。
- 如果你想分享自己的版本，请打开 **Distribution registration** issue 告知我们。maintainer 可能会查看，并在认为合适时自行准备 PR/commit 后合并到本 repository。
- 如果你的 build 是用 AI agent 或 coding harness 修改的，建议说明使用了哪些 agent/model/method。
- 已知 distribution 可能会列在 [`DISTRIBUTIONS.md`](DISTRIBUTIONS.md) 中。

Contribution、distribution registration 和 Homebrew tap 指南见 [`CONTRIBUTING.md`](CONTRIBUTING.md)。

## 开发

运行测试：

```bash
go test ./...
```

运行 benchmark：

```bash
go test -bench=. -benchmem ./internal/docs ./internal/render
```

本地运行：

```bash
go run ./cmd/glowed
```
