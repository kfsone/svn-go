package svn

const (
	Newline                 = "\n"
	UUIDHeader              = "UUID"
	VersionStringHeader     = "SVN-fs-dump-format-version"
	RevisionNumberHeader    = "Revision-number"
	PropContentLengthHeader = "Prop-content-length"
	ContentLengthHeader     = "Content-length"
)

var PropsEnd = []byte("PROPS-END")
