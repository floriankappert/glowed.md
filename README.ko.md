# glowed

**glowed**는 Ghostty 중심으로 설계한 터미널 TUI Markdown 브라우저/에디터입니다.

실행한 디렉터리를 project root로 사용하고, Markdown 파일을 스캔한 뒤 검색/미리보기/원본 편집/선택 복사/외부 LLM CLI 세션 실행을 지원합니다.

언어: [English](README.md) · [日本語](README.jp.md) · [中文](README.zh.md)

## 프로젝트 상태

이 프로젝트는 아직 초기 MVP 단계입니다.

현재 구현은 Go TUI이며 다음 도구를 사용합니다.

- Bubble Tea
- Lipgloss
- Glamour
- Go 표준 도구

glowed는 현재 Go 기반 터미널 애플리케이션으로 구현되어 있습니다.

현재 구현은 Codex GPT-5.5, local `TODO.md` planning file, pi agent coding harness를 사용해 작성되었습니다.

## 기능

- project root 아래 `.md` 파일 스캔
- 실행 중 polling으로 Markdown 파일 생성/수정/삭제/rename을 자동 감지해 갱신
- 일반적인 생성 경로는 built-in scan ignore로 제외하고 project-local `.glowedignore`로 override
- 제목, Markdown 본문, 파일명/경로, frontmatter, `tag:` / `tags:` metadata 검색
- 펼침/접힘 가능한 sidebar 디렉터리 트리
- Glamour 기반 Markdown preview
- raw Markdown edit mode
- backup을 동반한 atomic save
- edit mode undo/redo
- mouse click, wheel, drag 기반 app-managed selection
- 원본 Markdown을 정확히 복사하는 source selection mode
- click 가능한 footer action bar
- keymap/footer action 설정
- 설정한 CLI command를 실행하는 external LLM session launcher

## 설치

### Source build

```bash
git clone https://github.com/khw1031/glowed.git
cd glowed
go build -o ./bin/glowed ./cmd/glowed
./bin/glowed
```

또는 `PATH`에 들어있는 위치에 설치합니다.

```bash
go build -o glowed ./cmd/glowed
install -m 0755 glowed ~/.local/bin/glowed
```

### `go install`

```bash
go install github.com/khw1031/glowed/cmd/glowed@latest
```

### Homebrew tap

배포는 Homebrew core가 아니라 custom Homebrew tap을 우선 사용합니다.

```bash
brew tap khw1031/tap
brew install glowed
```

또는:

```bash
brew install khw1031/tap/glowed
```

fork나 custom variant도 각자 tap을 만들 수 있습니다.

```bash
brew install SOMEONE/tap/glowed
```

## 사용법

현재 디렉터리를 project root로 열기:

```bash
glowed
```

특정 project root 열기:

```bash
glowed /path/to/project
```

특정 Markdown 파일 열기:

```bash
glowed /path/to/project/notes/file.md
```

도움말:

```bash
glowed --help
```

project `.glowedignore` template 생성:

```bash
glowed --init-ignore /path/to/project
```

## 검색

`/`를 눌러 검색에 focus합니다. 검색어는 공백 기준으로 token화되며 AND 조건으로 결합됩니다. 예를 들어 `foo bar`는 `foo`와 `bar`가 모두 포함된 문서만 찾습니다.

검색 대상:

- 첫 `# Heading` 또는 frontmatter `title`에서 추출한 문서 제목
- 시작 frontmatter block을 제외한 Markdown 본문
- 상대 경로와 파일명
- raw frontmatter text
- frontmatter `tag` / `tags` field와 inline `tag:foo` marker에서 수집한 tag

일반 검색 결과는 제목, 본문, frontmatter, 경로/파일명, tag 순서로 우선순위를 둡니다. sidebar snippet에는 `title:`, `body:`, `frontmatter:`, `path:`, `tag:foo`처럼 match source가 표시됩니다.

`tag:foo`는 tag 전용 검색입니다. 검색 구문은 `tag:foo`이며, `tags:foo`는 query operator가 아닙니다. 예를 들어 `notes tag:ai draft`는 제목/본문/경로/frontmatter에 `notes`와 `draft`가 포함되고, tag에 `ai`가 포함된 문서를 찾습니다.

## 기본 키

- `q`: 종료
- `/`: 검색 focus
- `tab`: focus 순환; sidebar 디렉터리가 선택된 경우 펼침/접힘
- `enter`: 선택 문서 열기 / preview focus; sidebar 디렉터리가 선택된 경우 펼침/접힘
- `e`: 현재 문서 편집
- `v`: source selection mode
- `c`: external LLM session 열기
- `ctrl+s`: edit mode 저장
- `ctrl+z`: edit mode undo
- `ctrl+y`: edit mode redo
- `esc`: context에 따라 검색/편집/source mode 취소
- `r`: project root 수동 재스캔; 실행 중 Markdown 변경은 polling refresh로도 자동 갱신
- `ctrl+g b`: sidebar toggle
- `ctrl+g l`: external LLM session 열기
- `ctrl+g r`: 재스캔
- `ctrl+g q`: 종료

## 설정

설정 파일은 아래 순서로 로드됩니다.

1. `~/.config/glowed/config.json`
2. `<project-root>/.glowed.json`

project-local 설정이 global 설정을 덮어씁니다.

