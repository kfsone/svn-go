package svn

type DumpFile struct {
	Path       string
	DumpHeader DumpHeader
	Revisions  []*Revision
}

func NewDumpFile(path string, source []byte) (df *DumpFile, err error) {
	df = &DumpFile{
		Path: path,
	}

	if df.DumpHeader, source, err = NewDumpHeader(source); err != nil {
		return nil, err
	}

	df.Revisions = make([]*Revision, 0, 16384)

	for len(source) > 0 {
		rev, remainder, err := NewRevision(source)
		if err != nil {
			return nil, err
		}
		df.Revisions = append(df.Revisions, rev)
		source = remainder
	}

	return df, nil
}
