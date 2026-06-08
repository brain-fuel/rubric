// Package config ports bat's config module: the central Config struct plus the
// option enums (paging, wrapping, non-printable notation, binary behavior,
// strip-ansi, visible lines) and the SyntaxMapping used to override language
// detection by glob.
package config

import (
	"path/filepath"
	"strings"

	"goforge.dev/gat/components/linerange"
	"goforge.dev/gat/components/style"
)

// NonprintableNotation selects how non-printable characters are rendered.
type NonprintableNotation int

const (
	NotationCaret NonprintableNotation = iota // ^@, ^I, ...
	NotationUnicode                           // ␀, ␉, ...
)

// BinaryBehavior controls handling of binary content.
type BinaryBehavior int

const (
	BinaryNoPrinting BinaryBehavior = iota // suppress with a message
	BinaryAsText                           // print as-is
)

// WrappingMode controls line wrapping.
type WrappingMode int

const (
	WrapCharacter WrappingMode = iota // wrap at terminal width
	WrapNever                         // truncate / pass through
)

// PagingMode controls the pager.
type PagingMode int

const (
	PagingAuto PagingMode = iota // QuitIfOneScreen when interactive
	PagingAlways
	PagingQuitIfOneScreen
	PagingNever
)

// StripAnsiMode controls stripping of incoming ANSI escapes.
type StripAnsiMode int

const (
	StripAuto StripAnsiMode = iota
	StripAlways
	StripNever
)

// VisibleLines selects which lines are shown: explicit ranges or git diff
// context.
type VisibleLines struct {
	DiffMode    bool
	DiffContext int
	Ranges      linerange.LineRanges
}

// DiffModeEnabled reports whether only diff context lines are shown.
func (v VisibleLines) DiffModeEnabled() bool { return v.DiffMode }

// DefaultVisibleLines shows all lines.
func DefaultVisibleLines() VisibleLines {
	return VisibleLines{Ranges: linerange.AllLineRanges()}
}

// MappingTarget is the result of a syntax-mapping rule.
type MappingTarget struct {
	// MapTo names a syntax explicitly; empty means "MapToUnknown".
	MapTo string
	// Unknown forces auto-detection to be skipped (treated as plain text).
	Unknown bool
	// KeepOriginal maps the extension back onto the file name's real syntax.
	KeepOriginal bool
}

type mappingRule struct {
	glob   string
	target MappingTarget
}

// SyntaxMapping maps file globs to syntaxes (the -m/--map-syntax feature).
type SyntaxMapping struct {
	rules           []mappingRule
	ignoredSuffixes []string
}

// NewSyntaxMapping returns an empty mapping seeded with bat's builtin ignored
// suffixes.
func NewSyntaxMapping() SyntaxMapping {
	return SyntaxMapping{
		ignoredSuffixes: []string{
			".bak", ".new", ".old", ".orig", ".dpkg-dist", ".dpkg-old",
			".dpkg-new", ".dpkg-tmp", ".pacsave", ".pacnew", ".rpmnew",
			".rpmsave", ".rpmorig", ".in",
		},
	}
}

// Insert registers a glob -> target rule. Later rules take precedence.
func (m *SyntaxMapping) Insert(glob string, target MappingTarget) {
	m.rules = append(m.rules, mappingRule{glob: glob, target: target})
}

// AddIgnoredSuffix registers an extension that should be stripped before syntax
// detection (e.g. "foo.txt.bak" -> "foo.txt").
func (m *SyntaxMapping) AddIgnoredSuffix(suffix string) {
	if !strings.HasPrefix(suffix, ".") {
		suffix = "." + suffix
	}
	m.ignoredSuffixes = append(m.ignoredSuffixes, suffix)
}

// IgnoredSuffixes returns the configured ignored suffixes.
func (m SyntaxMapping) IgnoredSuffixes() []string { return m.ignoredSuffixes }

// Lookup returns the matching MappingTarget for a path, if any. The last
// matching rule wins, mirroring bat's override order.
func (m SyntaxMapping) Lookup(path string) (MappingTarget, bool) {
	base := filepath.Base(path)
	var matched MappingTarget
	found := false
	for _, r := range m.rules {
		if globMatch(r.glob, base) || globMatch(r.glob, path) {
			matched = r.target
			found = true
		}
	}
	return matched, found
}

// globMatch is a thin wrapper over filepath.Match that also supports a leading
// "*." style suffix glob against the basename.
func globMatch(pattern, name string) bool {
	if ok, err := filepath.Match(pattern, name); err == nil && ok {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(name, pattern[1:])
	}
	return false
}

// Config is the resolved runtime configuration handed to the controller and
// printer. Field names track bat's Config struct.
type Config struct {
	// Language is the explicitly requested syntax (-l/--language), if any.
	Language string
	// FallbackSyntax is used when auto-detection fails.
	FallbackSyntax string

	ShowNonprintable     bool
	NonprintableNotation NonprintableNotation
	Binary               BinaryBehavior

	TermWidth int
	TabWidth  int

	// LoopThrough means cat mode: stream input with no decorations/highlighting.
	LoopThrough bool

	ColoredOutput bool
	TrueColor     bool

	StyleComponents style.Components
	WrappingMode    WrappingMode
	// Chop truncates long lines to the terminal width (the -S/--chop-long-lines
	// behavior); only meaningful with WrapNever.
	Chop       bool
	PagingMode PagingMode

	VisibleLines VisibleLines

	Theme         string
	SyntaxMapping SyntaxMapping

	Pager string

	UseItalicText    bool
	HighlightedLines linerange.HighlightedLineRanges

	UseCustomAssets  bool
	UseLessOpen      bool
	SetTerminalTitle bool

	// SqueezeLines, when > 0, collapses runs of blank lines to that many.
	SqueezeLines int

	StripAnsi   StripAnsiMode
	QuietEmpty  bool
	Unbuffered  bool
}

// Default returns a zero-value Config with the all-lines visible default and an
// empty syntax mapping.
func Default() Config {
	return Config{
		VisibleLines:     DefaultVisibleLines(),
		SyntaxMapping:    NewSyntaxMapping(),
		HighlightedLines: linerange.DefaultHighlightedLineRanges(),
		TabWidth:         4,
	}
}
