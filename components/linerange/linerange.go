// Package linerange ports bat's line_range module: parsing of "N", "N:M",
// "N:", ":M", "N:+K", "N:-K", "N::C", "N:M:C" and end-relative ":-K"/"K:"
// offsets, plus the LineRanges / HighlightedLineRanges containers used to
// decide which lines are visible or highlighted.
package linerange

import (
	"fmt"
	"strconv"
	"strings"
)

// RangeBound is one side of a LineRange. It is either an absolute line number
// or an (implicitly negative) offset from the end of the file.
type RangeBound struct {
	Offset bool // true => OffsetFromEnd
	Val    int
}

func abs(v int) RangeBound     { return RangeBound{Offset: false, Val: v} }
func fromEnd(v int) RangeBound { return RangeBound{Offset: true, Val: v} }

// LineRange is an inclusive range of line numbers.
type LineRange struct {
	Lower RangeBound
	Upper RangeBound
}

const (
	minLine = 0
	maxLine = int(^uint(0) >> 1) // platform max int, stands in for usize::MAX
)

// DefaultLineRange matches every line.
func DefaultLineRange() LineRange {
	return LineRange{Lower: abs(minLine), Upper: abs(maxLine)}
}

// New returns an absolute [from, to] range.
func New(from, to int) LineRange {
	return LineRange{Lower: abs(from), Upper: abs(to)}
}

// saturating add/sub mirror Rust's usize::saturating_*.
func satAdd(a, b int) int {
	if a > maxLine-b {
		return maxLine
	}
	return a + b
}
func satSub(a, b int) int {
	if a < b {
		return 0
	}
	return a - b
}

func parseUint(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v > uint64(maxLine) {
		return maxLine, nil
	}
	return int(v), nil
}

// Parse parses a single line-range expression with bat's exact semantics.
func Parse(raw string) (LineRange, error) {
	r := DefaultLineRange()
	if raw == "" {
		return r, fmt.Errorf("Empty line range")
	}
	first := raw[0]
	last := raw[len(raw)-1]

	// ":M" or ":-K"  -> only upper bound
	if first == ':' {
		rest := raw[1:]
		if strings.HasPrefix(rest, "-") {
			v, err := parseUint(rest[1:])
			if err != nil {
				return r, err
			}
			r.Upper = fromEnd(v)
		} else {
			v, err := parseUint(rest)
			if err != nil {
				return r, err
			}
			r.Upper = abs(v)
		}
		return r, nil
	}

	// "N:" or "-K:" -> only lower bound
	if last == ':' {
		body := raw[:len(raw)-1]
		if first == '-' {
			v, err := parseUint(body[1:])
			if err != nil {
				return r, err
			}
			r.Lower = fromEnd(v)
		} else {
			v, err := parseUint(body)
			if err != nil {
				return r, err
			}
			r.Lower = abs(v)
		}
		return r, nil
	}

	parts := strings.Split(raw, ":")
	switch len(parts) {
	case 1:
		v, err := parseUint(parts[0])
		if err != nil {
			return r, err
		}
		r.Lower = abs(v)
		r.Upper = abs(v)
		return r, nil
	case 2:
		lower, err := parseUint(parts[0])
		if err != nil {
			return r, err
		}
		second := parts[1]
		var upper int
		switch {
		case strings.HasPrefix(second, "+"):
			more, err := parseUint(second[1:])
			if err != nil {
				return r, fmt.Errorf("Invalid character after +")
			}
			upper = satAdd(lower, more)
		case strings.HasPrefix(second, "-"):
			if strings.HasPrefix(second[1:], "+") {
				return r, fmt.Errorf("Invalid character after -")
			}
			prior, err := parseUint(second[1:])
			if err != nil {
				return r, fmt.Errorf("Invalid character after -")
			}
			upper = lower
			lower = satSub(lower, prior)
		default:
			u, err := parseUint(second)
			if err != nil {
				return r, err
			}
			upper = u
		}
		r.Lower = abs(lower)
		r.Upper = abs(upper)
		return r, nil
	case 3:
		// N::C (context around single line) or N:M:C (context around range)
		if parts[1] == "" {
			line, err := parseUint(parts[0])
			if err != nil {
				return r, fmt.Errorf("Invalid line number in N::C format")
			}
			ctx, err := parseUint(parts[2])
			if err != nil {
				return r, fmt.Errorf("Invalid context number in N::C format")
			}
			r.Lower = abs(satSub(line, ctx))
			r.Upper = abs(satAdd(line, ctx))
		} else {
			start, err := parseUint(parts[0])
			if err != nil {
				return r, fmt.Errorf("Invalid start line number in N:M:C format")
			}
			end, err := parseUint(parts[1])
			if err != nil {
				return r, fmt.Errorf("Invalid end line number in N:M:C format")
			}
			ctx, err := parseUint(parts[2])
			if err != nil {
				return r, fmt.Errorf("Invalid context number in N:M:C format")
			}
			r.Lower = abs(satSub(start, ctx))
			r.Upper = abs(satAdd(end, ctx))
		}
		return r, nil
	default:
		return r, fmt.Errorf("Line range contained too many ':' characters. Expected format: 'N', 'N:M', 'N::C', or 'N:M:C'")
	}
}

