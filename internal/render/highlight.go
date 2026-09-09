package render

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

// Span marks a colored rune range inside a single raw line. Start and End are
// rune indices into that line and End is exclusive.
type Span struct {
	Start int
	End   int
	Color string
}

// CodeSpans returns syntax highlighting spans for the fenced code blocks in a
// raw markdown buffer, keyed by line index. Only fences that name a language
// chroma knows are highlighted; everything else is left untouched so prose
// keeps rendering as plain text.
//
// Colors come from the same glamour style the preview uses, so a code block
// looks the same in the editor and in the preview.
func CodeSpans(lines []string, style string) map[int][]Span {
	out := map[int][]Span{}
	if len(lines) == 0 {
		return out
	}
	palette := chromaPalette(style)
	if palette == nil {
		return out
	}
	for _, block := range fencedBlocks(lines) {
		body := strings.Join(lines[block.First:block.Last+1], "\n")
		for offset, spans := range blockSpans(block.Language, body, style, palette) {
			out[block.First+offset] = spans
		}
	}
	return out
}

// blockSpans tokenizes one code block and returns spans keyed by the line
// offset inside the block. Results are cached so that editing one block does
// not re-tokenize every other block in the buffer on each keystroke.
func blockSpans(language, body, style string, palette map[chroma.TokenType]string) map[int][]Span {
	cacheKey := style + "\x00" + language + "\x00" + body
	if cached, ok := blockCache.Load(cacheKey); ok {
		return cached.(map[int][]Span)
	}

	spans := map[int][]Span{}
	if lexer := lexers.Get(language); lexer != nil {
		if iter, err := lexer.Tokenise(nil, body); err == nil {
			line, col := 0, 0
			for _, token := range iter.Tokens() {
				color := paletteColor(palette, token.Type)
				for i, part := range strings.Split(token.Value, "\n") {
					if i > 0 {
						line++
						col = 0
					}
					width := len([]rune(part))
					if width == 0 {
						continue
					}
					if color != "" && strings.TrimSpace(part) != "" {
						spans[line] = append(spans[line], Span{Start: col, End: col + width, Color: color})
					}
					col += width
				}
			}
		}
	}

	// Typing produces a new body on every keystroke, so cap the cache instead
	// of letting it grow with the edit history.
	if blockCacheLen.Add(1) > maxBlockCacheEntries {
		blockCache.Clear()
		blockCacheLen.Store(1)
	}
	blockCache.Store(cacheKey, spans)
	return spans
}

const maxBlockCacheEntries = 512

var (
	blockCache    sync.Map
	blockCacheLen atomic.Int64
)

type fencedBlock struct {
	Language string
	First    int // first line of the block body
	Last     int // last line of the block body
}

// fencedBlocks finds ``` and ~~~ fences that carry an info string. The fence
// lines themselves are not part of the returned body range.
func fencedBlocks(lines []string) []fencedBlock {
	blocks := []fencedBlock{}
	for i := 0; i < len(lines); i++ {
		marker, info, ok := fenceInfo(lines[i])
		if !ok {
			continue
		}
		body := i + 1
		end := len(lines) // unterminated fences run to the end of the buffer
		for j := body; j < len(lines); j++ {
			if m, _, ok := fenceInfo(lines[j]); ok && m == marker {
				end = j
				break
			}
			if strings.HasPrefix(strings.TrimSpace(lines[j]), marker) {
				end = j
				break
			}
		}
		if info != "" && body <= end-1 {
			blocks = append(blocks, fencedBlock{Language: info, First: body, Last: end - 1})
		}
		i = end
	}
	return blocks
}

// fenceInfo reports whether line opens or closes a fence and returns the
// marker plus the language from its info string.
func fenceInfo(line string) (marker string, language string, ok bool) {
	trimmed := strings.TrimSpace(line)
	for _, m := range []string{"```", "~~~"} {
		if !strings.HasPrefix(trimmed, m) {
			continue
		}
		info := strings.TrimSpace(strings.TrimPrefix(trimmed, m))
		info = strings.TrimLeft(info, "`~")
		if idx := strings.IndexAny(info, " \t{,"); idx >= 0 {
			info = info[:idx]
		}
		return m, strings.ToLower(info), true
	}
	return "", "", false
}

var paletteCache sync.Map

// chromaPalette maps chroma token types to the colors of a glamour style.
func chromaPalette(style string) map[chroma.TokenType]string {
	if style == "" {
		style = "dark"
	}
	if cached, ok := paletteCache.Load(style); ok {
		return cached.(map[chroma.TokenType]string)
	}
	cfg, ok := styles.DefaultStyles[style]
	if !ok || cfg == nil || cfg.CodeBlock.Chroma == nil {
		cfg = &styles.DarkStyleConfig
	}
	c := cfg.CodeBlock.Chroma
	if c == nil {
		return nil
	}
	color := func(p ansi.StylePrimitive) string {
		if p.Color == nil {
			return ""
		}
		return *p.Color
	}
	palette := map[chroma.TokenType]string{
		chroma.Text:                color(c.Text),
		chroma.Error:               color(c.Error),
		chroma.Comment:             color(c.Comment),
		chroma.CommentPreproc:      color(c.CommentPreproc),
		chroma.Keyword:             color(c.Keyword),
		chroma.KeywordReserved:     color(c.KeywordReserved),
		chroma.KeywordNamespace:    color(c.KeywordNamespace),
		chroma.KeywordType:         color(c.KeywordType),
		chroma.Operator:            color(c.Operator),
		chroma.Punctuation:         color(c.Punctuation),
		chroma.Name:                color(c.Name),
		chroma.NameBuiltin:         color(c.NameBuiltin),
		chroma.NameTag:             color(c.NameTag),
		chroma.NameAttribute:       color(c.NameAttribute),
		chroma.NameClass:           color(c.NameClass),
		chroma.NameConstant:        color(c.NameConstant),
		chroma.NameDecorator:       color(c.NameDecorator),
		chroma.NameException:       color(c.NameException),
		chroma.NameFunction:        color(c.NameFunction),
		chroma.NameOther:           color(c.NameOther),
		chroma.Literal:             color(c.Literal),
		chroma.LiteralNumber:       color(c.LiteralNumber),
		chroma.LiteralDate:         color(c.LiteralDate),
		chroma.LiteralString:       color(c.LiteralString),
		chroma.LiteralStringEscape: color(c.LiteralStringEscape),
		chroma.GenericDeleted:      color(c.GenericDeleted),
		chroma.GenericEmph:         color(c.GenericEmph),
		chroma.GenericInserted:     color(c.GenericInserted),
		chroma.GenericStrong:       color(c.GenericStrong),
		chroma.GenericSubheading:   color(c.GenericSubheading),
		chroma.Background:          color(c.Background),
	}
	for k, v := range palette {
		if v == "" {
			delete(palette, k)
		}
	}
	paletteCache.Store(style, palette)
	return palette
}

// paletteColor resolves a token's color, falling back to its sub-category and
// category the way chroma resolves style entries, so specific tokens such as
// KeywordDeclaration inherit the Keyword color.
func paletteColor(palette map[chroma.TokenType]string, t chroma.TokenType) string {
	for _, candidate := range []chroma.TokenType{t, t.SubCategory(), t.Category()} {
		if c, ok := palette[candidate]; ok {
			return c
		}
	}
	return palette[chroma.Text]
}
