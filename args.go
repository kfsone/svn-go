package main

import (
	"flag"
	"fmt"
	"os"
)

// -dump: required, specifies name of the dump file toread.
var dumpFileName = flag.String("dump", "svn.dump", "path to dump file")

// -stop: optional, only read upto (including) this revision.
var stopRevision = flag.Int("stop", -1, "stop after loading this revision")

// -rules: optional, specifies a rules file to work with. default: rules.yml
var rulesFile = flag.String("rules", "rules.yml", "path to rules file")

func parseCommandLine() {
	// Process command line flags.
	flag.Parse()
	// confirm no unmparsed arguments.
	if len(flag.Args()) > 0 {
		fmt.Println("unexpected arguments")
		flag.Usage()
		return
	}

	// '-dump' is required.
	if dumpFileName == nil || *dumpFileName == "" {
		fmt.Println("missing -dump filename")
		os.Exit(1)
	}

	// if -stop is non-negative, replace stopAt.
	if *stopRevision >= 0 {
		stopAt = *stopRevision
	}
}
