package svn

import (
	"fmt"
	"strconv"
)

type NodeKind int

const (
	NodeKindFile NodeKind = iota
	NodeKindDir           = iota
)

type NodeAction int

const (
	NodeActionChange  NodeAction = iota
	NodeActionAdd                = iota
	NodeActionDelete             = iota
	NodeActionReplace            = iota
)

type Node struct {
	Path   string
	Kind   NodeKind
	Action NodeAction

	FromRev  int
	FromPath string

	PropertiesLength int
	TextLength       int
	ContentLength    int
	SourceMd5        string
	SourceSha1       string
	ContentMd5       string
	ContentSha1      string
	Properties       *Properties
}

func NewNode(r *DumpReader) (*Node, error) {
	node := &Node{}
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

	label := node.Path
	if nodeKind != "" {
		label += ":" + nodeKind
	}
	label += ":" + nodeAction

	if fromRev, ok := r.LineAfter("Node-copyfrom-rev: "); ok {
		if node.FromPath, ok = r.LineAfter("Node-copyfrom-path: "); !ok {
			return nil, fmt.Errorf("%s: missing Node-copyfrom-path", label)
		}
		var err error
		if node.FromRev, err = strconv.Atoi(fromRev); err != nil {
			return nil, fmt.Errorf("%s: invalid Node-copyfrom-rev: %s", label, fromRev)
		}
	}

	log("| %-7s:%4s:%s", nodeAction, nodeKind, node.Path)

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
	var err error
	if node.PropertiesLength, err = r.IntAfter("Prop-content-length", false); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if node.TextLength, err = r.IntAfter("Text-content-length", false); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if node.ContentLength, err = r.IntAfter("Content-length", false); err != nil {
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
	skip := node.ContentLength - node.PropertiesLength
	if _, err = r.Read(skip); err != nil {
		return nil, fmt.Errorf("%s: content: %w", label, err)
	}

	if !r.Newline() || !r.Newline() {
		return nil, fmt.Errorf("%s: missing newline after properties slot", label)
	}

	return node, nil
}
