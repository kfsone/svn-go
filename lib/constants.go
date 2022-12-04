// constants.go has "constant" fixed values and types that define them.
package svn

import "errors"

// Strings used in the svn headers.
const (
	VersionStringHeader     = "SVN-fs-dump-format-version"
	UUIDHeader              = "UUID"
	RevisionNumberHeader    = "Revision-number"
	PropContentLengthHeader = "Prop-content-length"
	TextContentLengthHeader = "Text-content-length"
	ContentLengthHeader     = "Content-length"

	NodePathPrefix   = "Node-path: "
	NodeKindPrefix   = "Node-kind: "
	NodeActionPrefix = "Node-action: "

	NodeCopyfromRevHeader    = "Node-copyfrom-rev"
	NodeCopyfromPathPrefix   = "Node-copyfrom-path: "
	TextCopySourceMd5Prefix  = "Text-copy-source-md5: "
	TextCopySourceSha1Prefix = "Text-copy-source-sha1: "

	PropDeltaPrefix         = "Prop-delta: "
	TextDeltaPrefix         = "Text-delta: "
	TextDeltaBaseMd5Prefix  = "Text-delta-base-md5: "
	TextDeltaBaseSha1Prefix = "Text-delta-base-sha1: "
	TextContentMd5Prefix    = "Text-content-md5: "
	TextContentSha1Prefix   = "Text-content-sha1: "

	PropsEnd = "PROPS-END"
)

// Error types.
var ErrMissingField = errors.New("missing required field")
var ErrMissingNewline = errors.New("missing newline")

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
