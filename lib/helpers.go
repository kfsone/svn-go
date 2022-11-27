package svn

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// Helpers

func SkipNewline(source []byte) ([]byte, bool) {
	if len(source) >= 1 && source[0] == '\n' {
		return source[1:], true
	} else if bytes.HasPrefix(source, []byte("\r\n")) {
		return source[2:], true
	}
	return source, false
}

func EndLine(source []byte) ([]byte, []byte) {
	eol := bytes.IndexRune(source, '\n')

	if eol == -1 || eol == len(source) {
		return source, source[len(source):]
	}

	return source[:eol], source[eol+1:]
}

func readSized(source []byte, prefix rune) (value []byte, remainder []byte, crs int64, err error) {
	failure := func(err error) ([]byte, []byte, int64, error) {
		return nil, source, 0, err
	}

	// First line should be "{prefix} <digits>\r?\n"
	if !bytes.HasPrefix(source, []byte{byte(prefix), ' '}) {
		return failure(fmt.Errorf("expected '%c' prefix", prefix))
	}
	sizeStr, remainder := EndLine(source[2:])
	if len(sizeStr) == 0 {
		return failure(fmt.Errorf("invalid '%c' size", prefix))
	}
	if bytes.HasSuffix(sizeStr, []byte{'\r'}) {
		crs++
		sizeStr = sizeStr[:len(sizeStr)-1]
	}
	size, err := strconv.ParseInt(string(sizeStr), 10, 64)
	if err != nil {
		return failure(fmt.Errorf("invalid '%c' size: %w", prefix, err))
	}
	// Check for capacity
	if size >= int64(len(remainder)) {
		return failure(io.EOF)
	}
	value, remainder = remainder[:size], remainder[size:]
	if len(remainder) > 0 && remainder[0] == '\r' {
		crs++
		remainder = remainder[1:]
	}
	if len(remainder) == 0 || remainder[0] != '\n' {
		return failure(fmt.Errorf("expected newline after %s value %s", prefix, value))
	}
	return value, remainder[1:], crs, nil
}
