package main

// Functions for re-encoding the dumps.

import (
	"fmt"
	svn "github.com/kfsone/svn-go/lib"
	"os"
	"path/filepath"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func singleDump(filename string, session *Session, start, end int) error {
	Info("Dumping r%9d:%9d -> %s", start, end, filename)

	out, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		must(out.Close())
	}()

	enc := svn.NewEncoder(out)
	defer enc.Close()

	for progress := range session.Encode(enc, start, end) {
		fmt.Printf("%5.2f%% r%d\r", progress.Percent, progress.Revision)
	}
	fmt.Printf("%6s %11s\r", "", "")

	return nil
}

func multiDump(outPath string, session *Session) error {
	// MkdirAll does nothing if the path already exists as a directory.
	if err := os.MkdirAll(outPath, 0700); err != nil {
		return err
	}

	for _, dumpfile := range session.DumpFiles {
		dumpFilename := filepath.Join(outPath, filepath.Base(dumpfile.Filename))
		start, end := dumpfile.Revisions[0].Number, dumpfile.Revisions[len(dumpfile.Revisions)-1].Number
		if err := singleDump(dumpFilename, session, start, end); err != nil {
			return err
		}
		if err := endDump(dumpfile); err != nil {
			return err
		}
	}

	return nil
}
