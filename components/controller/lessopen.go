package controller

import (
	"os"
	"os/exec"
	"strings"
)

// lessOpenProcess runs the $LESSOPEN preprocessor on a file, returning the
// preprocessed bytes. It supports the common "|command %s" (pipe to stdout)
// form used by less. ok is false when no preprocessing applies, in which case
// the original content should be used.
func lessOpenProcess(path string) (content []byte, ok bool) {
	tmpl := os.Getenv("LESSOPEN")
	if tmpl == "" {
		return nil, false
	}
	// A leading "||" means "discard original on empty output"; "|" means the
	// command writes to stdout. We always read stdout, so strip the markers.
	tmpl = strings.TrimPrefix(tmpl, "||")
	tmpl = strings.TrimPrefix(tmpl, "|")
	tmpl = strings.TrimPrefix(tmpl, "-")
	if !strings.Contains(tmpl, "%s") {
		return nil, false
	}
	cmdStr := strings.Replace(tmpl, "%s", shellQuote(path), 1)

	cmd := exec.Command("sh", "-c", cmdStr)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil, false
	}
	return out, true
}

// shellQuote wraps s in POSIX single quotes, escaping embedded quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
