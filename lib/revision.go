package svn

import (
	"fmt"
)

type Revision struct {
	Number     int         // Repository's number for this revision.
	Headers    *Headers    // Table of headers for this revision.
	Properties *Properties // Table of svn:properties attached to the revision.
	Nodes      []*Node     // The actual file/directory changes in the revision.

	dump        *DumpReader // The dump file this revision is from.
	startOffset int         // Offset of first byte of this rev in that dump.
	endOffset   int         // Offset of last byte of this rev in that dump.
}

func NewRevision(dump *DumpReader) (rev *Revision, err error) {
	rev = &Revision{
		dump:        dump,
		startOffset: dump.Tell(),
	}

	if rev.Headers, err = NewHeaders(dump); err != nil {
		return nil, fmt.Errorf("revision headers: %w", err)
	}

	// Extract the revision number.
	if rev.Number, err = rev.Headers.Int(RevisionNumberHeader); err != nil {
		return nil, fmt.Errorf("revision header: %w", err)
	}

	// Find the length of the property data.
	var propLen int
	propLen, err = rev.Headers.Int(PropContentLengthHeader)
	if err != nil {
		return nil, err
	}
	if propLen == 0 {
		panic("zero length properties")
	}

	// Construct the property table.
	if rev.Properties, err = NewProperties(dump, propLen); err != nil {
		return nil, err
	}

	// Load it in: make this async later if we need the perf.
	if err = rev.Properties.Load(); err != nil {
		return nil, fmt.Errorf("loading properties: %w", err)
	}

	// Empty line between revision header and nodes.
	if !dump.ExpectAndConsume("\n") {
		return nil, fmt.Errorf("missing terminating newline after revision header")
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

	for {
		if !r.dump.HasPrefix(NodePathHeader) {
			break
		}
		if node, err = NewNode(r); err != nil {
			return err
		}
		nodes = append(nodes, node)
	}

	r.Nodes = append(r.Nodes, nodes...)

	r.endOffset = r.dump.Tell()

	r.dump.ExpectAndConsume("\n")

	return nil
}

func (r *Revision) FindNode(predicate func(*Node) bool) *Node {
	for _, node := range r.Nodes {
		if predicate(node) {
			return node
		}
	}
	return nil
}

// GetNodeIndexesWithPrefix returns a list of node indexes that match the given path-
// component prefix (distinguishing Model/ from Models/)
func (r *Revision) GetNodeIndexesWithPrefix(prefix string) []int {
	nodes := make([]int, 0)
	for idx, node := range r.Nodes {
		if MatchPathPrefix(node.Path(), prefix) {
			nodes = append(nodes, idx)
		}
	}
	return nodes
}

func (r *Revision) Encode(encoder *Encoder) {
	// Encode the headers ready for writing in binary form.
	properties := r.Properties.Bytes()

	// Update the length headers, incase the length changed.
	r.Headers.Set(PropContentLengthHeader, fmt.Sprintf("%d", len(properties)))
	r.Headers.Set(ContentLengthHeader, fmt.Sprintf("%d", len(properties)))

	// Encode the headers.
	r.Headers.Encode(encoder)

	// Write the properties as binary data, with a trailing \n.
	encoder.Write(properties)
	encoder.Write([]byte{'\n'})

	for _, node := range r.Nodes {
		node.Encode(encoder)
	}
}
