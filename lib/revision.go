package svn

import (
	"fmt"
	"io"
)

type Revision struct {
	Number     int         // Repository's number for this revision.
	Headers    *Headers    // Table of headers for this revision.
	Properties *Properties // Table of svn:properties attached to the revision.
	Nodes      []*Node     // The actual file/directory changes in the revision.

	dump        *DumpReader // The dump file this revision is from.
	startOffset int         // Offset of first byte of this rev in that dump.
	endOffset   int         // Offset of last byte of this rev in that dump.

	contentData []byte // The raw content data for this revision.
}

func NewRevision(dump *DumpReader) (rev *Revision, err error) {
	rev = &Revision{
		dump:        dump,
		startOffset: dump.Tell(),
	}

	if rev.Headers, err = NewHeaders(dump); err != nil {
		// Not an error, just no more revisions.
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}

	// Extract the revision number.
	if rev.Number, err = rev.Headers.Int(RevisionNumberHeader); err != nil {
		return nil, err
	}

	// Find the length of the property data.
	var propLen int
	propLen, err = rev.Headers.Int(PropContentLengthHeader)
	if err != nil {
		return nil, fmt.Errorf("r%d: %w", err)
	}

	// Construct the property table.
	if rev.Properties, err = NewProperties(dump, propLen); err != nil {
		return nil, err
	}

	// Load it in: make this async later if we need the perf.
	if err = rev.Properties.Load(); err != nil {
		return nil, fmt.Errorf("r%d: loading properties: %w")
	}

	return rev, nil
}

func (r *Revision) Close() error {
	return r.dump.Close()
}

// Load the nodes associated with this revision.
func (r *Revision) Load() (err error) {
	// Optimistically allocate a large block to reduce the number of reallocs.
	nodes := make([]*Node, 0, 4096)
	var node *Node

	for r.dump.HasPrefix(NodePathHeader) {
		if node, err = NewNode(r); err != nil {
			return fmt.Errorf("r%d: %w", r.Number, err)
		}
		nodes = append(nodes, node)
	}

	r.Nodes = append(r.Nodes, nodes...)

	return nil
}
