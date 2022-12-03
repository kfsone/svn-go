package svn

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/edsrzf/mmap-go"
)

type DumpFile struct {
	Path       string
	DumpHeader DumpHeader
	Revisions  []*Revision

	Data   mmap.MMap

	reader *DumpReader
}

func (df *DumpFile) GetHead() int {
	return len(df.Revisions) - 1
}

func (df *DumpFile) Close() {
	df.reader.Close()
	df.Data.Unmap()
}

func (df *DumpFile) GetRevision(rev int) (*Revision, error) {
	if rev < 0 {
		return nil, fmt.Errorf("invalid revision #%d", rev)
	}
	if rev >= len(df.Revisions) {
		return nil, fmt.Errorf("revision #%d does not exist", rev)
	}

	return df.Revisions[rev], nil
}

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

	log("dump header: %+#v", dump.DumpHeader)

	dump.Revisions = make([]*Revision, 0, 16384)

	return dump, nil
}

func (df *DumpFile) NextRevision() error {
	if df.reader.Empty() {
		return io.EOF
	}

	rev, err := NewRevision(df.reader)
	if err != nil {
		return err
	}

	if rev.Number != len(df.Revisions) {
		return fmt.Errorf("expected revision %d, got %d", len(df.Revisions), rev.Number)
	}

	df.Revisions = append(df.Revisions, rev)

	return nil
}
