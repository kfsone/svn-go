package svn

import (
	"errors"
	"fmt"
	"io"
	"math"
)

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

func (r *Repos) LoadRevisions(filename string, maxRev int) (err error) {
	dumpfile, err := NewDumpFile(filename)
	if err != nil {
		return err
	}

	dump, err := NewDumpReader(dumpfile, r.DumpFormat, r.UUID)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() {
		if err := dump.Close(); err != nil {
			panic(fmt.Errorf("closing reader: %w", err))
		}
	}()

	// Make sure the dumpfile actually contains anything.
	revisions := make([]*Revision, 0, 4096)
	if maxRev < 0 {
		maxRev = math.MaxInt
	}
	for revNo := len(r.Revisions); revNo <= maxRev; revNo++ {
		rev, err := NewRevision(dump)
		if err != nil {
			// SVN dump format doesn't provide a revision count, so we're just expecting
			// to hit EOF at some point.
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("r%d: %w", revNo, err)
		}

		if rev.Number != revNo {
			return fmt.Errorf("%w: out-of-sequence revision: expected %d, got %d", ErrDumpHeaderMismatch, revNo, rev.Number)
		}

		if err = rev.Load(); err != nil {
			return fmt.Errorf("r%d: %w", revNo, err)
		}

		revisions = append(revisions, rev)
	}

	// The dumpfile is now a keeper.
	r.DumpFiles = append(r.DumpFiles, dumpfile)

	// Add the revisions to our own.
	r.Revisions = append(r.Revisions, revisions...)

	if r.DumpFormat == 0 {
		r.DumpFormat = dump.DumpFormat
		r.UUID = dump.UUID
	}

	return nil
}

func (r *Repos) Encode(encoder *Encoder, start, end int) <-chan float64 {
	// Encode the header.
	// We currently guarantee there are only these two headers.
	encoder.Fprintf("%s: %d\n\n%s: %s\n\n", VersionStringHeader, r.DumpFormat, UUIDHeader, r.UUID)

	ch := make(chan float64)

	go func() {
		defer close(ch)
		r, encoder, start, end := r, encoder, start, end
		total := float64(end - start + 1)

		for i := start; i <= end; i++ {
			ch <- float64(end-start) * 100.0 / total
			r.Revisions[i].Encode(encoder)
		}
		ch <- 100.0
	}()

	return ch
}
