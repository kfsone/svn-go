package svn

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

type DumpReader struct {
	buffer []byte
	length int
}

func NewDumpReader(source []byte) *DumpReader {
	return &DumpReader{buffer: source, length: len(source)}
}

func (r *DumpReader) Close() {
	r.buffer = nil
}

func (r *DumpReader) Offset() int {
	return r.length - len(r.buffer)
}

func (r *DumpReader) Newline() (ok bool) {
	if len(r.buffer) > 0 && r.buffer[0] == '\n' {
		r.buffer = r.buffer[1:]
		ok = true
	}

	return ok
}

func (r *DumpReader) Line() (string, error) {
	if len(r.buffer) == 0 {
		return "", io.EOF
	}
	newline := bytes.IndexRune(r.buffer, '\n')
	if newline == -1 {
		return "", io.ErrUnexpectedEOF
	}
	var line []byte

	line, r.buffer = r.buffer[:newline], r.buffer[newline+1:]

	return string(line), nil
}

func (r *DumpReader) LineAfter(prefix string) (string, bool) {
	if bytes.HasPrefix(r.buffer, []byte(prefix)) {
		if line, err := r.Line(); err == nil {
			return line[len(prefix):], true
		}
	}
	return "", false
}

func (r *DumpReader) IntAfter(prefix string) (int, error) {
	str, present := r.LineAfter(prefix + ": ")
	if !present {
		return 0, fmt.Errorf("%w: %s; got: %s", ErrMissingField, prefix, r.Peek(32))
	}
	return strconv.Atoi(str)
}

func (r *DumpReader) Read(length int) (data []byte, err error) {
	if length > len(r.buffer) {
		return nil, io.ErrUnexpectedEOF
	}

	data, r.buffer = r.buffer[:length], r.buffer[length:]

	return data, nil
}

func (r *DumpReader) Discard(length int) (discard int, err error) {
	if length > len(r.buffer) {
		discard = len(r.buffer)
		err = io.ErrUnexpectedEOF
	} else {
		discard = length
	}
	r.buffer = r.buffer[discard:]
	return discard, err
}

func (r *DumpReader) ReadSized(prefix rune) (value []byte, err error) {
	// First line should be "{prefix} <digits>\n"
	sizeStr, ok := r.LineAfter(string(prefix) + " ")
	if !ok {
		return nil, fmt.Errorf("expected '%c' prefix", prefix)
	}
	size, err := strconv.Atoi(string(sizeStr))
	if err != nil {
		return nil, fmt.Errorf("invalid '%c' size: %w", prefix, err)
	}
	if value, err = r.Read(size); err != nil {
		return nil, err
	}
	if !r.Newline() {
		return nil, fmt.Errorf("%w: after sized %c data: %s", ErrMissingNewline, prefix, string(value))
	}

	return value, nil
}

func (r *DumpReader) Empty() bool {
	return len(r.buffer) == 0
}

func (r *DumpReader) Length() int {
	return len(r.buffer)
}

func (r *DumpReader) Peek(length int) (data []byte) {
	if length > len(r.buffer) {
		length = len(r.buffer)
		return r.buffer[:length]
	}

	return []byte(string(r.buffer[:length]) + "...")
}
