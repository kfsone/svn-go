package svn

import (
	"fmt"
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

	for !r.AtEOF() {
		if r.Newline() {
			continue
		}
		node, err := NewNode(rev, r)
		if err != nil {
			return nil, err
		}
		if node == nil {
			break
		}
		rev.Nodes = append(rev.Nodes, node)
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
