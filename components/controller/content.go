package controller

import (
	"regexp"
	"strings"
	"unicode/utf16"
)

// ContentType classifies buffered input, mirroring bat's content_inspector.
type ContentType int

const (
	ContentEmpty ContentType = iota
	ContentBinary
	ContentUTF8
	ContentUTF16LE
	ContentUTF16BE
)

func (c ContentType) isText() bool {
	return c == ContentUTF8 || c == ContentUTF16LE || c == ContentUTF16BE
}

// detectContentType inspects the leading bytes for BOMs and NUL bytes.
func detectContentType(b []byte) ContentType {
	if len(b) == 0 {
		return ContentEmpty
	}
	if len(b) >= 2 {
		if b[0] == 0xFF && b[1] == 0xFE {
			return ContentUTF16LE
		}
		if b[0] == 0xFE && b[1] == 0xFF {
			return ContentUTF16BE
		}
	}
	n := len(b)
	if n > 1024 {
		n = 1024
	}
	for _, c := range b[:n] {
		if c == 0 {
			return ContentBinary
		}
	}
	return ContentUTF8
}

// decodeContent converts buffered bytes to a UTF-8 string, decoding UTF-16 and
// stripping a leading BOM, matching bat's per-encoding handling.
func decodeContent(b []byte, ct ContentType) string {
	switch ct {
	case ContentUTF16LE:
		return decodeUTF16(b[2:], false)
	case ContentUTF16BE:
		return decodeUTF16(b[2:], true)
	default:
		s := string(b)
		// Strip a UTF-8 BOM if present.
		return strings.TrimPrefix(s, "\ufeff")
	}
}

func decodeUTF16(b []byte, bigEndian bool) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, len(b)/2)
	for i := 0; i < len(u16); i++ {
		if bigEndian {
			u16[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			u16[i] = uint16(b[2*i+1])<<8 | uint16(b[2*i])
		}
	}
	return string(utf16.Decode(u16))
}

var ansiEscapeRE = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)|\x1b[@-Z\\\\-_]")

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string { return ansiEscapeRE.ReplaceAllString(s, "") }

// stripOverstrike removes man-page overstrike formatting (X\bX bold, _\bX
// underline), keeping the visible characters. Ports bat's strip_overstrike.
func stripOverstrike(s string) string {
	if !strings.ContainsRune(s, '\b') {
		return s
	}
	var out []rune
	for _, r := range s {
		if r == '\b' {
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
