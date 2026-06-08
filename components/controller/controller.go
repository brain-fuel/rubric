// Package controller ports bat's controller module: it drives the whole
// pipeline for each input — open and buffer it, detect the syntax (honoring
// language overrides, syntax mappings and ignored suffixes), highlight it,
// gather git changes, then walk the lines emitting headers, snip separators,
// highlighted content and footers through the printer. It also implements the
// undecorated "cat" path and binary-content handling.
package controller

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"goforge.dev/gat/components/assets"
	"goforge.dev/gat/components/config"
	"goforge.dev/gat/components/decorations"
	"goforge.dev/gat/components/gitdiff"
	"goforge.dev/gat/components/inputsrc"
	"goforge.dev/gat/components/linerange"
	"goforge.dev/gat/components/printer"
)

// Controller renders a set of inputs according to a Config.
type Controller struct {
	cfg   config.Config
	stdin io.Reader
}

// New returns a Controller. stdin supplies standard input for KindStdin inputs.
func New(cfg config.Config, stdin io.Reader) *Controller {
	return &Controller{cfg: cfg, stdin: stdin}
}

// Run renders all inputs to w. It returns the first error encountered while
// still attempting to render remaining inputs (matching bat, which reports
// per-file errors but continues).
func (c *Controller) Run(w io.Writer, inputs []input.Input) error {
	var firstErr error
	for i, in := range inputs {
		if err := c.runOne(w, in, i > 0); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			fmt.Fprintf(w, "gat: %v\n", err)
		}
	}
	return firstErr
}

func (c *Controller) runOne(w io.Writer, in input.Input, addPadding bool) error {
	opened, err := in.Open(c.stdin)
	if err != nil {
		return err
	}
	defer opened.Close()

	content, err := opened.ReadAll()
	if err != nil {
		return err
	}

	// Cat mode: stream raw bytes, no decorations or highlighting.
	if c.cfg.LoopThrough {
		_, err := w.Write(content)
		return err
	}

	binary := isBinary(content) && !c.cfg.ShowNonprintable && c.cfg.Binary != config.BinaryAsText
	empty := len(content) == 0

	if empty && c.cfg.QuietEmpty {
		return nil
	}

	rawLines, lastLine := splitLines(content)

	// Resolve syntax.
	theme := assets.GetTheme(c.cfg.Theme)
	syntax := c.resolveSyntax(opened, content)

	// Highlight (skip for binary / non-printable / plain output).
	var segLines [][]assets.Segment
	if !binary && !c.cfg.ShowNonprintable {
		if sl, herr := syntax.HighlightLines(string(content), theme); herr == nil {
			segLines = sl
		}
	}

	// Git changes.
	var changeMap map[int]decorations.ChangeKind
	if c.cfg.StyleComponents.Changes() && opened.Kind == input.KindFile {
		changeMap = toChangeKinds(gitdiff.Get(opened.Path))
	}

	p := printer.NewInteractive(c.cfg, theme, changeMap)

	if err := p.PrintHeader(w, opened.Name, opened.Size, addPadding); err != nil {
		return err
	}

	if binary {
		fmt.Fprintf(w, "%s: Binary content from %q will not be printed to the terminal "+
			"(use 'gat -A' to show it).\n", "[gat warning]", opened.Name)
		return nil
	}

	// Determine visible lines.
	visible := c.visibleRanges(changeMap, lastLine)

	lastPrinted := 0
	for idx, raw := range rawLines {
		lineNumber := idx + 1
		if visible.Check(lineNumber, lastLine) != linerange.InRange {
			continue
		}
		// Snip separator across a gap between visible ranges.
		if c.cfg.StyleComponents.Snip() && lastPrinted > 0 && lineNumber > lastPrinted+1 {
			if err := p.PrintSnip(w); err != nil {
				return err
			}
		}
		lastPrinted = lineNumber

		var segs []assets.Segment
		if idx < len(segLines) && segLines[idx] != nil {
			segs = segLines[idx]
		} else {
			segs = []assets.Segment{assets.PlainSegment(raw.text)}
		}

		hl := !c.cfg.HighlightedLines.Ranges.Empty() &&
			c.cfg.HighlightedLines.Ranges.Check(lineNumber, lastLine) == linerange.InRange

		line := printer.Line{
			Number:    lineNumber,
			Raw:       raw.text,
			HadNewln:  raw.newln,
			Segments:  segs,
			Highlight: hl,
		}
		if err := p.PrintLine(w, line); err != nil {
			return err
		}
	}

	return p.PrintFooter(w)
}

