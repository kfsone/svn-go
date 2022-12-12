package svn

import (
	"errors"
	"fmt"
	"io"
)

type Node struct {
	Revision   *Revision   // Back-reference to the revision we exist in.
	Headers    *Headers    // Table of headers for this node.
	Properties *Properties // Table of svn:properties attached to the node.

	Action NodeAction // Action taken on the node (add/change/delete/replace).
	Kind   NodeKind   // Kind of node (file/dir).

	data []byte // Raw binary data for the node.
}

func NewNode(rev *Revision) (nodePtr *Node, err error) {
	node := &Node{
		Revision: rev,
	}

	if node.Headers, err = NewHeaders(rev.dump); err != nil {
		return nil, err
	}

	var path string
	if path, err = node.Headers.String(NodePathHeader); err != nil {
		return nil, err
	}

	if err = checkNodeHeaders(node); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	propLen, _ := node.Headers.Int(PropContentLengthHeader)
	bodyLen, _ := node.Headers.Int(TextContentLengthHeader)

	if node.Properties, err = NewProperties(rev.dump, propLen); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if err = node.Properties.Load(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if !rev.dump.Discard(bodyLen) {
		return nil, fmt.Errorf("%s: %w", path, io.ErrUnexpectedEOF)
	}

	return node, nil
}

func checkNodeHeaders(node *Node) (err error) {
	action, err := node.Headers.String(NodeActionHeader)
	if err != nil {
		return err
	}
	if node.Action, err = GetNodeAction(action); err != nil {
		return err
	}

	kind, err := node.Headers.String(NodeKindHeader)
	// delete doesn't tell us what kind of node is being deleted.
	if err != nil && !(errors.Is(err, ErrMissingField) && node.Action == NodeActionDelete) {
		return err
	}
	if kind != "" {
		if node.Kind, err = GetNodeKind(kind); err != nil {
			return err
		}
	}

	// Check other possible fields.
	if node.Headers.Has(NodeCopyfromPathHeader) {
		if _, err := node.Headers.Int(NodeCopyfromRevHeader); err != nil {
			return err
		}
	}

	// Some other integer values we need checked.
	if _, err := node.Headers.Int(PropContentLengthHeader); err != nil {
		return fmt.Errorf("%s: %w", PropContentLengthHeader, err)
	}
	if _, err := node.Headers.Int(TextContentLengthHeader); err != nil {
		return fmt.Errorf("%s: %w", TextContentLengthHeader, err)
	}
	if _, err := node.Headers.Int(ContentLengthHeader); err != nil {
		return fmt.Errorf("%s: %w", ContentLengthHeader, err)
	}

	return nil
}

func (n *Node) Path() string {
	path, _ := n.Headers.String(NodePathHeader)
	return path
}

// Branched returns the source revision and path of the node if it was branched.
func (n *Node) Branched() (revision int, path string, ok bool) {
	var err error
	if revision, err = n.Headers.Int(NodeCopyfromRevHeader); err == nil {
		if path, err = n.Headers.String(NodeCopyfromPathHeader); err == nil {
			ok = true
		}
	}
	return
}
