// Package decorations ports bat's decorations module: the gutter elements drawn
// to the left of each line (line numbers, git change markers, grid border).
// Each decoration yields its display Width separately from its (possibly
// colorized) Text so the printer can compute the panel width independent of
// escape sequences.
package decorations

import (
	"fmt"
	"unicode/utf8"
)

// DefaultGutterColor is bat's fallback gutter foreground (xterm 238).
const DefaultGutterColor = "38;5;238"

// Git marker colors (green / red / yellow).
const (
	GitAddedColor    = "32"
	GitRemovedColor  = "31"
	GitModifiedColor = "33"
)

// Paint wraps text in an SGR sequence. An empty sgr returns text unchanged
// (plain, uncolored output).
func Paint(sgr, text string) string {
	if sgr == "" {
		return text
	}
	return "\x1b[" + sgr + "m" + text + "\x1b[0m"
}

// Text is a rendered decoration: its terminal Width and the string to emit.
type Text struct {
	Width int
	Text  string
}

// LineNumber renders a 4-wide right-aligned line number, or blank padding on a
// wrapped continuation row. gutterSGR is the gutter color ("" for plain).
func LineNumber(gutterSGR string, lineNumber int, continuation bool) Text {
	if continuation {
		width := 4
		if lineNumber >= 10000 {
			width = len(fmt.Sprintf("%d", lineNumber))
		}
		return Text{Width: width, Text: Paint(gutterSGR, spaces(width))}
	}
	plain := fmt.Sprintf("%4d", lineNumber)
	return Text{Width: utf8.RuneCountInString(plain), Text: Paint(gutterSGR, plain)}
}

// GridBorder renders the vertical grid separator "│".
func GridBorder(gutterSGR string) Text {
	return Text{Width: 1, Text: Paint(gutterSGR, "│")}
}

// ChangeKind identifies a git line change for the changes gutter.
type ChangeKind int

const (
	ChangeNone ChangeKind = iota
	ChangeAdded
	ChangeRemovedAbove
	ChangeRemovedBelow
	ChangeModified
)

// LineChange renders the single-character git change marker for a line.
func LineChange(kind ChangeKind) Text {
	switch kind {
	case ChangeAdded:
		return Text{Width: 1, Text: Paint(GitAddedColor, "+")}
	case ChangeRemovedAbove:
		return Text{Width: 1, Text: Paint(GitRemovedColor, "‾")}
	case ChangeRemovedBelow:
		return Text{Width: 1, Text: Paint(GitRemovedColor, "_")}
	case ChangeModified:
		return Text{Width: 1, Text: Paint(GitModifiedColor, "~")}
	default:
		return Text{Width: 1, Text: " "}
	}
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
