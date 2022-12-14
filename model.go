package main

//import (
//	"fmt"
//	svn "github.com/kfsone/svn-go/lib"
//	"strings"
//)
//
//type ModelNode struct {
//	*svn.Node
//	References []*ModelNode
//	path       string
//}
//
//func NewModelNode(node *svn.Node) *ModelNode {
//	return &ModelNode{
//		Node:       node,
//		References: make([]*ModelNode, 0),
//	}
//}
//
//func (n *ModelNode) Path() string {
//	if n.path != "" {
//		return n.path
//	}
//	return n.Node.Path()
//}
//
//type RevisionModel struct {
//	*svn.Revision
//	Session   *Session
//	Table     map[string]*ModelNode
//	Additions []*ModelNode
//}
//
//func NewRevisionModel(session *Session, rev *svn.Revision, model *RevisionModel) *RevisionModel {
//	newModel := &RevisionModel{
//		Revision:  rev,
//		Session:   session,
//		Table:     nil,
//		Additions: make([]*ModelNode, 0),
//	}
//
//	if model != nil {
//		newModel.Table = make(map[string]*ModelNode, len(model.Table))
//		for k, v := range model.Table {
//			newModel.Table[k] = v
//		}
//	} else {
//		newModel.Table = make(map[string]*ModelNode, 4096)
//	}
//
//	return newModel
//}
//
//func (session *Session) Model() (err error) {
//	model := NewRevisionModel(session, nil, nil)
//
//	for _, rev := range session.Revisions {
//		if rev.Number%100 == 0 {
//			fmt.Printf("\rr%09d (%5.2f%%)\r", rev.Number, float64(rev.Number)/float64(len(session.Revisions)+1)*100)
//		}
//
//		for _, node := range rev.Nodes {
//			switch node.Action {
//			case svn.NodeActionAdd:
//				err = model.addNode(node)
//
//			case svn.NodeActionChange:
//				err = model.changeNode(node)
//
//			case svn.NodeActionReplace:
//				err = model.replaceNode(node)
//
//			case svn.NodeActionDelete:
//				err = model.deleteNode(node)
//			}
//			if err != nil {
//				return fmt.Errorf("r%d: %s %s %s: %w", rev.Number, *node.Action, *node.Kind, node.Path(), err)
//			}
//		}
//
//		session.RevisionModels = append(session.RevisionModels, NewRevisionModel(session, rev, model))
//	}
//
//	fmt.Printf("\rr%9d (100.00%%)\n", len(session.Revisions))
//
//	return nil
//}
//
//func (m *RevisionModel) addNode(node *svn.Node) error {
//	path := node.Path()
//	newNode := NewModelNode(node)
//	if previous, present := m.Table[path]; present {
//		previous.References = append(previous.References, newNode)
//	}
//
//	m.Table[path] = newNode
//	m.Additions = append(m.Additions, newNode)
//
//	if branchRev, branchPath, branched := node.Branched(); branched {
//		if node.Kind == svn.NodeKindDir {
//			if err := m.copyChildren(m.Session.RevisionModels[branchRev], branchPath, newNode); err != nil {
//				return err
//			}
//		}
//		branchNode := m.getBranchNode(branchRev, branchPath)
//		if branchNode == nil {
//			panic("missing branch node")
//		}
//		branchNode.References = append(branchNode.References, newNode)
//	}
//
//	return nil
//}
//
//func (m *RevisionModel) changeNode(node *svn.Node) error {
//	path := node.Path()
//	branchRev, branchPath, _ := node.Branched()
//	branchNode := m.getBranchNode(branchRev, branchPath)
//
//	if _, present := m.Table[path]; present {
//		return nil
//	}
//
//	if branchNode != nil {
//		return m.addNode(node)
//	}
//
//	return fmt.Errorf("node does not exist")
//}
//
//func (m *RevisionModel) replaceNode(node *svn.Node) error {
//	path := node.Path()
//	previous, present := m.Table[path]
//	if !present {
//		return fmt.Errorf("replacee does not exist")
//	}
//
//	if _, _, branched := node.Branched(); branched {
//		return fmt.Errorf("wasn't expecting a replace branch")
//	}
//
//	newNode := NewModelNode(node)
//	if previous != nil {
//		previous.References = append(previous.References, newNode)
//	}
//
//	m.Table[path] = newNode
//	if newNode.Kind == svn.NodeKindDir {
//		return m.copyChildren(m, path, newNode)
//	}
//
//	return nil
//}
//
//func (m *RevisionModel) deleteNode(node *svn.Node) error {
//	path := node.Path()
//	if oldNode, present := m.Table[path]; present {
//		delete(m.Table, path)
//
//		if oldNode.Kind == svn.NodeKindDir {
//			for childPath, _ := range m.Table {
//				if svn.MatchPathPrefix(childPath, path) {
//					delete(m.Table, childPath)
//				}
//			}
//		}
//
//		return nil
//	}
//
//	return fmt.Errorf("node not found")
//}
//
//func (m *RevisionModel) getBranchNode(rev int, path string) *ModelNode {
//	branchRev := m.Session.RevisionModels[rev]
//	return branchRev.Table[path]
//}
//
//func (m *RevisionModel) copyChildren(src *RevisionModel, path string, newParent *ModelNode) error {
//	needlePath := path + "/"
//	parentPath := newParent.Path()
//
//	for oldPath, oldNode := range src.Table {
//		if !strings.HasPrefix(oldPath, needlePath) {
//			continue
//		}
//		newPath := parentPath + oldPath[len(path):]
//
//		newNode := NewModelNode(oldNode.Node)
//		newNode.path = newPath
//		newNode.References = append(newNode.References, newParent)
//		oldNode.References = append(oldNode.References, newNode)
//
//		m.Table[newPath] = newNode
//		m.Additions = append(m.Additions, newNode)
//	}
//
//	return nil
//}
//func constrainRetrofit(session *Session, path string) (int, int, error) {
//	// Find the revision where the path is introduced.
//	var created *RevisionModel = nil
//	for _, rev := range session.RevisionModels {
//		if _, present := rev.Table[path]; present {
//			created = rev
//			break
//		}
//	}
//	if created == nil {
//		return 0, 0, fmt.Errorf("no creation found for %s", path)
//	}
//
//	return created.Number, 0, nil
//}