// resolveSyntax applies ignored-suffix stripping, syntax mappings and the
// language/fallback overrides to pick a lexer.
func (c *Controller) resolveSyntax(opened *input.Opened, content []byte) assets.Syntax {
	language := c.cfg.Language

	name := opened.Name
	if opened.Kind == input.KindFile {
		name = opened.Path
	}

	// Syntax mapping by glob.
	if language == "" && name != "" {
		if target, ok := c.cfg.SyntaxMapping.Lookup(name); ok {
			switch {
			case target.Unknown:
				return assets.PlainSyntax()
			case target.MapTo != "":
				language = target.MapTo
			}
		}
	}

	detectName := stripIgnoredSuffixes(baseName(name), c.cfg.SyntaxMapping.IgnoredSuffixes())

	firstChunk := content
	if len(firstChunk) > 4096 {
		firstChunk = firstChunk[:4096]
	}

	syntax, err := assets.DetectSyntax(language, c.cfg.FallbackSyntax, detectName, firstChunk)
	if err != nil {
		return assets.PlainSyntax()
	}
	return syntax
}

// visibleRanges yields the line ranges to display, accounting for diff mode.
func (c *Controller) visibleRanges(changes map[int]decorations.ChangeKind, lastLine int) linerange.LineRanges {
	if c.cfg.VisibleLines.DiffModeEnabled() {
		ctx := c.cfg.VisibleLines.DiffContext
		var ranges []linerange.LineRange
		for ln := range changes {
			lo := ln - ctx
			if lo < 1 {
				lo = 1
			}
			ranges = append(ranges, linerange.New(lo, ln+ctx))
		}
		if len(ranges) == 0 {
			return linerange.NoneLineRanges()
		}
		return linerange.NewLineRanges(ranges)
	}
	return c.cfg.VisibleLines.Ranges
}

// rawLine is one input line and whether it ended with a newline.
type rawLine struct {
	text  string
	newln bool
}

func splitLines(content []byte) ([]rawLine, int) {
	if len(content) == 0 {
		return nil, 0
	}
	parts := strings.SplitAfter(string(content), "\n")
	// SplitAfter leaves a trailing "" when content ends in "\n"; drop it.
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	lines := make([]rawLine, 0, len(parts))
	for _, p := range parts {
		newln := strings.HasSuffix(p, "\n")
		text := strings.TrimRight(p, "\n")
		text = strings.TrimRight(text, "\r")
		lines = append(lines, rawLine{text: text, newln: newln})
	}
	return lines, len(lines)
}

func isBinary(content []byte) bool {
	n := len(content)
	if n > 8192 {
		n = 8192
	}
	return bytes.IndexByte(content[:n], 0) >= 0
}

func toChangeKinds(c gitdiff.LineChanges) map[int]decorations.ChangeKind {
	if c == nil {
		return nil
	}
	out := make(map[int]decorations.ChangeKind, len(c))
	for ln, kind := range c {
		switch kind {
		case gitdiff.Added:
			out[ln] = decorations.ChangeAdded
		case gitdiff.RemovedAbove:
			out[ln] = decorations.ChangeRemovedAbove
		case gitdiff.RemovedBelow:
			out[ln] = decorations.ChangeRemovedBelow
		case gitdiff.Modified:
			out[ln] = decorations.ChangeModified
		}
	}
	return out
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func stripIgnoredSuffixes(name string, suffixes []string) string {
	changed := true
	for changed {
		changed = false
		for _, s := range suffixes {
			if strings.HasSuffix(name, s) && len(name) > len(s) {
				name = name[:len(name)-len(s)]
				changed = true
			}
		}
	}
	return name
}
