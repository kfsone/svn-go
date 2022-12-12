package svn

import (
	"fmt"
	"io"
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

func (r *Repos) LoadRevisions(filename string) (err error) {
	dumpfile, err := NewDumpFile(filename)
	if err != nil {
		return err
	}

	dump, err := NewDumpReader(dumpfile, r.DumpFormat, r.UUID)
	if err != nil {
		return err
	}
	defer dump.Close()

	// Make sure the dumpfile actually contains anything.
	revisions := make([]*Revision, 0, 4096)
	for {
		rev, err := NewRevision(dump)
		if err != nil {
			// SVN dump format doesn't provide a revision count, so we're just expecting
			// to hit EOF at some point.
			if err == io.EOF {
				break
			}
			return fmt.Errorf("r%d: %w", len(r.Revisions), err)
		}

		if rev.Number == 0 {
			return fmt.Errorf("%w: out-of-sequence revision: expected %d, got %d", ErrDumpHeaderMismatch, len(r.Revisions), rev.Number)
		}

		revisions = append(revisions, rev)
	}

	// The dumpfile is now a keeper.
	r.DumpFiles = append(r.DumpFiles, dumpfile)

	// Add the revisions to our own.
	r.Revisions = append(r.Revisions, revisions...)

	return nil
}
