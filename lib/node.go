package svn

// Svn dumps refer to per-revision entries for a file/directory as a "node".

import (
	"errors"
	"fmt"
)

// FileHistory represents an ancestor revision/path in, for example, a copy
// action.
type FileHistory struct {
	Rev  int    `yaml:"rev"`
	Path string `yaml:"path"`
}

// Node represents some action against a file or directory as part of a
// revision.
type Node struct {
	Revision       *Revision    `yaml:"-"`
	Path           string       `yaml:"path,flow"`
	Kind           NodeKind     `yaml:"kind,flow"`
	Action         NodeAction   `yaml:"action,flow"`
	History        *FileHistory `yaml:"history,flow,omitempty"`
	PropertyData   []byte       `yaml:"-"`
	PropertyLength int          `yaml:"-"`
	TextLength     int          `yaml:"-"`
	ContentLength  int          `yaml:"-"`
	Properties     *Properties  `yaml:"props,flow,omitempty"`

	modified bool
	removed bool
}

// NewNode tries to parse a node from the dump reader and return a Node
// representing it, otherwise an error is returned. If the buffer is
// prefixed with anything other than a "Node-path" header, we have left
// the node section of the dump, and nil, nil is returned.
func NewNode(rev *Revision, r *DumpReader) (node *Node, err error) {
	node = &Node{Revision: rev}

	var ok bool
	if node.Path, ok = r.LineAfter(NodePathPrefix); !ok {
		return nil, nil
	}

	var nodeKind string
	if nodeKind, ok = r.LineAfter(NodeKindPrefix); ok {
		switch nodeKind {
		case "file":
			node.Kind = NodeKindFile
		case "dir":
			node.Kind = NodeKindDir
		default:
			return nil, fmt.Errorf("%s: invalid %s%s", node.Path, NodeKindPrefix, nodeKind)
		}
	}

	nodeAction, ok := r.LineAfter(NodeActionPrefix)
	if !ok {
		return nil, fmt.Errorf("%s: missing %sgot %s", node.Path, NodeActionPrefix, r.Peek(30))
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
		return nil, fmt.Errorf("%s: invalid %s%s", node.Path, NodeActionPrefix, nodeAction)
	}
	if node.Action != NodeActionDelete && nodeKind == "" {
		return nil, fmt.Errorf("%s: missing Node-kind", node.Path)
	}

	log("| %s:%4s:%s", *node.Action, nodeKind, node.Path)

	label := node.Path
	if nodeKind != "" {
		label += ":" + nodeKind
	}
	label += ":" + nodeAction

	var history FileHistory
	if history.Rev, err = r.IntAfter(NodeCopyfromRevHeader); err == nil {
		if history.Path, ok = r.LineAfter(NodeCopyfromPathPrefix); !ok {
			return nil, fmt.Errorf("%s: missing %sgot %s", label, NodeCopyfromPathPrefix, r.Peek(30))
		}
		node.History = &FileHistory{Rev: history.Rev, Path: history.Path}
		r.LineAfter(TextCopySourceMd5Prefix)
		r.LineAfter(TextCopySourceSha1Prefix)
	}

	if node.Action == NodeActionDelete {
		if !r.Newline() {
			return nil, fmt.Errorf("%s: missing newline after %sdelete", NodeActionPrefix, label)
		}
		return node, nil
	}

	r.LineAfter(PropDeltaPrefix)
	if _, ok = r.LineAfter(TextDeltaPrefix); ok {
		r.LineAfter(TextDeltaBaseMd5Prefix)
		r.LineAfter(TextDeltaBaseSha1Prefix)
	}
	r.LineAfter(TextContentMd5Prefix)
	r.LineAfter(TextContentSha1Prefix)

	if node.PropertyLength, err = r.IntAfter(PropContentLengthHeader); err != nil && !errors.Is(err, ErrMissingField) {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if node.TextLength, err = r.IntAfter(TextContentLengthHeader); err != nil && !errors.Is(err, ErrMissingField) {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if node.ContentLength, err = r.IntAfter(ContentLengthHeader); err != nil && !errors.Is(err, ErrMissingField) {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if !r.Newline() {
		return nil, fmt.Errorf("%s: missing newline after node headers; got: %s", label, r.Peek(60))
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
	r.Discard(node.ContentLength - node.PropertyLength)

	if !r.Newline() || !r.Newline() {
		return nil, fmt.Errorf("%s: missing newline after properties slot", label)
	}

	return node, nil
}
