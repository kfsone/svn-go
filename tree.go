package main

import (
	"fmt"
	svn "github.com/kfsone/svn-go/lib"
	"strings"
)

type TreeNode struct {
	Path      string
	Parent    *TreeNode
	Revisions []*svn.Node
	Explicit  map[int]*svn.Node
	Children  map[string]*TreeNode
}

func NewTreeNode(path string, node *svn.Node, parent *TreeNode) *TreeNode {
	revs := append(make([]*svn.Node, 0, 1), node)
	return &TreeNode{
		Path:      path,
		Parent:    parent,
		Revisions: revs,
		Explicit:  make(map[int]*svn.Node),
		Children:  make(map[string]*TreeNode),
	}
}

func (t *TreeNode) AddRevision(node *svn.Node, explicit bool) {
	if back := t.Last(); back != nil && back.Revision.Number == node.Revision.Number {
		t.Revisions[len(t.Revisions)-1] = node
	} else {
		t.Revisions = append(t.Revisions, node)
	}
	if explicit {
		t.Explicit[node.Revision.Number] = node
	}
}

func (t *TreeNode) At(revision int) *svn.Node {
	if node, present := t.Explicit[revision]; present {
		return node
	}
	var last *svn.Node = nil
	for _, node := range t.Revisions {
		if node.Revision.Number < revision {
			last = node
			continue
		}
		if node.Revision.Number == revision {
			return node
		}
		break
	}
	return last
}

func (t *TreeNode) Last() *svn.Node {
	if len(t.Revisions) == 0 {
		return nil
	}
	return t.Revisions[len(t.Revisions)-1]
}

func (t *TreeNode) First() *svn.Node {
	if len(t.Revisions) == 0 {
		return nil
	}
	return t.Revisions[0]
}

func (t *TreeNode) Walk() <-chan *TreeNode {
	ch := make(chan *TreeNode)
	go func() {
		defer close(ch)
		ch <- t
		for _, child := range t.Children {
			for subChild := range child.Walk() {
				ch <- subChild
			}
		}
	}()
	return ch
}

type Tree struct {
	Root *TreeNode
}

func NewTree() *Tree {
	return &Tree{
		Root: NewTreeNode("", nil, nil),
	}
}

func splitPath(path string) []int {
	start := 0
	parts := make([]int, 0, len(path)/4)
	for {
		end := strings.Index(path[start:], "/")
		if end < 0 {
			break
		}
		if end > 0 {
			parts = append(parts, start+end)
		}
		// Skip the slash
		start += end + 1
	}
	parts = append(parts, len(path))
	return parts
}

func (t *Tree) Lookup(path string) (treeNode *TreeNode, match bool) {
	treeNode = t.Root
	if path == "." || path == "/" {
		path = ""
	}

	start := 0
	for _, part := range splitPath(path) {
		piece := path[start:part]
		child, ok := treeNode.Children[piece]
		if !ok {
			return treeNode, false
		}
		treeNode = child
		start = part + 1
	}

	return treeNode, true
}

func audit(format string, args ...interface{}) {
	//fmt.Printf(format+"\n", args...)
}

func (t *Tree) insertBelow(treeNode *TreeNode, path string, node *svn.Node) error {
	start := 0
	for _, partEnds := range splitPath(path) {
		piece := path[start:partEnds]
		child, ok := treeNode.Children[piece]
		if !ok {
			if partEnds != len(path) {
				return fmt.Errorf("missing intermediate node for %s: %s", path, piece)
			}
			fullPath := treeNode.Path + "/" + piece
			if node.Action == svn.NodeActionDelete {
				return fmt.Errorf("deleting non-existent node %s", fullPath)
			}
			child = NewTreeNode(fullPath, node, treeNode)
			treeNode.Children[piece] = child
			treeNode = child
			break
		}

		treeNode = child
		start = partEnds + 1
	}

	branchRev, branchPath, branched := node.Branched()

	treeNode.AddRevision(node, true)
	if branched {
		branchNode, ok := t.Lookup(branchPath)
		if !ok {
			return fmt.Errorf("missing branch node %s", branchPath)
		}
		return t.copyBranch(treeNode, branchRev, branchNode, node)
	}
	if node.Action == svn.NodeActionDelete {
		for _, subChild := range treeNode.Children {
			subChild.AddRevision(node, false)
		}
	}
	return nil
}

func (t *Tree) copyBranch(treeNode *TreeNode, branchRev int, branchNode *TreeNode, node *svn.Node) error {
	branchRevNode := branchNode.At(branchRev)
	if branchRevNode == nil || branchRevNode.Action == svn.NodeActionDelete {
		// It wasn't there at this revision.
		return nil
	}

	audit("r%d branching %s@r%d to %s", node.Revision.Number, branchNode.Path, branchRev, node.Path())

	for branchPart, branchChild := range branchNode.Children {
		// ignore deleted items.
		if childRevNode := branchChild.At(branchRev); childRevNode == nil || childRevNode.Action == svn.NodeActionDelete {
			continue
		}
		treeChild, present := treeNode.Children[branchPart]
		if !present {
			destPath := treeNode.Path + "/" + branchPart
			treeChild = NewTreeNode(destPath, branchRevNode, treeNode)
			treeNode.Children[branchPart] = treeChild
		}
		treeChild.AddRevision(node, false)
		if len(branchChild.Children) > 0 {
			if err := t.copyBranch(treeChild, branchRev, branchChild, node); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Tree) Insert(node *svn.Node) error {
	path := node.Path()
	if path == "." || path == "/" {
		path = ""
	}

	return t.insertBelow(t.Root, path, node)
}
