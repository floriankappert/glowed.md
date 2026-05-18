# glowed

**glowed** は、Ghostty を主な対象として設計されたターミナル TUI Markdown ブラウザ/エディタです。

起動したディレクトリを project root として扱い、Markdown ファイルをスキャンし、検索、プレビュー、raw Markdown 編集、選択範囲のコピー、現在のドキュメント context を渡した外部 LLM CLI セッションの起動を行えます。

言語: [English](README.md) · [한국어](README.ko.md) · [中文](README.zh.md)

## プロジェクトの状態

このプロジェクトはまだ初期 MVP 段階です。

現在の実装は Go TUI で、以下を使用しています。

- Bubble Tea
- Lipgloss
- Glamour
- Go 標準ツール

glowed は現在、Go ベースのターミナルアプリケーションとして実装されています。

現在の実装は Codex GPT-5.5、local `TODO.md` planning file、pi agent coding harness を使って作成されました。

## 機能

- project root 配下の `.md` ファイルをスキャン
- 実行中に polling で Markdown ファイルの作成/変更/削除/rename を自動検出して更新
- 一般的な生成パスは built-in scan ignore で除外し、project-local `.glowedignore` で override
- タイトル、Markdown 本文、ファイル名/path、frontmatter、`tag:` / `tags:` metadata による検索
- 展開/折りたたみ可能な sidebar ディレクトリツリー
- Glamour ベースの Markdown preview
- raw Markdown edit mode
- backup 付き atomic save
- edit mode での undo/redo
- mouse click、wheel、drag による app-managed selection
- 元の Markdown を正確にコピーする source selection mode
- click 可能な footer action bar
- keymap/footer action の設定
- 設定した CLI command を起動する external LLM session launcher

## インストール

### Source build

```bash
git clone https://github.com/khw1031/glowed.git
cd glowed
go build -o ./bin/glowed ./cmd/glowed
./bin/glowed
```

または `PATH` に入っている場所へインストールします。

```bash
go build -o glowed ./cmd/glowed
install -m 0755 glowed ~/.local/bin/glowed
```

### `go install`

```bash
go install github.com/khw1031/glowed/cmd/glowed@latest
```

### Homebrew tap

配布は Homebrew core ではなく custom Homebrew tap を優先します。

```bash
brew tap khw1031/tap
brew install glowed
```

または:

```bash
brew install khw1031/tap/glowed
```

fork や custom variant も、それぞれの tap を公開できます。

```bash
brew install SOMEONE/tap/glowed
```

## 使い方

現在のディレクトリを project root として開く:

```bash
glowed
```

特定の project root を開く:

```bash
glowed /path/to/project
```

特定の Markdown ファイルを開く:

```bash
glowed /path/to/project/notes/file.md
```

ヘルプ:

```bash
glowed --help
```

project `.glowedignore` template を作成:

```bash
glowed --init-ignore /path/to/project
```

## 検索

`/` を押すと検索に focus します。検索語は空白で token 化され、AND 条件として扱われます。たとえば `foo bar` は `foo` と `bar` の両方を含むドキュメントに一致します。

検索対象:

- 最初の `# Heading`、または frontmatter `title` から取得したドキュメントタイトル
- 先頭の frontmatter block を除いた Markdown 本文
- relative path と filename
- raw frontmatter text
- frontmatter `tag` / `tags` field と inline `tag:foo` marker から収集した tag

通常の検索結果は title、body、frontmatter、path/filename、tag の順に優先されます。sidebar snippet には `title:`, `body:`, `frontmatter:`, `path:`, `tag:foo` のように match source が表示されます。

`tag:foo` は tag 専用検索です。query operator は `tag:foo` であり、`tags:foo` ではありません。たとえば `notes tag:ai draft` は title/body/path/frontmatter に `notes` と `draft` が含まれ、tag に `ai` が含まれるドキュメントを探します。

## デフォルトキー

- `q`: 終了
- `/`: 検索に focus
- `tab`: focus を循環。sidebar のディレクトリ選択中は展開/折りたたみ
- `enter`: 選択中のドキュメントを開く / preview に focus。sidebar のディレクトリ選択中は展開/折りたたみ
- `e`: 現在のドキュメントを編集
- `v`: source selection mode
- `c`: external LLM session を開く
- `ctrl+s`: edit mode で保存
- `ctrl+z`: edit mode で undo
- `ctrl+y`: edit mode で redo
- `esc`: context に応じて検索/編集/source mode をキャンセル
- `r`: project root を手動で再スキャン。実行中の Markdown 変更は polling refresh でも自動更新
- `ctrl+g b`: sidebar toggle
- `ctrl+g l`: external LLM session を開く
- `ctrl+g r`: 再スキャン
- `ctrl+g q`: 終了

## 設定

設定ファイルは以下の順番で読み込まれます。

1. `~/.config/glowed/config.json`
2. `<project-root>/.glowed.json`

project-local の設定が global 設定を上書きします。

