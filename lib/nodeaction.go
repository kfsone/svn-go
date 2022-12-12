package svn

import "fmt"

// NodeAction represents what action is being applied to a
// node, i.e a change, addition, replacement or deletion.
type NodeAction *string

func NewNodeAction(act string) NodeAction {
	str := string(act)
	return &str
}

var (
	NodeActionChange  = NewNodeAction("chg")
	NodeActionAdd     = NewNodeAction("add")
	NodeActionDelete  = NewNodeAction("del")
	NodeActionReplace = NewNodeAction("rep")
)

var NodeActions = map[string]NodeAction{
	"change":  NodeActionChange,
	"add":     NodeActionAdd,
	"delete":  NodeActionDelete,
	"replace": NodeActionReplace,
}

func GetNodeAction(act string) (NodeAction, error) {
	if result, ok := NodeActions[act]; ok {
		return result, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownNodeAction, act)
}
