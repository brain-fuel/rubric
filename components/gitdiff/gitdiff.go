// Package gitdiff ports bat's diff module: it computes per-line modifications
// (added / removed / modified) between a file's working-tree state and the git
// index, so the printer can draw change markers in the gutter. It shells out to
// the git CLI with -U0 and parses the resulting hunk headers, mirroring the
// classification libgit2 performs in upstream bat.
package gitdiff

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LineChange classifies a single line's git status.
type LineChange int

const (
	Added LineChange = iota
	RemovedAbove
	RemovedBelow
	Modified
)

// LineChanges maps 1-based line numbers to their change kind.
type LineChanges map[int]LineChange

var hunkRE = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// Get returns the line changes for filename, or nil if it is not in a git
// repository or git is unavailable.
func Get(filename string) LineChanges {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return nil
	}
	dir := filepath.Dir(abs)

	top, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil
	}
	repoRoot := strings.TrimSpace(top)
	rel, err := filepath.Rel(repoRoot, abs)
	if err != nil {
		return nil
	}

	out, err := runGit(repoRoot, "--no-pager", "diff", "--no-color", "--no-ext-diff", "-U0", "--", rel)
	if err != nil {
		return nil
	}

	changes := LineChanges{}
	for _, line := range strings.Split(out, "\n") {
		m := hunkRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		oldLines := optInt(m[2], 1)
		newStart := mustInt(m[3])
		newLines := optInt(m[4], 1)
		newEnd := newStart + newLines - 1

		switch {
		case oldLines == 0 && newLines > 0:
			mark(changes, newStart, newEnd, Added)
		case newLines == 0 && oldLines > 0:
			if newStart == 0 {
				mark(changes, 1, 1, RemovedAbove)
			} else {
				mark(changes, newStart, newStart, RemovedBelow)
			}
		default:
			mark(changes, newStart, newEnd, Modified)
		}
	}
	if len(changes) == 0 {
		return nil
	}
	return changes
}

func mark(c LineChanges, start, end int, kind LineChange) {
	for l := start; l <= end; l++ {
		c[l] = kind
	}
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}

func mustInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func optInt(s string, def int) int {
	if s == "" {
		return def
	}
	return mustInt(s)
}
