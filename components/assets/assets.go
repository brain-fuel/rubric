// Package assets wraps the chroma syntax-highlighting engine to provide bat's
// asset layer: lexer (syntax) detection by language name, file name and
// content; theme (style) lookup with bat-compatible theme-name aliases; and the
// per-line styled-segment output the printer consumes.
package assets

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Segment is a run of text sharing one style, ready to be colorized.
type Segment struct {
	Text  string
	entry chroma.StyleEntry
}

// SGR returns the ANSI escape prefix for this segment given the color depth.
// It returns an empty string when the segment has no styling. The caller is
// responsible for appending a reset (\x1b[0m).
func (s Segment) SGR(trueColor bool) string {
	var codes []string
	e := s.entry
	if e.Colour.IsSet() {
		codes = append(codes, fgColor(e.Colour, trueColor))
	}
	if e.Background.IsSet() {
		codes = append(codes, bgColor(e.Background, trueColor))
	}
	if e.Bold == chroma.Yes {
		codes = append(codes, "1")
	}
	if e.Italic == chroma.Yes {
		codes = append(codes, "3")
	}
	if e.Underline == chroma.Yes {
		codes = append(codes, "4")
	}
	if len(codes) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

func fgColor(c chroma.Colour, trueColor bool) string {
	if trueColor {
		return fmt.Sprintf("38;2;%d;%d;%d", c.Red(), c.Green(), c.Blue())
	}
	return fmt.Sprintf("38;5;%d", to256(c))
}

func bgColor(c chroma.Colour, trueColor bool) string {
	if trueColor {
		return fmt.Sprintf("48;2;%d;%d;%d", c.Red(), c.Green(), c.Blue())
	}
	return fmt.Sprintf("48;5;%d", to256(c))
}

// to256 quantizes a 24-bit color to the xterm-256 palette (6x6x6 cube + grays).
func to256(c chroma.Colour) int {
	r, g, b := int(c.Red()), int(c.Green()), int(c.Blue())
	if r == g && g == b {
		if r < 8 {
			return 16
		}
		if r > 248 {
			return 231
		}
		return 232 + (r-8)*24/247
	}
	q := func(v int) int { return (v * 5) / 255 }
	return 16 + 36*q(r) + 6*q(g) + q(b)
}

// Syntax is a resolved lexer ready to highlight source.
type Syntax struct {
	lexer chroma.Lexer
	name  string
}

// Name returns the human-readable syntax name (for headers / --language echo).
func (s Syntax) Name() string { return s.name }

// Theme is a resolved chroma style plus background metadata.
type Theme struct {
	style *chroma.Style
	name  string
}

// Name returns the theme's name.
func (t Theme) Name() string { return t.name }

// Background returns the theme's background color (r,g,b) and whether it is set.
func (t Theme) Background() (uint8, uint8, uint8, bool) {
	bg := t.style.Get(chroma.Background).Background
	if !bg.IsSet() {
		return 0, 0, 0, false
	}
	return bg.Red(), bg.Green(), bg.Blue(), true
}

// LineHighlightSGR returns the background SGR body for highlighted lines
// (--highlight-line), derived from the theme, and whether one is available.
func (t Theme) LineHighlightSGR(trueColor bool) (string, bool) {
	if t.style == nil {
		return "", false
	}
	bg := t.style.Get(chroma.LineHighlight).Background
	if !bg.IsSet() {
		bg = t.style.Get(chroma.Background).Background
		if !bg.IsSet() {
			return "", false
		}
		// Lighten the base background slightly for a visible highlight.
		bg = bg.BrightenOrDarken(0.15)
	}
	return bgColor(bg, trueColor), true
}

// themeAliases maps bat's theme names onto the closest bundled chroma style.
var themeAliases = map[string]string{
	"Monokai Extended":        "monokai",
	"Monokai Extended Bright": "monokai",
	"Monokai Extended Light":  "monokailight",
	"Monokai Extended Origin": "monokai",
	"TwoDark":                 "onedark",
	"OneHalfDark":             "onehalfdark",
	"OneHalfLight":            "onehalflight",
	"GitHub":                  "github",
	"Nord":                    "nord",
	"Dracula":                 "dracula",
	"Solarized (dark)":        "solarized-dark",
	"Solarized (light)":       "solarized-light",
	"gruvbox-dark":            "gruvbox",
	"gruvbox-light":           "gruvbox",
	"base16":                  "base16-snazzy",
	"base16-256":              "base16-snazzy",
	"ansi":                    "bw",
	"Coldark-Cold":            "github",
	"Coldark-Dark":            "monokai",
	"Visual Studio Dark+":     "vs",
	"Sublime Snazzy":          "base16-snazzy",
	"DarkNeon":                "monokai",
}

// DefaultThemeName is gat's default theme, matching bat's default appearance.
const DefaultThemeName = "Monokai Extended"

// GetTheme resolves a theme by bat name or chroma style name. Unknown names
// fall back to the default theme.
func GetTheme(name string) Theme {
	if name == "" {
		name = DefaultThemeName
	}
	candidates := []string{name}
	if alias, ok := themeAliases[name]; ok {
		candidates = append(candidates, alias)
	}
	candidates = append(candidates, strings.ToLower(strings.ReplaceAll(name, " ", "")))
	for _, c := range candidates {
		if s := styles.Get(c); s != styles.Fallback {
			return Theme{style: s, name: name}
		}
	}
	return Theme{style: styles.Get("monokai"), name: name}
}

// ListThemes returns the available theme names, sorted.
func ListThemes() []string {
	names := styles.Names()
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}

// LanguageInfo describes one available syntax.
type LanguageInfo struct {
	Name       string
	Extensions []string
}

// ListLanguages returns the available syntaxes sorted by name.
func ListLanguages() []LanguageInfo {
	var out []LanguageInfo
	for _, name := range lexers.Names(false) {
		l := lexers.Get(name)
		if l == nil {
			continue
		}
		cfg := l.Config()
		out = append(out, LanguageInfo{Name: cfg.Name, Extensions: cfg.Filenames})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// DetectSyntax resolves a lexer using bat's precedence:
//  1. an explicit language name (or fallback name),
//  2. the file name / extension,
//  3. the content (shebang / first lines),
//  4. plain text.
//
// language and fallback are names already resolved from syntax mappings by the
// caller. A non-empty language that does not resolve is reported as an error,
// matching bat's "unknown syntax" behavior.
func DetectSyntax(language, fallback, fileName string, content []byte) (Syntax, error) {
	if language != "" {
		if l := lexers.Get(language); l != nil {
			return wrap(l), nil
		}
		return Syntax{}, fmt.Errorf("unknown syntax: '%s'", language)
	}
	if fileName != "" {
		if l := lexers.Match(fileName); l != nil {
			return wrap(l), nil
		}
	}
	if len(content) > 0 {
		if l := lexers.Analyse(string(content)); l != nil {
			return wrap(l), nil
		}
	}
	if fallback != "" {
		if l := lexers.Get(fallback); l != nil {
			return wrap(l), nil
		}
	}
	return wrap(lexers.Fallback), nil
}

func wrap(l chroma.Lexer) Syntax {
	cfg := l.Config()
	name := "Plain Text"
	if cfg != nil && cfg.Name != "" {
		name = cfg.Name
	}
	return Syntax{lexer: chroma.Coalesce(l), name: name}
}

// PlainSyntax returns the plain-text lexer (no highlighting).
func PlainSyntax() Syntax { return wrap(lexers.Fallback) }

// PlainSegment wraps text as an unstyled segment, used when highlighting is
// disabled or when falling back to raw content.
func PlainSegment(text string) Segment { return Segment{Text: text} }

// HighlightLines tokenizes source with the syntax+theme and returns one slice
// of Segments per source line. Trailing newlines are not included in segment
// text. When the theme is the zero value, only plain segments are returned.
func (s Syntax) HighlightLines(source string, theme Theme) ([][]Segment, error) {
	it, err := s.lexer.Tokenise(nil, source)
	if err != nil {
		return nil, err
	}
	tokens := it.Tokens()
	lines := chroma.SplitTokensIntoLines(tokens)
	out := make([][]Segment, 0, len(lines))
	for _, lineToks := range lines {
		var segs []Segment
		for _, tok := range lineToks {
			text := strings.TrimRight(tok.Value, "\n")
			if text == "" && tok.Value == "\n" {
				continue
			}
			var entry chroma.StyleEntry
			if theme.style != nil {
				entry = theme.style.Get(tok.Type)
			}
			segs = append(segs, Segment{Text: text, entry: entry})
		}
		out = append(out, segs)
	}
	return out, nil
}
