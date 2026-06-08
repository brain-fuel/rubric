// Package printer ports bat's printer module. It renders buffered input lines
// with bat's decorations (header, grid, line numbers, git change markers,
// snip separators), syntax-highlighted regions, tab expansion, non-printable
// notation, line wrapping and line highlighting. SimplePrinter implements the
// undecorated "cat" path.
package printer

import (
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-runewidth"

	"goforge.dev/gat/components/assets"
	"goforge.dev/gat/components/config"
	"goforge.dev/gat/components/decorations"
)

// Line is a single buffered input line with its precomputed highlight segments.
type Line struct {
	Number    int
	Raw       string // line content without trailing newline
	HadNewln  bool   // whether the original line ended with '\n'
	Segments  []assets.Segment
	Highlight bool // draw the line-highlight background
}

// Printer renders headers, footers, snip markers and lines.
type Printer interface {
	PrintHeader(w io.Writer, name string, size int64, addPadding bool) error
	PrintFooter(w io.Writer) error
	PrintSnip(w io.Writer) error
	PrintLine(w io.Writer, line Line) error
}

// SimplePrinter is the "cat" path: it writes raw line bytes unchanged.
type SimplePrinter struct{}

// NewSimple returns a SimplePrinter.
func NewSimple() *SimplePrinter { return &SimplePrinter{} }

func (p *SimplePrinter) PrintHeader(io.Writer, string, int64, bool) error { return nil }
func (p *SimplePrinter) PrintFooter(io.Writer) error                      { return nil }
func (p *SimplePrinter) PrintSnip(io.Writer) error                        { return nil }

// PrintLine writes the raw line followed by a newline if the original had one.
func (p *SimplePrinter) PrintLine(w io.Writer, line Line) error {
	if _, err := io.WriteString(w, line.Raw); err != nil {
		return err
	}
	if line.HadNewln {
		_, err := io.WriteString(w, "\n")
		return err
	}
	return nil
}

// InteractivePrinter is the decorated, highlighted printer.
type InteractivePrinter struct {
	cfg        config.Config
	theme      assets.Theme
	colored    bool
	gutterSGR  string
	highlitSGR string

	showNumbers bool
	showChanges bool
	showGrid    bool

	panelWidth int

	changes map[int]decorations.ChangeKind
}

// NewInteractive builds an InteractivePrinter. changes maps line numbers to git
// change kinds (may be nil). theme is the resolved highlighting theme.
func NewInteractive(cfg config.Config, theme assets.Theme, changes map[int]decorations.ChangeKind) *InteractivePrinter {
	sc := cfg.StyleComponents
	colored := cfg.ColoredOutput

	gutter := decorations.DefaultGutterColor
	if !colored {
		gutter = ""
	}

	highlit := ""
	if colored {
		if sgr, ok := theme.LineHighlightSGR(cfg.TrueColor); ok {
			highlit = sgr
		}
	}

	p := &InteractivePrinter{
		cfg:         cfg,
		theme:       theme,
		colored:     colored,
		gutterSGR:   gutter,
		highlitSGR:  highlit,
		showNumbers: sc.Numbers(),
		showChanges: sc.Changes() && len(changes) > 0,
		showGrid:    sc.Grid(),
		changes:     changes,
	}

	// Panel width = sum of decoration widths (numbers + change marker), each
	// followed by a separating space. The grid border is accounted separately.
	width := 0
	count := 0
	if p.showNumbers {
		width += 4
		count++
	}
	if p.showChanges {
		width += 1
		count++
	}
	p.panelWidth = width + count // each decoration prints a trailing space

	// Disable the panel entirely on a too-small terminal (need >=5 cols spare).
	if count > 0 && cfg.TermWidth < (width+count)+5 {
		p.panelWidth = 0
		p.showNumbers = false
		p.showChanges = false
	}
	if count == 0 {
		p.panelWidth = 0
	}
	return p
}

func paint(sgr, text string) string { return decorations.Paint(sgr, text) }

// printableWidth is the panel width including the grid border + its space.
func (p *InteractivePrinter) gridPanelWidth() int {
	if p.showGrid && p.panelWidth > 0 {
		return p.panelWidth + 2
	}
	return p.panelWidth
}

func (p *InteractivePrinter) horizontalLine(w io.Writer, gridChar string) error {
	var line string
	if p.panelWidth == 0 {
		line = strings.Repeat("─", p.cfg.TermWidth)
	} else {
		left := strings.Repeat("─", p.panelWidth)
		right := strings.Repeat("─", p.cfg.TermWidth-(p.panelWidth+1))
		line = left + gridChar + right
	}
	_, err := fmt.Fprintln(w, paint(p.gutterSGR, line))
	return err
}

func (p *InteractivePrinter) headerIndent(w io.Writer) error {
	if p.showGrid {
		sep := ""
		if p.panelWidth > 0 {
			sep = "│ "
		}
		_, err := fmt.Fprint(w, strings.Repeat(" ", p.panelWidth)+paint(p.gutterSGR, sep))
		return err
	}
	_, err := fmt.Fprint(w, strings.Repeat(" ", p.panelWidth))
	return err
}

