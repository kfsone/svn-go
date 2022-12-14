package svn

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Headers are a simple RFC-822 style collection of headers as a map for
// ease of access, that can easily be re-emitted as-is.
type Headers struct {
	index    []string          // Preserve the order of the keys.
	table    map[string]string // Map keys to values for non-deletions.
	newlines int               // Count of new lines that followed.
}

// NewHeaders returns a default constructed Headers object from the given reader.
func NewHeaders(dump *DumpReader) (h *Headers, err error) {
	h = &Headers{
		index:    make([]string, 0),
		table:    make(map[string]string),
		newlines: 0,
	}

	for {
		line, err := dump.PeekLine()
		if err != nil {
			return nil, err
		}

		// Extract the part before the newline.
		content := line[:len(line)-1]

		// Once we see a line with 0 length, we're at a newline denoting end of the block.
		if len(content) == 0 {
			break
		}

		key, value, err := ReadHeader(content)
		if err != nil {
			return nil, err
		}
		h.index = append(h.index, key)
		h.table[key] = value

		dump.Discard(len(line))
	}

	if dump.ExpectAndConsume("\n") {
		h.newlines++
	}

	return h, nil
}

var headerSplit = []byte{':', ' '}

// ReadHeader interprets a byte slice as an RFC-822 style header and adds it to
// the Headers index and table.
func ReadHeader(line []byte) (key string, value string, err error) {
	// Eliminate trailing newline.
	colon := bytes.Index(line, headerSplit)
	if colon == -1 {
		lineText := strings.ReplaceAll(string(line), "\r", "\\r")
		return "", "", fmt.Errorf("malformed header line: %s", lineText)
	}

	// We're going to trust the dump file format and assume that headers only
	// appear once. Not our job to validate the header format.
	key, value = string(line[:colon]), string(line[colon+len(headerSplit):])

	return key, value, nil
}

func (h *Headers) Has(key string) bool {
	_, ok := h.table[key]
	return ok
}

func (h *Headers) Int(key string) (int, error) {
	value, ok := h.table[key]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrMissingField, key)
	}
	ret, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidHeader, err)
	}
	return ret, nil
}

func (h *Headers) String(key string) (string, error) {
	value, ok := h.table[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrMissingField, key)
	}
	return value, nil
}

func (h *Headers) Len() int {
	return len(h.index)
}

func (h *Headers) Set(key, value string) {
	h.table[key] = value
}

func (h *Headers) Encode(encoder *Encoder) {
	// Write the headers in the original order
	buffer := make([]byte, 0, len(h.index)*80)
	for _, key := range h.index {
		buffer = append(buffer, []byte(key)...)
		buffer = append(buffer, headerSplit...)
		buffer = append(buffer, []byte(h.table[key])...)
		buffer = append(buffer, '\n')
	}

	for i := 0; i < h.newlines; i++ {
		buffer = append(buffer, '\n')
	}

	encoder.Write(buffer)
}

func (h *Headers) Remove(key string) {
	delete(h.table, key)
	idx := Index(h.index, key)
	if idx != -1 {
		h.index = append(h.index[:idx], h.index[idx+1:]...)
	}
}
