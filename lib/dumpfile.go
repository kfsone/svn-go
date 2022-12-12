package svn

import (
	"bytes"
	"fmt"
	"github.com/edsrzf/mmap-go"
	"os"
)

type DumpFile struct {
	filename string    // Remembers the file path/name we sourced from.
	data     mmap.MMap // The full memory mapped view of the data.

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

	if err = checkDumpFormat(data); err != nil {
		return nil, err
	}

	return df, nil
}

func (df *DumpFile) Close() error {
	return df.data.Unmap()
}

func checkDumpFormat(source []byte) (err error) {
	if !bytes.HasPrefix(source, []byte(VersionStringHeader)) {
		return ErrInvalidDumpFile
	}
	eol := bytes.IndexRune(source, '\n')
	if eol < 0 {
		return ErrInvalidDumpFile
	}
	// Because of the prefix check, we know eol > 1.
	if source[eol-1] == '\r' {
		return ErrWindowsDumpFile
	}

	return nil
}
