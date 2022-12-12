package svn

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/edsrzf/mmap-go"
	"io"
	"os"
)

type DumpFile struct {
	Filename  string      // Remembers the file path/name we sourced from.
	data      mmap.MMap   // The full memory mapped view of the data.
	Revisions []*Revision // Which revisions were in this file.

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
		Filename:  filename,
		data:      data,
		Revisions: make([]*Revision, 0),
	}

	if err = checkDumpFormat(data); err != nil {
		return nil, err
	}

	return df, nil
}

func (df *DumpFile) LoadRevisions() (err error) {
	dump, err := NewDumpReader(df)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	// we'll need to close that at the end.
	defer func() {
		if err := dump.Close(); err != nil {
			panic(fmt.Errorf("closing reader: %w", err))
		}
	}()

	revisions := make([]*Revision, 0, 4096)

	for revNo := 0; !dump.IsEOI(); revNo++ {
		rev, err := NewRevision(dump)
		if err != nil {
			// SVN dump format doesn't provide a revision count, so we're just expecting
			// to hit EOF at some point.
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("revision #%d: %w", revNo, err)
		}

		if err = rev.Load(); err != nil {
			return fmt.Errorf("r%d: %w", rev.Number, err)
		}

		revisions = append(revisions, rev)
	}

	// The dumpfile is now a keeper.
	df.Revisions = append(df.Revisions, revisions...)

	return nil
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
