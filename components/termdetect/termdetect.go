// Package terminfo detects terminal properties bat relies on: whether stdout/
// stdin are interactive ttys, the terminal width, true-color support and
// whether color output should be enabled at all.
package terminfo

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// IsTerminal reports whether the given file descriptor is an interactive tty.
func IsTerminal(fd uintptr) bool { return term.IsTerminal(int(fd)) }

// StdoutIsTerminal reports whether stdout is an interactive tty.
func StdoutIsTerminal() bool { return IsTerminal(os.Stdout.Fd()) }

// StdinIsTerminal reports whether stdin is an interactive tty.
func StdinIsTerminal() bool { return IsTerminal(os.Stdin.Fd()) }

// Width returns the terminal width in columns. It honors an explicit COLUMNS
// environment variable, then queries the tty, and finally falls back to 80.
func Width() int {
	if c := os.Getenv("COLUMNS"); c != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(c)); err == nil && n > 0 {
			return n
		}
	}
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}

// TrueColor reports whether the terminal advertises 24-bit color support via
// COLORTERM.
func TrueColor() bool {
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	return ct == "truecolor" || ct == "24bit"
}

// ColorEnabled reports whether colorized output should be produced, honoring
// the NO_COLOR convention and a dumb TERM.
func ColorEnabled() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}
