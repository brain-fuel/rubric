package printer

import (
	"fmt"
	"regexp"
	"strings"

	"goforge.dev/rubric/components/config"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// replaceNonprintable renders spaces, tabs, newlines and control characters
// using bat's visible notation (the -A/--show-all behavior). It ports bat's
// replace_nonprintable.
func replaceNonprintable(input string, tabWidth int, notation config.NonprintableNotation) string {
	if tabWidth == 0 {
		tabWidth = 4
	}
	var out strings.Builder
	lineIdx := 0
	for _, chr := range input {
		lineIdx++
		switch {
		case chr == ' ':
			out.WriteRune('·')
		case chr == '\t':
			tabStop := tabWidth - ((lineIdx - 1) % tabWidth)
			lineIdx = 0
			if tabStop == 1 {
				out.WriteRune('↹')
			} else {
				out.WriteRune('├')
				out.WriteString(strings.Repeat("─", tabStop-2))
				out.WriteRune('┤')
			}
		case chr == '\n':
			if notation == config.NotationCaret {
				out.WriteString("^J")
			} else {
				out.WriteRune('␊')
			}
			lineIdx = 0
		case chr <= 0x1F:
			if notation == config.NotationCaret {
				out.WriteRune('^')
				out.WriteRune(rune(0x40 + chr))
			} else {
				out.WriteRune(rune(0x2400 + chr))
			}
		case chr == 0x7F:
			if notation == config.NotationCaret {
				out.WriteString("^?")
			} else {
				out.WriteRune('␡')
			}
		case chr < 0x80:
			out.WriteRune(chr)
		default:
			out.WriteRune(chr)
		}
	}
	return out.String()
}

var _ = fmt.Sprintf
