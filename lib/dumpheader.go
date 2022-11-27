package svn

import "fmt"

type DumpHeader struct {
	Format    int64
	ReposUUID string
}

func NewDumpHeader(source []byte) (dh DumpHeader, remainder []byte, err error) {
	//g: FormatHeader  <- FormatVersion Newline [UUID Newline]? Newline
	//g: FormatVersion <- SVN-fs-dump-format-version: <digits>
	//g: UUID          <- UUID: <uuid>
	// First field should denote the dump format version.

	headers, remainder, err := ReadItems(source,
		HeaderLine{Label: "SVN-fs-dump-format-version", Optional: false, Paragraph: true},
		HeaderLine{Label: "UUID", Optional: true, Paragraph: true})
	if err != nil {
		return dh, remainder, err
	}

	if dh.Format, err = headers["SVN-fs-dump-format-version"].Int64(); err != nil || dh.Format > 2 {
		return dh, remainder, fmt.Errorf("dump format version: %w", err)
	}
	if uuid, ok := headers["UUID"]; ok {
		dh.ReposUUID = uuid.String
	}

	return dh, remainder, nil
}
