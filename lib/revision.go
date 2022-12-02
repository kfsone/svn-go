package svn

import (
	"fmt"
)

type Revision struct {
	Number        int
	ContentLength int
	Properties    *Properties
	Nodes         []*Node
}

func NewRevision(r *DumpReader) (rev *Revision, err error) {
	//g: Revision <- RevisionHeader Node*
	//g: RevisionHeader <- RevisionNumber Newline PropContentLength Newline ContentLength Newline Newline
	//g: RevisionNumber <- Revision-number: <digits>
	//g: PropContentLength <- Prop-content-length: <digits>
	//g: ContentLength <- Content-length: <digits>
	headers, err := r.ReadItems(
		HeaderLine{Label: "Revision-number", Optional: false},
		HeaderLine{Label: "Prop-content-length", Optional: false},
		HeaderLine{Label: "Content-length", Optional: false})

	if err != nil {
		return nil, fmt.Errorf("invalid revision header: %w", err)
	}

	var number, cl int
	if number, err = headers.Int("Revision-number"); err != nil {
		return nil, fmt.Errorf("invalid revision number: %w", err)
	}
	info("revision: %d", number)

	if cl, err = headers.Int("Content-length"); err != nil {
		return nil, fmt.Errorf("r%d: invalid content length: %w", number, err)
	}

	properties, err := NewProperties(r, cl)
	if err != nil {
		return nil, fmt.Errorf("r%d: invalid property data: %w", number, err)
	}
	if !r.Newline() {
		return nil, fmt.Errorf("r%d: missing newline after properties", number)
	}

	rev = &Revision{
		Number:        number,
		ContentLength: cl,
		Properties:    properties,
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
