package svn

// constants.go has "constant" fixed values and types that define them.

import (
	"errors"
	"fmt"
)

// Strings used in the svn headers.
const (
	VersionStringHeader     = "SVN-fs-dump-format-version"
	UUIDHeader              = "UUID"
	RevisionNumberHeader    = "Revision-number"
	PropContentLengthHeader = "Prop-content-length"
	TextContentLengthHeader = "Text-content-length"
	ContentLengthHeader     = "Content-length"

	NodePathHeader   = "Node-path"
	NodeKindHeader   = "Node-kind"
	NodeActionHeader = "Node-action"

	NodeCopyfromRevHeader  = "Node-copyfrom-rev"
	NodeCopyfromPathHeader = "Node-copyfrom-path"

	PropsEnd = "PROPS-END"
)

// Error types.
var ErrDumpHeaderMismatch = errors.New("dump header mismatch")
var ErrInvalidDumpFile = errors.New("invalid svn dump file")
var ErrInvalidHeader = errors.New("invalid header")
var ErrMissingField = errors.New("missing required field")
var ErrMissingNewline = errors.New("missing newline")
var ErrWindowsDumpFile = fmt.Errorf("%w: windows line-ending translations detected, on windows use `svnadmin dump -F filename` rather than redirecting output", ErrInvalidDumpFile)
var ErrUnknownNodeKind = errors.New("unknown node kind")
var ErrUnknownNodeAction = errors.New("unknown node action")
