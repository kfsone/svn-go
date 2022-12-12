package svn

import "fmt"

// Repos represents the loaded model of a Subversion repository.

type Repos struct {
	DumpFormat int    // Dump format version - must be consistent across files.
	UUID       string // UUID of the repository - must be consistent across files.

	Revisions []*Revision // List of revisions in the repository.

	DumpFiles []*DumpFile // A list of all the dump files we loaded.
}

func NewRepos() *Repos {
	return &Repos{
		Revisions: make([]*Revision, 0),
		DumpFiles: make([]*DumpFile, 0),
	}
}

func (r *Repos) GetHead() int {
	// If there are 2 entries, then head is r1. If there is just 1 entry,
	// then head is r0. But if there are no entries, then head is -1.
	return len(r.Revisions) - 1
}

func (r *Repos) Close() error {
	for _, df := range r.DumpFiles {
		if err := df.Close(); err != nil {
			return err
		}
	}
	r.DumpFiles = nil
	return nil
}

func (r *Repos) AddDumpFile(dumpfile *DumpFile) (err error) {
	if len(dumpfile.Revisions) == 0 {
		return fmt.Errorf("dump file contains no revisions")
	}
	if dumpfile.Revisions[0].Number != len(r.Revisions) {
		return fmt.Errorf("revisions out of sequence: expected %d; got %d", len(r.Revisions), dumpfile.Revisions[0].Number)
	}

	if r.DumpFormat != 0 {
		if r.DumpFormat != dumpfile.DumpFormat {
			return fmt.Errorf("%w: dump format mismatch: %d != %d", ErrInvalidDumpFile, r.DumpFormat, dumpfile.DumpFormat)
		}
		if r.UUID != dumpfile.UUID {
			return fmt.Errorf("%w: repository UUID mismatch: %s != %s", ErrInvalidDumpFile, r.UUID, dumpfile.UUID)
		}
	} else {
		r.DumpFormat = dumpfile.DumpFormat
		r.UUID = dumpfile.UUID
	}

	r.Revisions = append(r.Revisions, dumpfile.Revisions...)
	r.DumpFiles = append(r.DumpFiles, dumpfile)

	return nil
}

type EncodingProgress struct {
	Revision int
	Percent  float64
}

func (r *Repos) Encode(encoder *Encoder, start, end int) <-chan EncodingProgress {
	// Encode the header.
	// We currently guarantee there are only these two headers.
	encoder.Fprintf("%s: %d\n\n%s: %s\n\n", VersionStringHeader, r.DumpFormat, UUIDHeader, r.UUID)

	ch := make(chan EncodingProgress, 4)

	go func() {
		defer close(ch)
		r, encoder, start, end := r, encoder, start, end
		total := float64(r.GetHead() + 1)

		for i := start; i <= end; i++ {
			ch <- EncodingProgress{i, float64(i) * 100.0 / total}
			r.Revisions[i].Encode(encoder)
		}
	}()

	return ch
}