Markdown scan の除外ルールは built-in defaults と `<project-root>/.glowedignore` の project-local override を組み合わせて使います。built-in defaults は `.git/`, `node_modules/`, `vendor/`, `.cache/`, `/build/`, `/dist/` など、一般的な VCS、dependency、cache、root generated-output path を隠します。文法は gitignore スタイルです。root の `build` だけを除外するには `/build/`、名前が `build` の全ディレクトリを除外するには `build/` を使います。`.glowedignore` のルールは built-in defaults の後に適用されるため、`!pattern` で default ignore を再度含めることができます。`.gitignore` は意図的に読みません。

`glowed --init-ignore [project-root]` で starter `.glowedignore` template を作成できます。既存ファイルは上書きしません。

実行中は lightweight polling snapshot で note 関連のファイル変更を検出します。デフォルトの polling 間隔は 5 秒です。Markdown ファイルの path/size/modtime snapshot、または `.glowedignore` fingerprint が変わると project を再スキャンします。すぐに反映したい場合は手動 refresh キー (`r`) を使えます。

参照:

- [`glowed.schema.json`](glowed.schema.json)
- [`.glowed.example.json`](.glowed.example.json)
- [`.glowedignore`](.glowedignore)

## External LLM session

`glowed` は OAuth、password、API key を直接扱いません。

代わりに `llm.command` に設定された外部 CLI command を起動し、現在の Markdown context を一時 context file と clipboard prompt で渡します。ユーザーは事前にその CLI をインストールし、ターミナル環境でログインしている必要があります。

`claude` と `codex` は例であり、唯一の supported command ではありません。`PATH` から実行できる interactive CLI であれば、別の LLM CLI や独自の wrapper script も `llm.command` に指定できます。

想定している動作:

- `llm.command` に設定された CLI を起動
- 例: `claude`, `codex`, `aider`, custom wrapper script
- Claude/Codex のような既知の command には、必要に応じて小さな launch default を適用
- 可能であれば Ghostty split を開く
- 現在ファイルの absolute path、relative path、mode、選択範囲、必要に応じて raw Markdown context を含める

## 重要な制限

利用または配布する前に、このセクションを読んでください。

### Ghostty 優先であり、すべてのターミナル対応ではありません

`glowed` は現在 Ghostty を前提に設計されています。他のターミナルでも動く可能性はありますが、まだ first-class target ではありません。

リスクがある領域:

- mouse tracking
- drag selection
- wheel event
- `Cmd+Left` / `Cmd+Right` など platform-specific key sequence
- cursor shape
- alternate screen
- OSC52 clipboard fallback
- external LLM session 用 terminal split 起動

### 多様な環境でのテストは不足しています

まだ幅広い環境では十分にテストしていません。

以下は保証されていません。

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

現時点で最も期待される環境は macOS + Ghostty です。

### External LLM launch は環境に依存します

external LLM launcher は、選択した CLI command がすでにインストールされ、ログイン済みであることを前提にしています。

Ghostty split support は特に Ghostty 向けです。他のターミナルでは別の command が必要だったり、別 window のみになる場合があります。

### Preview selection は source mapping ではありません

Preview selection は rendered preview text と metadata をコピーします。ターミナル上のレンダリング座標を正確な Markdown source range へ逆変換するものではありません。

元の Markdown を正確にコピーするには以下を使ってください。

- edit/raw mode selection
- `sourceSelect` mode (`v`)

### 初期 editor に関する注意

editor は backup + atomic save を行いますが、まだ初期段階のソフトウェアです。重要な文書では必ず version control を使ってください。

## Custom distributions

変更した build を区別する推奨方法は、Homebrew tap namespace を使うことです。

- 同じ formula 名 `glowed` は、異なる tap に同時に存在できます。
- たとえば `khw1031/tap/glowed` と `someone/tap/glowed` はどちらも配布できます。
- あいまいさを避けるため、ユーザーは `brew install someone/tap/glowed` のように full tap path でインストールするのが安全です。
- 自分の workflow に合わせた tap/build を自由に維持し、利用することを歓迎します。
- 変更した build は、drop-in 用途なら binary を `glowed` としてインストールしてもよく、複数 build と共存させたい場合は `glowed-<name>` としてインストールしても構いません。
- この repository では、外部 pull request を標準の contribution path として扱いません。
- 自分の version を共有したい場合は、**Distribution registration** issue で知らせてください。maintainer が確認し、必要だと判断した場合は、独自に PR/commit を作成してこの repository に反映することがあります。
- AI agent や coding harness で build を変更した場合は、使用した agent/model/method も明記することを推奨します。
- 既知の distribution は [`DISTRIBUTIONS.md`](DISTRIBUTIONS.md) にまとめられる場合があります。

Contribution、distribution registration、Homebrew tap の案内は [`CONTRIBUTING.md`](CONTRIBUTING.md) を参照してください。

## 開発

テスト:

```bash
go test ./...
```

ベンチマーク:

```bash
go test -bench=. -benchmem ./internal/docs ./internal/render
```

ローカル実行:

```bash
go run ./cmd/glowed
```
