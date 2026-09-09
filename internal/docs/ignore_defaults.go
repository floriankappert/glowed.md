package docs

const (
	IgnoreReasonDefault = "built-in"
	IgnoreReasonProject = "project"
)

var defaultIgnorePatterns = []string{
	".git/",
	// Obsidian keeps its settings in .obsidian/ and its deleted notes in
	// .trash/; plugins and templates there hold .md files that are not the
	// project's documents.
	".obsidian/",
	".trash/",
	".hg/",
	".svn/",
	"node_modules/",
	"vendor/",
	".cache/",
	".turbo/",
	".next/",
	".pytest_cache/",
	".mypy_cache/",
	".ruff_cache/",
	"__pycache__/",
	".venv/",
	"venv/",
	"/bin/",
	"/build/",
	"/dist/",
	"/out/",
	"/target/",
	"/coverage/",
	"/tmp/",
	"*.log",
	"*.tmp",
	"*.bak",
	"*.swp",
	"*.swo",
	"*~",
	".DS_Store",
	".AppleDouble",
	".LSOverride",
	".idea/",
	".vscode/",
}

const defaultGlowedIgnoreTemplate = `# glowed Markdown scan ignore rules
#
# glowed already applies built-in default ignores for common VCS,
# dependency, cache, and generated-output paths. Add project-specific rules
# below. Use !pattern to re-include paths hidden by built-in defaults.
#
# Examples:
# /private-notes/
# *.draft.md
# !/build/
# !vendor/
# !vendor/docs/
`
