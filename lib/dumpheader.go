package svn

import (
	"fmt"
	"io"
)

// DumpHeader represents the premable of the dump, which denotes the dump format number
// and the UUID of the repository.
type DumpHeader struct {
	Format    int
	ReposUUID string
}

// NewDumpHeader attempts to parse preamble from a dump file and returns a DumpHeader
// if the premable is valid.
func NewDumpHeader(r *DumpReader) (h *DumpHeader, err error) {
	h = &DumpHeader{}

	//g: FormatHeader  <- FormatVersion Newline [UUID Newline]? Newline
	//g: FormatVersion <- SVN-fs-dump-format-version: <digits>
	if h.Format, err = r.IntAfter(VersionStringHeader); err != nil {
		return nil, fmt.Errorf("missing/invalid %s header, not an svn dump file? %w", VersionStringHeader, err)
	}
	if !r.Newline() {
		return nil, fmt.Errorf("missing newline after %s header", VersionStringHeader)
	}

	//g: UUID          <- UUID: <uuid>
	if h.Format >= 2 {
		if uuid, ok := r.LineAfter(UUIDHeader + ": "); ok {
			h.ReposUUID = uuid
			if !r.Newline() {
				return nil, fmt.Errorf("missing newline after %s header", UUIDHeader)
			}
		}
	}

	return h, nil
}

func (h *DumpHeader) Encode(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "%s: %d\n\n", VersionStringHeader, h.Format); err != nil {
		return err
	}
	if h.Format >= 2 {
		if _, err := fmt.Fprintf(w, "%s: %s\n\n", UUIDHeader, h.ReposUUID); err != nil {
			return err
		}
	}

	return nil
}
