package svn

import (
	"fmt"
)

type Revision struct {
	Number        int
	StartOffset   int
	EndOffset     int
	PropLength    int
	ContentLength int
	PropertyData  []byte
	Properties    *Properties
	Nodes         []*Node
}

func NewRevision(r *DumpReader) (rev *Revision, err error) {
	rev = &Revision{StartOffset: r.Offset()}
	defer func() {
		rev.EndOffset = r.Offset()
	}()
	//g: Revision <- RevisionHeader Node*
	//g: RevisionHeader <- RevisionNumber Newline PropContentLength Newline ContentLength Newline Newline
	//g: RevisionNumber <- Revision-number: <digits>
	//g: PropContentLength <- Prop-content-length: <digits>
	//g: ContentLength <- Content-length: <digits>
	if rev.Number, err = r.IntAfter("Revision-number"); err != nil {
		return nil, err
	}
	log("revision: %d", rev.Number)
	if rev.PropLength, err = r.IntAfter("Prop-content-length"); err != nil {
		return nil, err
	}
	if rev.ContentLength, err = r.IntAfter("Content-length"); err != nil {
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

	for !r.Empty() {
		if r.Newline() {
			continue
		}
		node, err := NewNode(r)
		if err != nil {
			return nil, err
		}
		if node == nil {
			break
		}
		rev.Nodes = append(rev.Nodes, node)
	}

	return rev, nil
}
