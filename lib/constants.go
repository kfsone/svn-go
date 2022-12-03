package svn

import "errors"

const (
	Newline                 = "\n"
	VersionStringHeader     = "SVN-fs-dump-format-version"
	UUIDHeader              = "UUID"
	RevisionNumberHeader    = "Revision-number"
	PropContentLengthHeader = "Prop-content-length"
	ContentLengthHeader     = "Content-length"
	PropsEnd                = "PROPS-END"
)

var ErrMissingField = errors.New("missing required field")
var ErrMissingNewline = errors.New("missing newline")
