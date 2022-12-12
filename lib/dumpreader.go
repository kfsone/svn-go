package svn

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

type DumpReader struct {
	*DumpFile
	buffer []byte // Read buffer, manipulated by DumpReader.
}

// NewDumpReader wraps a DumpFile in a DumpReader for convenience, as long as either
// the given format/uuid are defaulted (0/"") or match the DumpFile's values.
func NewDumpReader(df *DumpFile, format int, uuid string) (dump *DumpReader, err error) {
	dump = &DumpReader{DumpFile: df, buffer: df.data}

	// The dump format starts with two header blocks that happen to contain only
	// one header line each.
	//   Svn-Dump-Format-Version: N\n
	//	 \n
	//	 UUID: <uuid>\n
	//	 \n
	df.DumpFormat, err = getDumpHeader(dump, VersionStringHeader, strconv.Atoi)
	if err != nil {
		return nil, err
	}
	if format != 0 && format != df.DumpFormat {
		return nil, fmt.Errorf("%w: format version: expected %d, got %d", ErrDumpHeaderMismatch, format, df.DumpFormat)
	}

	// Parse the repository header which should contain just the UUID.
	df.UUID, err = getDumpHeader(dump, UUIDHeader, func(s string) (string, error) { return s, nil })
	if err != nil {
		return nil, err
	}
	if uuid != "" && uuid != df.UUID {
		return nil, fmt.Errorf("%w: repository UUID: expected %s, got %s", ErrDumpHeaderMismatch, uuid, df.UUID)
	}

	return dump, nil
}

func getDumpHeader[T any](dump *DumpReader, header string, converter func(string) (T, error)) (value T, err error) {
	formatHeader, err := NewHeaders(dump)
	if err == nil {
		var text string
		text, err = formatHeader.String(header)
		if err == nil {
			value, err = converter(text)
			if err == nil && formatHeader.Len() != 1 {
				err = fmt.Errorf("%w: expected single format header, got %d", ErrInvalidDumpFile, formatHeader.Len())
			}
		}
	}

	return
}

// Close will close the read buffer, not the underlying map. The dumpfile itself
// must be closed discretely.
func (r *DumpReader) Close() error {
	r.buffer = r.data[len(r.data):]
	return nil
}

// IsEOI returns true if the dumpfile is at the end of its input.
func (r *DumpReader) IsEOI() bool {
	return len(r.buffer) == 0
}

// PeekLine will return the next line of the dumpfile without advancing the read cursor
// including the newline itself. If the reader is already at EOI, it returns io.EOF.
// If the reader does find another newline, returns io.ErrUnexpectedEOF.
func (r *DumpReader) PeekLine() ([]byte, error) {
	if len(r.buffer) == 0 {
		return nil, io.EOF
	}
	eol := bytes.IndexByte(r.buffer, '\n')
	if eol == -1 {
		return nil, io.ErrUnexpectedEOF
	}

	return r.buffer[:eol+1], nil
}

// Peek will attempt to return the next n bytes from the reader without moving
// the read cursor. If the buffer is at EOI, returns io.EOF, otherwise if the
// buffer does not contain n bytes, returns io.ErrUnexpectedEOF.
func (r *DumpReader) Peek(n int) ([]byte, error) {
	if len(r.buffer) == 0 {
		return nil, io.EOF
	}
	if n > len(r.buffer) {
		return nil, io.ErrUnexpectedEOF
	}

	return r.buffer[:n], nil
}

// Discard advances the read cursor by upto n bytes. If the buffer contained at least
// n bytes, returns true. Otherwise, moves the read cursor to eoi and returns false.
func (r *DumpReader) Discard(n int) bool {
	if n > len(r.buffer) {
		r.buffer = r.buffer[len(r.buffer):]
		return false
	}

	r.buffer = r.buffer[n:]
	return true
}

// Read will return a direct view of the underlying buffer upto n bytes long. If the
// buffer does not contain enough remaining bytes, will return io.ErrUnexpectedEOF,
// or if the buffer is at EOI it will return io.EOF.
func (r *DumpReader) Read(n int) (data []byte, err error) {
	if n == 0 {
		// Return a pointer to our actual offset, but with no length.
		return r.buffer[:0], nil
	}
	if data, err = r.Peek(n); err == nil {
		r.Discard(n)
		return data, nil
	}
	return nil, err
}

// ExpectAndConsume will attempt to discard the given string from the read cursor, returning
// true if the string was present and the cursor moved, otherwise false.
func (r *DumpReader) ExpectAndConsume(s string) bool {
	if !r.HasPrefix(s) {
		return false
	}

	r.Discard(len(s))

	return true
}

// HasPrefix returns true if the read cursor begins with the given string.
func (r *DumpReader) HasPrefix(s string) bool {
	return bytes.HasPrefix(r.buffer, []byte(s))
}

// Tell returns the current offset of the read cursor from the start of the reader's buffer.
func (r *DumpReader) Tell() int {
	return len(r.data) - len(r.buffer)
}