// IsInside reports whether line falls within this range. lastLine is the final
// line number of the buffered input, used to resolve end-relative offsets.
func (lr LineRange) IsInside(line, lastLine int) bool {
	lower := lr.Lower.Val
	if lr.Lower.Offset {
		lower = satSub(lastLine, lr.Lower.Val)
	}
	upper := lr.Upper.Val
	if lr.Upper.Offset {
		upper = satSub(lastLine, lr.Upper.Val)
	}
	return lower <= line && line <= upper
}

// RangeCheckResult mirrors bat's tri-state range check, used by the printer to
// decide where to draw "snip" separators.
type RangeCheckResult int

const (
	InRange RangeCheckResult = iota
	BeforeOrBetweenRanges
	AfterLastRange
)

// LineRanges is an ordered set of LineRange.
type LineRanges struct {
	ranges                    []LineRange
	largestAbsoluteUpperBound int
	smallestOffsetFromEnd     int
	largestOffsetFromEndValue int
}

// NewLineRanges builds the aggregate bounds bat precomputes.
func NewLineRanges(ranges []LineRange) LineRanges {
	largestAbs := maxLine
	foundAbs := false
	for _, r := range ranges {
		if !r.Upper.Offset {
			if !foundAbs || r.Upper.Val > largestAbs {
				largestAbs = r.Upper.Val
				foundAbs = true
			}
		}
	}
	if !foundAbs {
		largestAbs = maxLine
	}

	smallest, largest := 0, 0
	foundOff := false
	for _, r := range ranges {
		for _, b := range []RangeBound{r.Lower, r.Upper} {
			if b.Offset {
				if !foundOff {
					smallest, largest = b.Val, b.Val
					foundOff = true
				} else {
					if b.Val < smallest {
						smallest = b.Val
					}
					if b.Val > largest {
						largest = b.Val
					}
				}
			}
		}
	}

	return LineRanges{
		ranges:                    ranges,
		largestAbsoluteUpperBound: largestAbs,
		smallestOffsetFromEnd:     smallest,
		largestOffsetFromEndValue: largest,
	}
}

// AllLineRanges matches every line.
func AllLineRanges() LineRanges { return NewLineRanges([]LineRange{DefaultLineRange()}) }

// NoneLineRanges matches no line.
func NoneLineRanges() LineRanges { return NewLineRanges(nil) }

// Empty reports whether there are no ranges at all.
func (lrs LineRanges) Empty() bool { return len(lrs.ranges) == 0 }

// Check returns the tri-state membership of line, with lastLine being the final
// buffered line number.
func (lrs LineRanges) Check(line, lastLine int) RangeCheckResult {
	for _, r := range lrs.ranges {
		if r.IsInside(line, lastLine) {
			return InRange
		}
	}
	if line > satSub(lastLine, lrs.smallestOffsetFromEnd) {
		return AfterLastRange
	}
	if line < lrs.largestAbsoluteUpperBound {
		return BeforeOrBetweenRanges
	}
	return AfterLastRange
}

// LargestOffsetFromEnd is used by the printer to know how many trailing lines
// must be buffered before end-relative ranges can be resolved.
func (lrs LineRanges) LargestOffsetFromEnd() int { return lrs.largestOffsetFromEndValue }

// HighlightedLineRanges wraps LineRanges used for --highlight-line.
type HighlightedLineRanges struct {
	Ranges LineRanges
}

// DefaultHighlightedLineRanges highlights nothing.
func DefaultHighlightedLineRanges() HighlightedLineRanges {
	return HighlightedLineRanges{Ranges: NoneLineRanges()}
}
