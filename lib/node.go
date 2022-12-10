package svn

// Svn dumps refer to per-revision entries for a file/directory as a "node".

import (
	"fmt"
	"io"
	"strconv"
)

// FileHistory represents an ancestor revision/path in, for example, a copy
// action.
type FileHistory struct {
	Rev  int    `yaml:"rev"`
	Path string `yaml:"path"`
}

type HeaderBlock struct {
	Index			[]string
	Table			map[string]string
	Newlines		int
}

// Node represents some action against a file or directory as part of a
// revision.
type Node struct {
	Revision       *Revision    `yaml:"-"`
	HeaderBlock	   HeaderBlock	`yaml:"headers,flow,omitempty"`
	Path           string       `yaml:"path,flow"`
	Kind           NodeKind     `yaml:"kind,flow"`
	Action         NodeAction   `yaml:"action,flow"`
	History        *FileHistory `yaml:"history,flow,omitempty"`
	PropertyLength int          `yaml:"-"`
	PropertyData   []byte       `yaml:"-"`
	TextLength     int          `yaml:"-"`
	TextData	   []byte		`yaml:"-"`
	ContentLength  int          `yaml:"-"`
	Properties     *Properties  `yaml:"props,flow,omitempty"`
	Newlines	   int

	// For user tracking
	Modified bool `yaml:"-"`
	Removed  bool `yaml:"-"`
}

var nodeKinds = map[string]NodeKind{
	"file": NodeKindFile,
	"dir":  NodeKindDir,
}

var nodeActions = map[string]NodeAction{
	"change":  NodeActionChange,
	"add":     NodeActionAdd,
	"delete":  NodeActionDelete,
	"replace": NodeActionReplace,
}

func NewHeaderBlock(r *DumpReader) (block HeaderBlock, err error) {
	block = HeaderBlock{
		Index: make([]string, 0, 8),
		Table: make(map[string]string, 8),
		Newlines: 0,
	}

	for !r.Newline() {
		key, value, err := r.HeaderLine()
		if err != nil {
			return block, err
		}

		block.Index = append(block.Index, key)
		block.Table[key] = value
	}

	if len(block.Index) == 0 {
		return block, fmt.Errorf("expected header block, encountered blank line")
	}

	// Count trailing newlines; we must have already had one to break the top loop.
	for {
		block.Newlines++
		if !r.Newline() {
			break
		}
	}

	return block, nil
}


// NewNode tries to parse a node from the dump reader and return a Node
// representing it, otherwise an error is returned. If the buffer is
// prefixed with anything other than a "Node-path" header, we have left
// the node section of the dump, and nil, nil is returned.
func NewNode(rev *Revision, r *DumpReader) (node *Node, err error) {
	node = &Node{
		Revision: rev,
	}

	var present bool
	var nodeAction, nodeKind string

	node.HeaderBlock, err = NewHeaderBlock(r)
	if err != nil {
		return nil, fmt.Errorf("r%d: %w", err)
	}
	if node.Path, present = node.HeaderBlock.Table[NodePathHeader]; !present {
		return nil, fmt.Errorf("missing %s header", NodePathHeader)
	}
	if nodeAction, present = node.HeaderBlock.Table[NodeActionHeader]; !present {
		return nil, fmt.Errorf("missing %s header", NodeActionHeader)
	}
	if node.Action, present = nodeActions[nodeAction]; !present {
		return nil, fmt.Errorf("unrecognized %s: %s", NodeActionHeader, nodeAction)
	}
	isDeletion := node.Action == NodeActionDelete
	if nodeKind, present = node.HeaderBlock.Table[NodeKindHeader]; !present && !isDeletion {
		return nil, fmt.Errorf("missing %s header", NodeKindHeader)
	}

	if nodeKind != "" {
		if node.Kind, present = nodeKinds[nodeKind]; !present {
			return nil, fmt.Errorf("unrecognized %s: %s", NodeKindHeader, nodeKind)
		}
	}

	Log("| %s:%4s:%s", *node.Action, nodeKind, node.Path)

	label := node.Path
	if nodeKind != "" {
		label += ":" + nodeKind
	}
	label += ":" + nodeAction

	if fromRev, present := node.HeaderBlock.Table[NodeCopyfromRevHeader]; present {
		node.History = &FileHistory{}
		if node.History.Rev, err = strconv.Atoi(fromRev); err != nil {
			return nil, fmt.Errorf("%s: %w", NodeCopyfromRevHeader, err)
		}
		if node.History.Path, present = node.HeaderBlock.Table[NodeCopyfromPathHeader]; !present {
			return nil, fmt.Errorf("missing %s header", NodeCopyfromPathHeader)
		}
	}

	if node.Action == NodeActionDelete {
		return node, nil
	}

	// helper to get a key value as an int.
	getOptionalInt := func (header string) (value int, err error) {
		var text string
		if text, present = node.HeaderBlock.Table[header]; present {
			if value, err = strconv.Atoi(text); err != nil {
				err = fmt.Errorf("%s: %w", header, err)
			}
		}
		return value, err
	}

	if node.PropertyLength, err = getOptionalInt(PropContentLengthHeader); err != nil {
		return nil, err
	}
	if node.TextLength, err = getOptionalInt(TextContentLengthHeader); err != nil {
		return nil, err
	}
	if node.ContentLength, err = getOptionalInt(ContentLengthHeader); err != nil {
		return nil, err
	}

	if node.PropertyLength == 0 && node.TextLength == 0 {
		return node, nil
	}

	if node.PropertyLength > 0 {
		if node.PropertyData, err = r.Read(node.PropertyLength); err != nil {
			return nil, fmt.Errorf("%s: properties: %w", label, err)
		}
	}

	// Skip the content data.
	if node.PropertyLength + node.TextLength != node.ContentLength {
		panic("lengths mismatch")
	}
	if node.TextLength > 0 {
		if node.TextData, err = r.Read(node.TextLength); err != nil {
			return nil, fmt.Errorf("text read: %w", err)
		}
	}

	for r.Newline() {
		node.Newlines++
	}
	if node.Newlines < 2 {
		return nil, fmt.Errorf("%s: missing newlines after properties slot", label)
	}

	return node, nil
}

func writeNewlines(w io.Writer, lines int) error {
	for i := 0; i < lines; i++ {
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

func (b *HeaderBlock) Encode(encoder *Encoder) {
	for _, key := range b.Index {
		value, present := b.Table[key]
		if !present {
			panic("missing key")
		}
		encoder.Fprintf("%s: %s\n", key, value)
	}

	encoder.Newlines(b.Newlines)
}

func (n *Node) Encode(encoder *Encoder) {
	// We need the property block so we can recalculate the property length and
	// content length.
	properties := n.Properties.Bytes()

	propLength := len(properties)
	contLength := propLength + n.TextLength

	n.HeaderBlock.Table[PropContentLengthHeader] = fmt.Sprintf("%d", propLength)
	n.HeaderBlock.Table[ContentLengthHeader] = fmt.Sprintf("%d", contLength)

	n.HeaderBlock.Encode(encoder)

	encoder.Write(properties)

	if n.TextLength > 0 {
		encoder.Write(n.TextData)
	}

	encoder.Newlines(n.Newlines)
}
