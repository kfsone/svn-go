package svn

import (
	"bytes"
	"fmt"
	"github.com/edsrzf/mmap-go"
	"os"
	"strconv"
)

type DumpFile struct {
	filename string    // Remembers the file path/name we sourced from.
	data     mmap.MMap // The full memory mapped view of the data.
	buffer   []byte    // Read buffer, manipulated by DumpReader.

	DumpFormat int
	UUID       string
}

// NewDumpFile opens and mmaps the given filename into memory,
// then consumes/checks the leading header lines that should
// describe the dump format and UUID. The file is then ready
// to be presented to a Repository to parse/load revisions.
func NewDumpFile(filename string) (*DumpFile, error) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			panic(fmt.Errorf("%w: closing dump file: %s", err, filename))
		}
	}()

	data, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		return nil, err
	}

	df := &DumpFile{
		filename: filename,
		data:     data,
	}

	if df.DumpFormat, df.UUID, df.buffer, err = parseDumpPreamble(data); err != nil {
		return nil, fmt.Errorf("%w: %s", err, filename)
	}

	return df, nil
}

func (df *DumpFile) Close() error {
	return df.data.Unmap()
}

var dumpHeaderSplit = []byte{'\n', '\n'}
var windowsLineEnd = []byte{'\r'}

func parseDumpPreamble(source []byte) (format int, uuid string, body []byte, err error) {
	if format, body, err = readDumpHeaderLine(VersionStringHeader, strconv.Atoi, source); err == nil {
		uuid, body, err = readDumpHeaderLine(UUIDHeader, func(s string) (string, error) { return s, nil }, body)
	}
	return
}

func readDumpHeaderLine[T any](key string, convert func(string) (T, error), source []byte) (value T, body []byte, err error) {
	err = ErrInvalidDumpFile
	// Dump headers have double newlines.
	eol := bytes.Index(source, dumpHeaderSplit)

	if eol != -1 {
		line := source[:eol]
		body = source[eol+len(dumpHeaderSplit):]

		var headerKey, headerValue string
		headerKey, headerValue, err = ReadHeader(line)
		if err == nil && headerKey == key {
			value, err = convert(headerValue)
		}
		// Before we finish, regardless of convert, check if this is actually a windows-encoded dump.
		if bytes.HasSuffix(line, windowsLineEnd) {
			err = ErrWindowsDumpFile
		}
	}

	return
}
