package svn

import (
	"fmt"
)

type Revision struct {
	Number            int64
	PropContentLength int64
	ContentLength     int64
	PropertyBytes     []byte
	Properties        Properties
	Nodes             []*Node
}

func NewRevision(source []byte) (rev *Revision, remainder []byte, err error) {
	//g: Revision <- RevisionHeader Node*
	//g: RevisionHeader <- RevisionNumber Newline PropContentLength Newline ContentLength Newline Newline
	//g: RevisionNumber <- Revision-number: <digits>
	//g: PropContentLength <- Prop-content-length: <digits>
	//g: ContentLength <- Content-length: <digits>
	headers, source, err := ReadItems(source,
		HeaderLine{Label: "Revision-number", Optional: false, Paragraph: false},
		HeaderLine{Label: "Prop-content-length", Optional: false, Paragraph: false},
		HeaderLine{Label: "Content-length", Optional: false, Paragraph: true})

	if err != nil {
		return nil, source, fmt.Errorf("invalid revision header: %w", err)
	}

	var number, pcl, cl int64
	if number, err = headers["Revision-number"].Int64(); err != nil {
		return nil, source, fmt.Errorf("invalid revision number: %w", err)
	}
	if pcl, err = headers["Prop-content-length"].Int64(); err != nil {
		return nil, source, fmt.Errorf("r%d: invalid property content length: %w", number, err)
	}
	if cl, err = headers["Content-length"].Int64(); err != nil {
		return nil, source, fmt.Errorf("r%d: invalid content length: %w", number, err)
	}
	if pcl != cl {
		panic(fmt.Sprintf("r%d: pcl (%d) != cl (%d)", number, pcl, cl))
	}
	if int64(len(source)) < pcl {
		panic(fmt.Sprintf("r%d: premature end of file (expected %d bytes, %d remaining)", number, pcl, len(source)))
	}

	properties := NewProperties()
	var crs int64
	remainder, crs, err = properties.Read(source)
	if err != nil {
		return nil, source, fmt.Errorf("invalid property data: %w", err)
	}
	if int64(len(remainder)) != int64(len(source))-(cl+crs) {
		panic("wrong property data size")
	}

	// Blank line after the header block.
	remainder, _ = SkipNewline(remainder)

	rev = &Revision{
		Number:            number,
		PropContentLength: pcl,
		ContentLength:     cl,
		Properties:        properties,
	}

	for len(remainder) > 0 {
		var node *Node
		node, remainder, err = NewNode(remainder)
		if err != nil {
			return nil, remainder, err
		}
		if node == nil {
			break
		}
		rev.Nodes = append(rev.Nodes, node)
	}
	remainder, _ = SkipNewline(remainder)

	return rev, remainder, nil
}
