package svn

import (
	"errors"
	"fmt"
)

type NodeKind int

const (
	NodeKindFile NodeKind = iota
	NodeKindDir           = iota
)

func (k NodeKind) String() string {
	switch k {
	case NodeKindFile:
		return "file"
	case NodeKindDir:
		return "dir"
	}
	return "unknown"
}

type NodeAction int

const (
	NodeActionChange  NodeAction = iota
	NodeActionAdd                = iota
	NodeActionDelete             = iota
	NodeActionReplace            = iota
)

func (a NodeAction) String() string {
	switch a {
	case NodeActionChange:
		return "chg"
	case NodeActionAdd:
		return "add"
	case NodeActionDelete:
		return "del"
	case NodeActionReplace:
		return "rep"
	}
	return "unk"
}

type FileHistory struct {
	Rev  int
	Path string
}

type Node struct {
	Path   string
	Kind   NodeKind
	Action NodeAction

	History *FileHistory

	PropertiesLength int
	TextLength       int
	ContentLength    int
	SourceMd5        string
	SourceSha1       string
	ContentMd5       string
	ContentSha1      string
	Properties       *Properties
}

func NewNode(r *DumpReader) (node *Node, err error) {
	node = &Node{}

	var ok bool
	if node.Path, ok = r.LineAfter("Node-path: "); !ok {
		return nil, nil
	}

	var nodeKind string
	if nodeKind, ok = r.LineAfter("Node-kind: "); ok {
		switch nodeKind {
		case "file":
			node.Kind = NodeKindFile
		case "dir":
			node.Kind = NodeKindDir
		default:
			return nil, fmt.Errorf("%s: invalid Node-kind: %s", node.Path, nodeKind)
		}
	}

	nodeAction, ok := r.LineAfter("Node-action: ")
	if !ok {
		return nil, fmt.Errorf("%s: missing Node-action", node.Path)
	}
	switch nodeAction {
	case "change":
		node.Action = NodeActionChange
	case "add":
		node.Action = NodeActionAdd
	case "delete":
		node.Action = NodeActionDelete
	case "replace":
		node.Action = NodeActionReplace
	default:
		return nil, fmt.Errorf("%s: invalid Node-action: %s", node.Path, nodeAction)
	}
	if node.Action != NodeActionDelete && nodeKind == "" {
		return nil, fmt.Errorf("%s: missing Node-kind", node.Path)
	}

	log("| %s:%4s:%s", node.Action, node.Kind, node.Path)

	label := node.Path
	if nodeKind != "" {
		label += ":" + nodeKind
	}
	label += ":" + nodeAction

	var history FileHistory
	if history.Rev, err = r.IntAfter("Node-copyfrom-rev"); err == nil {
		if history.Path, ok = r.LineAfter("Node-copyfrom-path: "); !ok {
			return nil, fmt.Errorf("%s: missing Node-copyfrom-path", label)
		}
		node.History = &FileHistory{Rev: history.Rev, Path: history.Path}
	}

	if node.Action == NodeActionDelete {
		if !r.Newline() {
			return nil, fmt.Errorf("missing newline after Node-action: delete")
		}
		return node, nil
	}

	node.SourceMd5, _ = r.LineAfter("Text-copy-source-md5: ")
	node.SourceSha1, _ = r.LineAfter("Text-copy-source-sha1: ")
	node.ContentMd5, _ = r.LineAfter("Text-content-md5: ")
	node.ContentSha1, _ = r.LineAfter("Text-content-sha1: ")

	if node.PropertiesLength, err = r.IntAfter("Prop-content-length"); err != nil && !errors.Is(err, ErrMissingField) {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if node.TextLength, err = r.IntAfter("Text-content-length"); err != nil && !errors.Is(err, ErrMissingField) {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if node.ContentLength, err = r.IntAfter("Content-length"); err != nil && !errors.Is(err, ErrMissingField) {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if !r.Newline() {
		return nil, fmt.Errorf("%s: missing newline after node headers", label)
	}

	if node.PropertiesLength == 0 && node.TextLength == 0 {
		return node, nil
	}

	if node.PropertiesLength > 0 {
		// Load any property values
		if node.Properties, err = NewProperties(r, node.PropertiesLength); err != nil {
			return nil, fmt.Errorf("%s: properties: %w", label, err)
		}
	}

	// Skip the content data.
	r.Discard(node.ContentLength - node.PropertiesLength)

	if !r.Newline() || !r.Newline() {
		return nil, fmt.Errorf("%s: missing newline after properties slot", label)
	}

	return node, nil
}