// PrintHeader prints the rule/grid and the filename/size header.
func (p *InteractivePrinter) PrintHeader(w io.Writer, name string, size int64, addPadding bool) error {
	sc := p.cfg.StyleComponents
	if addPadding && sc.Rule() {
		if _, err := fmt.Fprintln(w, paint(p.gutterSGR, strings.Repeat("─", p.cfg.TermWidth))); err != nil {
			return err
		}
	}

	if !sc.Header() {
		if sc.Grid() {
			return p.horizontalLine(w, "┬")
		}
		if addPadding && !sc.Rule() {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		return nil
	}

	if sc.Grid() {
		if err := p.horizontalLine(w, "┬"); err != nil {
			return err
		}
	} else if addPadding && !sc.Rule() {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if sc.HeaderFilename() {
		val := name
		if p.colored {
			val = paint("1", name) // bold
		}
		if err := p.headerMultiline(w, "File: "+val); err != nil {
			return err
		}
	}
	if sc.HeaderFilesize() {
		s := "-"
		if size >= 0 {
			s = humanSize(size)
		}
		val := s
		if p.colored {
			val = paint("1", s)
		}
		if err := p.headerMultiline(w, "Size: "+val); err != nil {
			return err
		}
	}

	if sc.Grid() {
		return p.horizontalLine(w, "┼")
	}
	return nil
}

func (p *InteractivePrinter) headerMultiline(w io.Writer, content string) error {
	if err := p.headerIndent(w); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, content)
	return err
}

// PrintFooter prints the closing grid border.
func (p *InteractivePrinter) PrintFooter(w io.Writer) error {
	if p.cfg.StyleComponents.Grid() {
		return p.horizontalLine(w, "┴")
	}
	return nil
}

// PrintSnip prints the "8<" separator drawn between visible line ranges.
func (p *InteractivePrinter) PrintSnip(w io.Writer) error {
	panel := p.fakePanel(" ...")
	panelCount := runewidth.StringWidth(stripANSI(panel))
	title := "8<"
	titleCount := 2

	leftReps := (p.cfg.TermWidth - panelCount - (titleCount / 2)) / 4
	if leftReps < 0 {
		leftReps = 0
	}
	snipLeft := strings.Repeat("─ ", leftReps)
	snipLeftCount := runewidth.StringWidth(snipLeft)

	rightReps := (p.cfg.TermWidth - panelCount - snipLeftCount - titleCount) / 2
	if rightReps < 0 {
		rightReps = 0
	}
	snipRight := strings.Repeat(" ─", rightReps)

	_, err := fmt.Fprintln(w, paint(p.gutterSGR, panel+snipLeft+title+snipRight))
	return err
}

func (p *InteractivePrinter) fakePanel(text string) string {
	if p.panelWidth == 0 {
		return ""
	}
	truncated := runewidth.Truncate(text, p.panelWidth-1, "")
	filled := truncated + strings.Repeat(" ", p.panelWidth-1-runewidth.StringWidth(truncated))
	if p.showGrid {
		return filled + " │ "
	}
	return filled
}

// PrintLine prints one decorated, highlighted line.
func (p *InteractivePrinter) PrintLine(w io.Writer, line Line) error {
	var sb strings.Builder

	cursorMax := p.cfg.TermWidth

	// Decorations (gutter).
	if p.panelWidth > 0 {
		if p.showNumbers {
			d := decorations.LineNumber(p.gutterSGR, line.Number, false)
			sb.WriteString(d.Text)
			sb.WriteByte(' ')
			cursorMax -= d.Width + 1
		}
		if p.showChanges {
			d := decorations.LineChange(p.changes[line.Number])
			sb.WriteString(d.Text)
			sb.WriteByte(' ')
			cursorMax -= d.Width + 1
		}
		if p.showGrid {
			d := decorations.GridBorder(p.gutterSGR)
			sb.WriteString(d.Text)
			sb.WriteByte(' ')
			cursorMax -= d.Width + 1
		}
	}

	bg := ""
	if line.Highlight {
		bg = p.highlitSGR
	}

	// Preprocess segments (tab expansion / non-printable) into width-tracked
	// styled cells.
	cells := p.buildCells(line)

	if p.cfg.WrappingMode == config.WrapNever {
		p.emitNoWrap(&sb, cells, cursorMax, bg)
	} else {
		p.emitWrap(&sb, cells, cursorMax, bg, line.Number)
	}

	sb.WriteByte('\n')
	_, err := io.WriteString(w, sb.String())
	return err
}

// cell is a single display character carrying its style.
type cell struct {
	r     rune
	width int
	sgr   string
}

func (p *InteractivePrinter) buildCells(line Line) []cell {
	var cells []cell
	cursor := 0
	tabW := p.cfg.TabWidth

	emit := func(text, sgr string) {
		for _, r := range text {
			if r == '\t' && tabW > 0 && !p.cfg.ShowNonprintable {
				spaces := tabW - (cursor % tabW)
				for i := 0; i < spaces; i++ {
					cells = append(cells, cell{r: ' ', width: 1, sgr: sgr})
				}
				cursor += spaces
				continue
			}
			cw := runewidth.RuneWidth(r)
			if cw == 0 {
				cw = 1
			}
			cells = append(cells, cell{r: r, width: cw, sgr: sgr})
			cursor += cw
		}
	}

	if p.cfg.ShowNonprintable {
		text := line.Raw
		if line.HadNewln {
			text += "\n" // rendered as the visible ␊ / ^J marker
		}
		emit(replaceNonprintable(text, tabW, p.cfg.NonprintableNotation), "")
		return cells
	}

	for _, seg := range line.Segments {
		sgr := ""
		if p.colored {
			sgr = seg.SGR(p.cfg.TrueColor)
		}
		emit(seg.Text, sgr)
	}
	return cells
}

// writeCells renders cells with proper SGR transitions and the background fill.
func writeCells(sb *strings.Builder, cells []cell, bg string) {
	cur := ""
	bgOpen := false
	if bg != "" {
		sb.WriteString("\x1b[" + bg + "m")
		bgOpen = true
	}
	for _, c := range cells {
		if c.sgr != cur {
			sb.WriteString("\x1b[0m")
			if bg != "" {
				sb.WriteString("\x1b[" + bg + "m")
			}
			if c.sgr != "" {
				sb.WriteString(c.sgr)
			}
			cur = c.sgr
		}
		sb.WriteRune(c.r)
	}
	if cur != "" || bgOpen {
		sb.WriteString("\x1b[0m")
	}
}

// fillBG appends n background-colored spaces when a highlight background is set.
func fillBG(sb *strings.Builder, bg string, n int) {
	if bg == "" || n <= 0 {
		return
	}
	sb.WriteString("\x1b[" + bg + "m" + strings.Repeat(" ", n) + "\x1b[0m")
}

// emitNoWrap prints cells on a single line. With Chop set, content beyond
// cursorMax is truncated; otherwise the full line is printed.
func (p *InteractivePrinter) emitNoWrap(sb *strings.Builder, cells []cell, cursorMax int, bg string) {
	used := 0
	if p.cfg.Chop {
		kept := cells[:0:0]
		kept = make([]cell, 0, len(cells))
		for _, c := range cells {
			if used+c.width > cursorMax {
				break
			}
			kept = append(kept, c)
			used += c.width
		}
		cells = kept
	} else {
		for _, c := range cells {
			used += c.width
		}
	}
	writeCells(sb, cells, bg)
	if bg != "" && used < cursorMax {
		fillBG(sb, bg, cursorMax-used)
	}
}

// emitWrap performs character or word wrapping at cursorMax, drawing the
// continuation gutter on each wrapped row.
func (p *InteractivePrinter) emitWrap(sb *strings.Builder, cells []cell, cursorMax int, bg string, lineNumber int) {
	gutter := p.continuationGutter(lineNumber)
	wordWrap := p.cfg.WrappingMode == config.WrapWord
	width := 0
	lastWS := -1 // index in buf of the most recent whitespace cell
	var buf []cell
	flushTo := func(emitEnd int, last bool, carry []cell) {
		writeCells(sb, buf[:emitEnd], bg)
		if bg != "" {
			emitted := 0
			for _, c := range buf[:emitEnd] {
				emitted += c.width
			}
			fillBG(sb, bg, cursorMax-emitted)
		}
		if !last {
			sb.WriteString("\n")
			sb.WriteString(gutter)
		}
		buf = append(buf[:0], carry...)
		width = 0
		for _, c := range buf {
			width += c.width
		}
		lastWS = -1
	}
	for _, c := range cells {
		if len(buf) > 0 && width+c.width > cursorMax {
			if wordWrap && lastWS >= 0 && lastWS < len(buf)-1 {
				carry := append([]cell(nil), buf[lastWS+1:]...)
				flushTo(lastWS, false, carry)
			} else {
				flushTo(len(buf), false, nil)
			}
		}
		buf = append(buf, c)
		if wordWrap && c.r == ' ' {
			lastWS = len(buf) - 1
		}
		width += c.width
	}
	flushTo(len(buf), true, nil)
}

// continuationGutter is the blank gutter drawn to the left of wrapped rows.
func (p *InteractivePrinter) continuationGutter(lineNumber int) string {
	if p.panelWidth == 0 {
		return ""
	}
	var sb strings.Builder
	if p.showNumbers {
		d := decorations.LineNumber(p.gutterSGR, lineNumber, true)
		sb.WriteString(d.Text)
		sb.WriteByte(' ')
	}
	if p.showChanges {
		d := decorations.LineChange(decorations.ChangeNone)
		sb.WriteString(d.Text)
		sb.WriteByte(' ')
	}
	if p.showGrid {
		d := decorations.GridBorder(p.gutterSGR)
		sb.WriteString(d.Text)
		sb.WriteByte(' ')
	}
	return sb.String()
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}
