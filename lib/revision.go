package svn

import (
	"fmt"
	"io"
)

type Revision struct {
	Number        int         `yaml:"rev,flow"`
	StartOffset   int         `yaml:"start,flow"`
	EndOffset     int         `yaml:"end,flow"`
	PropLength    int         `yaml:"-"`
	ContentLength int         `yaml:"-"`
	PropertyData  []byte      `yaml:"-"`
	Properties    *Properties `yaml:"props,flow,omitempty"`
	Nodes         []*Node     `yaml:"nodes,omitempty"`
}

func NewRevision(r *DumpReader) (rev *Revision, err error) {
	rev = &Revision{StartOffset: r.Offset()}
	//g: Revision <- RevisionHeader Node*
	//g: RevisionHeader <- RevisionNumber Newline PropContentLength Newline ContentLength Newline Newline
	//g: RevisionNumber <- Revision-number: <digits>
	//g: PropContentLength <- Prop-content-length: <digits>
	//g: ContentLength <- Content-length: <digits>
	if rev.Number, err = r.IntAfter(RevisionNumberHeader); err != nil {
		return nil, err
	}
	Log("revision: %d", rev.Number)
	if rev.PropLength, err = r.IntAfter(PropContentLengthHeader); err != nil {
		return nil, err
	}
	if rev.ContentLength, err = r.IntAfter(ContentLengthHeader); err != nil {
		return nil, err
	}
	if !r.Newline() {
		return nil, fmt.Errorf("r%d: missing newline after revision header", rev.Number)
	}

	// Look at the property later.
	rev.PropertyData, err = r.Read(rev.PropLength)
	if err != nil {
		return nil, err
	}
	if !r.Newline() {
		return nil, fmt.Errorf("r%d: missing newline after properties", rev.Number)
	}
	r.Newline()

	for {
		if ahead := r.Peek(len(NodePathHeader)); string(ahead) != NodePathHeader {
			break
		}
		node, err := NewNode(rev, r)
		if err != nil {
			return nil, fmt.Errorf("r%d: %w", rev.Number, err)
		}
		if node == nil {
			break
		}
		rev.Nodes = append(rev.Nodes, node)

		for r.Newline() {
		}
	}

	rev.EndOffset = r.Offset()

	return rev, nil
}

func (rev *Revision) FindNode(predicate func(*Node) bool) *Node {
	for _, node := range rev.Nodes {
		if predicate(node) {
			return node
		}
	}
	return nil
}

// GetNodeIndexesWithPrefix returns a list of node indexes that match the given path-
// component prefix (distinguishing Model/ from Models/)
func (rev *Revision) GetNodeIndexesWithPrefix(prefix string) []int {
	nodes := make([]int, 0)
	for idx, node := range rev.Nodes {
		if MatchPathPrefix(node.Path, prefix) {
			nodes = append(nodes, idx)
		}
	}
	return nodes
}

func (rev *Revision) Encode(w io.Writer) error {
	//g: Revision <- RevisionHeader Node*
	//g: RevisionHeader <- RevisionNumber Newline PropContentLength Newline ContentLength Newline Newline
	//g: RevisionNumber <- Revision-number: <digits>
	//g: PropContentLength <- Prop-content-length: <digits>
	//g: ContentLength <- Content-length: <digits>

	// Get the property packet so we can determine the size
	properties := rev.Properties.Bytes()

	headers := []struct {
		key string
		val int
	} {
		{ key: RevisionNumberHeader, val: rev.Number },
		{ key: PropContentLengthHeader, val: len(properties) },
		{ key: ContentLengthHeader, val: len(properties) },
	}
	for _, header := range headers {
		if _, err := fmt.Fprintf(w, "%s: %d\n", header.key, header.val); err != nil {
			return fmt.Errorf("r%d: rev-hdrs: %w", rev.Number, err)
		}
	}
	if _, err := w.Write([]byte{ '\n' }); err != nil {
		return fmt.Errorf("r%d: %w", rev.Number, err)
	}

	// Append revision properties.
	if _, err := w.Write(properties); err != nil {
		return fmt.Errorf("r%d: rev-props: %w", rev.Number, err)
	}
	if _, err := w.Write([]byte{ '\n' }); err != nil {
		return fmt.Errorf("r%d: tail: %w", rev.Number, err)
	}

	for _, node := range rev.Nodes {
		if err := node.Encode(w); err != nil {
			return fmt.Errorf("r%d: node: %w", rev.Number, err)
		}
	}

	return nil
}
