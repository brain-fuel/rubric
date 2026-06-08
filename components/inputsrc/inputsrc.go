// Package input ports bat's input module: the description of an input source
// (an ordinary file, standard input, or in-memory bytes), how it is opened for
// reading, and the display name / size metadata the printer needs for headers.
package input

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// Kind identifies the variety of an input source.
type Kind int

const (
	KindFile Kind = iota
	KindStdin
	KindBytes
)

// Input describes a source before it is opened.
type Input struct {
	kind  Kind
	path  string
	name  string // explicit display name override (--file-name)
	bytes []byte
}

// OrdinaryFile describes a file on disk.
func OrdinaryFile(path string) Input { return Input{kind: KindFile, path: path} }

// StdIn describes standard input.
func StdIn() Input { return Input{kind: KindStdin} }

// FromBytes describes an in-memory buffer (used by the library API / tests).
func FromBytes(b []byte) Input { return Input{kind: KindBytes, bytes: b} }

// WithName sets an explicit display name (the --file-name override).
func (i Input) WithName(name string) Input { i.name = name; return i }

// Kind returns the source variety.
func (i Input) Kind() Kind { return i.kind }

// Path returns the file path (empty for non-file inputs).
func (i Input) Path() string { return i.path }

// Opened is a ready-to-read input with resolved metadata.
type Opened struct {
	Reader *bufio.Reader
	Name   string // display name for headers
	Kind   Kind
	Path   string
	Size   int64 // -1 when unknown (stdin)

	closer io.Closer
}

// Open resolves and opens the input. stdin supplies the reader for KindStdin so
// callers (and tests) can inject it.
func (i Input) Open(stdin io.Reader) (*Opened, error) {
	switch i.kind {
	case KindFile:
		f, err := os.Open(i.path)
		if err != nil {
			return nil, err
		}
		name := i.path
		if i.name != "" {
			name = i.name
		}
		size := int64(-1)
		if st, err := f.Stat(); err == nil {
			if st.IsDir() {
				f.Close()
				return nil, fmt.Errorf("'%s' is a directory", i.path)
			}
			size = st.Size()
		}
		return &Opened{
			Reader: bufio.NewReader(f),
			Name:   name,
			Kind:   KindFile,
			Path:   i.path,
			Size:   size,
			closer: f,
		}, nil
	case KindStdin:
		name := "STDIN"
		if i.name != "" {
			name = i.name
		}
		return &Opened{
			Reader: bufio.NewReader(stdin),
			Name:   name,
			Kind:   KindStdin,
			Size:   -1,
		}, nil
	case KindBytes:
		name := "<bytes>"
		if i.name != "" {
			name = i.name
		}
		return &Opened{
			Reader: bufio.NewReader(bytesReader(i.bytes)),
			Name:   name,
			Kind:   KindBytes,
			Size:   int64(len(i.bytes)),
		}, nil
	default:
		return nil, fmt.Errorf("unknown input kind")
	}
}

// Close releases any underlying file handle.
func (o *Opened) Close() error {
	if o.closer != nil {
		return o.closer.Close()
	}
	return nil
}

// ReadAll returns the full input contents.
func (o *Opened) ReadAll() ([]byte, error) {
	return io.ReadAll(o.Reader)
}

func bytesReader(b []byte) io.Reader {
	return &sliceReader{b: b}
}

type sliceReader struct {
	b   []byte
	pos int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}
