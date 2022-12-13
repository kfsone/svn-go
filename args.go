package main

import (
	"flag"
	"fmt"
	"os"
)

// -dump: required, specifies name of the dump file to read.
var dumpFileName = flag.String("read", "svn.dump", "path or glob of dump file(s) to be read")

// -rules: optional, specifies a rules file to work with. default: rules.yml
var rulesFile = flag.String("rules", "", "optional path to rules file")

// -verbose: crank up the output.
var verbose = flag.Bool("verbose", false, "emit more output")

// -quiet: suppress verbose output.
var quiet = flag.Bool("quiet", false, "suppress more output")

// -outfile: optional, write the entire dump to one file
var outFilename = flag.String("outfile", "", "specify a single file/path to write the entire dump to")

// -outdir: generate dump files in this directory.
var outDir = flag.String("outdir", "", "specify a directory to write dump file(s) to")

// -pathinfo: displays a list of all the paths that are created (and when) in the dump.
var pathInfo = flag.Bool("pathinfo", false, "display paths created in the loaded dump")

func parseCommandLine() {
	// Process command line flags.
	flag.Parse()

	// confirm no unparsed arguments.
	if len(flag.Args()) > 0 {
		fmt.Println("unexpected arguments")
		flag.Usage()
		os.Exit(1)
	}

	// '-dump' is required.
	if dumpFileName == nil || *dumpFileName == "" {
		fmt.Println("missing -dump filename")
		os.Exit(1)
	}

	if *outFilename != "" && *outDir != "" {
		fmt.Println("-outfile and -outdir are mutually exclusive")
		os.Exit(1)
	}

	if *verbose && *quiet {
		fmt.Println("-quiet and -verbose are mutually exclusive")
		os.Exit(1)
	}
}