Markdown 스캔 제외 규칙은 built-in default와 `<project-root>/.glowedignore`의 project-local override를 함께 사용합니다. built-in default는 `.git/`, `node_modules/`, `vendor/`, `.cache/`, `/build/`, `/dist/` 같은 일반적인 VCS, dependency, cache, root 생성 산출물 경로를 숨깁니다. 문법은 gitignore 스타일입니다. 루트 `build`만 제외하려면 `/build/`, 이름이 `build`인 모든 디렉터리를 제외하려면 `build/`를 사용합니다. `.glowedignore` 규칙은 built-in default 뒤에 적용되므로 `!pattern`으로 default ignore를 다시 포함할 수 있습니다. `.gitignore`는 의도적으로 읽지 않습니다.

`glowed --init-ignore [project-root]`로 starter `.glowedignore` template을 생성할 수 있습니다. 기존 파일은 덮어쓰지 않습니다.

실행 중에는 lightweight polling snapshot으로 note 관련 파일 변경을 감지합니다. 기본 polling 간격은 5초이며, Markdown 파일의 path/size/modtime snapshot 또는 `.glowedignore` fingerprint가 바뀌면 project를 재스캔합니다. 즉시 반영이 필요하면 수동 refresh 키(`r`)를 사용할 수 있습니다.

참고:

- [`glowed.schema.json`](glowed.schema.json)
- [`.glowed.example.json`](.glowed.example.json)
- [`.glowedignore`](.glowedignore)

## External LLM session

`glowed`는 OAuth, password, API key를 직접 다루지 않습니다.

대신 `llm.command`에 설정된 외부 CLI command를 실행하고, 현재 Markdown context를 임시 context file과 clipboard prompt로 전달합니다. 사용자는 해당 CLI를 미리 설치하고 터미널 환경에서 로그인해 두어야 합니다.

`claude`와 `codex`는 예시일 뿐 유일한 지원 대상이 아닙니다. `PATH`에서 실행 가능한 interactive CLI라면 다른 LLM CLI나 자체 wrapper script도 `llm.command`로 지정할 수 있습니다.

의도한 동작:

- `llm.command`에 설정된 CLI 실행
- 예시: `claude`, `codex`, `aider`, custom wrapper script
- Claude/Codex처럼 알려진 command에는 필요한 경우 작은 launch default 적용
- 가능하면 Ghostty split 열기
- 현재 파일 absolute path, relative path, mode, 선택 영역, 선택적으로 raw Markdown context 포함

## 중요한 한계점

사용하거나 배포하기 전에 이 내용을 꼭 확인하세요.

### Ghostty 우선, 모든 터미널 대응 아님

`glowed`는 현재 Ghostty를 기준으로 설계되어 있습니다. 다른 터미널에서도 실행될 수는 있지만, 아직 1차 지원 대상은 아닙니다.

위험이 있는 영역:

- mouse tracking
- drag selection
- wheel event
- `Cmd+Left` / `Cmd+Right` 등 platform-specific key sequence
- cursor shape
- alternate screen
- OSC52 clipboard fallback
- external LLM session용 terminal split 실행

### 다양한 환경 테스트 부족

아직 여러 환경에서 충분히 테스트하지 않았습니다.

다음 환경은 보장하지 않습니다.

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

현재 가장 강하게 기대하는 환경은 macOS + Ghostty입니다.

### External LLM launch는 환경 의존적

external LLM launcher는 선택한 CLI command가 이미 설치되어 있고 로그인되어 있다고 가정합니다.

Ghostty split support는 Ghostty에 맞춰져 있습니다. 다른 터미널에서는 다른 command가 필요하거나 별도 window만 열릴 수 있습니다.

### Preview selection은 source mapping이 아님

Preview selection은 rendered preview text와 metadata를 복사합니다. 터미널에 렌더링된 좌표를 정확한 Markdown 원본 range로 역매핑하지 않습니다.

원본 Markdown을 정확히 복사하려면 다음을 사용하세요.

- edit/raw mode selection
- `sourceSelect` mode (`v`)

### 초기 editor 주의

editor는 backup + atomic save를 수행하지만 아직 초기 소프트웨어입니다. 중요한 문서는 반드시 version control을 사용하세요.

## Custom distributions

수정한 build를 구분하는 권장 방식은 Homebrew tap namespace를 사용하는 것입니다.

- 같은 formula 이름 `glowed`는 서로 다른 tap에 동시에 존재할 수 있습니다.
- 예를 들어 `khw1031/tap/glowed`와 `someone/tap/glowed`는 둘 다 배포 가능합니다.
- 모호함을 피하려면 사용자는 `brew install someone/tap/glowed`처럼 full tap path로 설치하는 것이 좋습니다.
- 자신의 workflow에 맞춘 tap/build를 자유롭게 유지하고 사용해도 됩니다.
- 수정한 build는 drop-in 용도라면 binary를 `glowed`로 설치해도 되고, 여러 build와 공존해야 한다면 `glowed-<name>`으로 설치해도 됩니다.
- 이 repository는 외부 pull request를 기본 contribution 경로로 받지 않습니다.
- 자신의 버전을 공유하고 싶다면 **Distribution registration** issue로 알려 주세요. maintainer가 살펴본 뒤 필요하다고 판단하면 직접 PR/commit을 만들어 이 repository에 반영할 수 있습니다.
- AI agent나 coding harness로 build를 수정했다면 어떤 agent/model/method를 사용했는지도 명시하는 것을 권장합니다.
- 알려진 distribution은 [`DISTRIBUTIONS.md`](DISTRIBUTIONS.md)에 정리될 수 있습니다.

Contribution, distribution registration, Homebrew tap 안내는 [`CONTRIBUTING.md`](CONTRIBUTING.md)를 참고하세요.

## 개발

테스트:

```bash
go test ./...
```

벤치마크:

```bash
go test -bench=. -benchmem ./internal/docs ./internal/render
```

로컬 실행:

```bash
go run ./cmd/glowed
```
