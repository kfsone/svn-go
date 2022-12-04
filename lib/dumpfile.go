package svn

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/edsrzf/mmap-go"
)

// DumpFile encapsulates the key attributes of an svn dump file.
type DumpFile struct {
	Path       string
	DumpHeader *DumpHeader
	Revisions  []*Revision

	Data mmap.MMap

	reader *DumpReader
}

// Close releases resources held by the dump. Note: This will invalidate
// any slices referencing the data since it releases the mmap.
func (df *DumpFile) Close() {
	df.reader.Close()
	df.Data.Unmap()
}

// GetHead returns the highest revision number represented by the dump.
func (df *DumpFile) GetHead() int {
	return len(df.Revisions) - 1
}

// GetRevision returns either a pointer to the record for the specified
// revision or an error explaining why it could not.
func (df *DumpFile) GetRevision(rev int) (*Revision, error) {
	// Disallow negative revision numbers.
	if rev < 0 {
		return nil, fmt.Errorf("invalid revision #%d", rev)
	}
	// Bounds check the revision number.
	if rev >= len(df.Revisions) {
		return nil, fmt.Errorf("revision #%d does not exist", rev)
	}

	return df.Revisions[rev], nil
}

// checkValidSource tests that a mapped file looks like an actual, valid svn dump.
// Also checks that the user created the dump with "-F" by testing whether the
// first line has windows (CRLF) line endings. The OS adds these when svnadmin
// writes to the console and invalidates all of the headers by making the byte
// counts wrong (svnadmin is unaware these characters are being added).
func checkValidSource(source []byte) error {
	if !bytes.HasPrefix(source, []byte(VersionStringHeader+":")) {
		return errors.New("missing dump format header, not an svnadmin dump file?")
	}

	// Now check that there's a newline on this line, but don't look too far.
	lf := bytes.IndexByte(source[:len(VersionStringHeader)*2], '\n')
	if lf < len(VersionStringHeader) {
		return errors.New("unrecognized dump file format, not an svnadmin dump file?")
	}

	// Great, just check there's no <cr> caused by outputting it to a CRLF console.
	if cr := bytes.IndexByte(source[:lf], '\r'); cr != -1 {
		return errors.New("windows line-ending translations detected, on windows use `svnadmin dump -F filename` rather than redirecting output")
	}

	return nil
}

// NewDumpFile creates a new DumpFile representation of a disk file,
// mapping it into memory and parsing the header.
func NewDumpFile(path string) (dump *DumpFile, err error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		return nil, err
	}

	if err := checkValidSource(data); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	dump = &DumpFile{Path: path, Data: data}
	dump.reader = NewDumpReader(data)
	if dump.DumpHeader, err = NewDumpHeader(dump.reader); err != nil {
		return nil, err
	}

	log("dump header: %+#v", *dump.DumpHeader)

	// Start the list big so it doesn't have to spend a lot of time growing.
	dump.Revisions = make([]*Revision, 0, 32768)

	return dump, nil
}

// NextRevision attempts to read the next revision from the dump file, or
// returns io.EOF if the end of file has been reached.
func (df *DumpFile) NextRevision() (*Revision, error) {
	if df.reader.AtEOF() {
		return nil, io.EOF
	}

	rev, err := NewRevision(df.reader)
	if err != nil {
		return nil, err
	}

	if rev.Number != len(df.Revisions) {
		return rev, fmt.Errorf("expected revision %d, got %d", len(df.Revisions), rev.Number)
	}

	df.Revisions = append(df.Revisions, rev)

	return rev, nil
}
