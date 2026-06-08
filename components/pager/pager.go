// Package output ports bat's output module: it decides whether to write to
// stdout directly or through a pager (less by default), builds the correct
// less arguments, and exposes a single io.Writer plus Close that waits for the
// pager to exit.
package output

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"goforge.dev/gat/components/config"
)

// Handle is an open output target. Write to it, then Close.
type Handle struct {
	w      io.Writer
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	isPage bool
}

// IsPager reports whether output is going through a pager process.
func (h *Handle) IsPager() bool { return h.isPage }

// Write implements io.Writer.
func (h *Handle) Write(p []byte) (int, error) { return h.w.Write(p) }

// WriteString writes a string.
func (h *Handle) WriteString(s string) (int, error) { return io.WriteString(h.w, s) }

// Close flushes and, for a pager, closes stdin and waits for it to exit.
func (h *Handle) Close() error {
	if h.cmd != nil {
		h.stdin.Close()
		return h.cmd.Wait()
	}
	return nil
}

// Stdout returns a plain stdout handle.
func Stdout() *Handle { return &Handle{w: os.Stdout} }

// resolvePager picks the pager command per bat precedence: explicit config,
// then $BAT_PAGER, then $PAGER, defaulting to "less". The returned source flag
// indicates whether it came from $PAGER (which permits arg replacement).
func resolvePager(cfgPager string) (cmd string, fromEnvPager bool) {
	if cfgPager != "" {
		return cfgPager, false
	}
	if p := os.Getenv("BAT_PAGER"); p != "" {
		return p, false
	}
	if p := os.Getenv("PAGER"); p != "" {
		return p, true
	}
	return "less", false
}

// FromMode opens the output target for the given paging mode. interactive
// reports whether stdout is a tty; when it is not, or the mode is Never, output
// goes straight to stdout. chopLongLines toggles less's -S flag. On any failure
// to spawn the pager, it falls back to stdout.
func FromMode(mode config.PagingMode, cfgPager string, interactive, chopLongLines, quitIfOneScreen bool) *Handle {
	if mode == config.PagingNever || !interactive {
		return Stdout()
	}
	pagerStr, fromEnvPager := resolvePager(cfgPager)
	fields := strings.Fields(pagerStr)
	if len(fields) == 0 {
		return Stdout()
	}
	bin := fields[0]
	args := fields[1:]
	base := strings.ToLower(filepath.Base(bin))

	// Refuse a recursive bat/gat pager, like bat does.
	if base == "bat" || base == "gat" {
		return Stdout()
	}

	if base == "less" {
		if len(args) == 0 || fromEnvPager {
			args = []string{"-R"}
			if quitIfOneScreen {
				args = append(args, "-F")
			}
			if chopLongLines {
				args = append(args, "-S")
			}
			args = append(args, "-K", "--no-init")
		}
	}

	cmd := exec.Command(bin, args...)
	if base == "less" {
		cmd.Env = append(os.Environ(), "LESSCHARSET=UTF-8")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Stdout()
	}
	if err := cmd.Start(); err != nil {
		return Stdout()
	}
	return &Handle{w: stdin, cmd: cmd, stdin: stdin, isPage: true}
}
