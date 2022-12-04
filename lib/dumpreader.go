package svn

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// DumpReader is a wrapper and series of helpers around a byte slice and size
// that represents a portion of a dump file.
type DumpReader struct {
	buffer []byte
	length int
}

// NewDumpReader allocates a DumpReader object that describes the given byte slice.
func NewDumpReader(source []byte) *DumpReader {
	return &DumpReader{buffer: source, length: len(source)}
}

// Close releases the reference to the buffer.
func (r *DumpReader) Close() {
	r.buffer = nil
	r.length = -1
}

// Offset returns the offset of the first byte in the remaining buffer relative
// to the beginning of the original slice.
func (r *DumpReader) Offset() int {
	return r.length - len(r.buffer)
}

// Newline will attempt to consume a single newline character at the beginning
// of the buffer. Returns true if a newline was consumed, otherwise false.
func (r *DumpReader) Newline() bool {
	if len(r.buffer) > 0 && r.buffer[0] == '\n' {
		r.buffer = r.buffer[1:]
		return true
	}
	return false
}

// LineAfter checks if the first characters in the reader match prefix, if so, it will
// consume the entire line returning the portion after prefix, before the newline.
// If the prefix does not match, the reader is left unchanged and false is returned.
func (r *DumpReader) LineAfter(prefix string) (line string, ok bool) {
	if bytes.HasPrefix(r.buffer, []byte(prefix)) {
		newline := bytes.IndexRune(r.buffer[len(prefix):], '\n')
		if newline == -1 {
			line, r.buffer = string(r.buffer[len(prefix):]), r.buffer[len(r.buffer):]
		} else {
			line, r.buffer = string(r.buffer[len(prefix):len(prefix)+newline]), r.buffer[len(prefix)+newline+1:]
		}
		return line, true
	}
	return "", false
}

// IntAfter will check if the line begins with prefix + ": ", and if so, will consume
// the line and attempt to parse the remainder of the line as an integer. If the prefix
// does not match, the reader is left unchanged and ErrMissingField is returned.
func (r *DumpReader) IntAfter(prefix string) (int, error) {
	str, present := r.LineAfter(prefix + ": ")
	if !present {
		return 0, fmt.Errorf("%w: %s; got: %s", ErrMissingField, prefix, r.Peek(32))
	}
	return strconv.Atoi(str)
}

// Read attempts to consume the specified number of bytes from the reader and returns
// a slice representing them. If the reader does not have enough bytes, ErrUnexpectedEOF
// is returned.
func (r *DumpReader) Read(length int) (data []byte, err error) {
	if length > len(r.buffer) {
		return nil, io.ErrUnexpectedEOF
	}

	data, r.buffer = r.buffer[:length], r.buffer[length:]

	return data, nil
}

// Discard attempts to dsicard bytes from the front of the reader and returns the
// number of bytes discarded and an error if the count is less than the amount
// requested. If the length was greater than the size of the remaining buffer,
// the error is io.ErrUnexpectedEOF.
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

// ReadSized attempts to read a pascal-sized labelled value from the reader.
// This is where the first byte represents the type of field (K: key, V: Value,
// D: deletion), followed by an ascii representation of the length of the field,
// and a line feed, followed by length bytes of data and another line feed.
// E.g.
//
//	K 10<LF>
//	svn:ignore<LF>
func (r *DumpReader) ReadSized(prefix rune) (value []byte, err error) {
	// First line should be "{prefix} <digits>\n"
	sizeStr, ok := r.LineAfter(string(prefix) + " ")
	if !ok {
		return nil, fmt.Errorf("expected '%c' prefix; got: %s", prefix, r.Peek(48))
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

// AtEOF returns true if there is no data left in the reader.
func (r *DumpReader) AtEOF() bool {
	return len(r.buffer) == 0
}

// Length returns the remaining byte count of the reader.
func (r *DumpReader) Length() int {
	return len(r.buffer)
}

// Peek returns a byte slice containing the immediate N bytes at the front
// of the reader, without actually consuming them.
// Invalidated by the next read-type operation.
func (r *DumpReader) Peek(length int) (data []byte) {
	if length > len(r.buffer) {
		length = len(r.buffer)
		return r.buffer[:length]
	}

	return []byte(string(r.buffer[:length]) + "...")
}
