package svn

import (
	"fmt"
	"strconv"
)

type DumpHeader struct {
	Format    int
	ReposUUID string
}

func NewDumpHeader(r *DumpReader) (h DumpHeader, err error) {
	//g: FormatHeader  <- FormatVersion Newline [UUID Newline]? Newline

	//g: FormatVersion <- SVN-fs-dump-format-version: <digits>
	verStr, ok := r.LineAfter(VersionStringHeader + ": ")
	if !ok {
		return h, fmt.Errorf("missing %s header, not an svn dump file?", VersionStringHeader)
	}
	h.Format, err = strconv.Atoi(verStr)
	if err != nil {
		return h, fmt.Errorf("invalid %s header: %w", VersionStringHeader, err)
	}
	if !r.Newline() {
		return h, fmt.Errorf("missing newline after %s header", VersionStringHeader)
	}

	//g: UUID          <- UUID: <uuid>
	if h.Format >= 2 {
		if uuid, ok := r.LineAfter(UUIDHeader + ": "); ok {
			h.ReposUUID = uuid
			if !r.Newline() {
				return h, fmt.Errorf("missing newline after %s header", UUIDHeader)
			}
		}
	}

	return h, nil
}
