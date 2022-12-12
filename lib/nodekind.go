package svn

import "fmt"

// NodeKind represents whether a node is a file/directory,
// but note that deletes don't have a kind.
type NodeKind *string

func NewNodeKind(kind string) NodeKind {
	str := string(kind)
	return &str
}

var (
	NodeKindFile = NewNodeKind("file")
	NodeKindDir  = NewNodeKind("dir")
)

var NodeKinds = map[string]NodeKind{
	"file": NodeKindFile,
	"dir":  NodeKindDir,
}

func GetNodeKind(kind string) (NodeKind, error) {
	if result, ok := NodeKinds[kind]; ok {
		return result, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownNodeKind, kind)
}
