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

// -remove-originals: remove the original dump files once they are regenerated. requires -outdir
var removeOriginals = flag.Bool("remove-originals", false, "remove original dump files once they are regenerated. requires -outdir")

// - reduce the data size of any file containing > this many bytes to this size.
var reduceData = flag.Int("reduce-data", -1, "reduce the data size of any file containing > this many bytes to this size")

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

	if *reduceData != -1 && *outFilename == "" && *outDir == "" {
		fmt.Println("-reduce-data requires -outfile or -outdir")
		os.Exit(1)
	}

	if *removeOriginals && *outDir == "" {
		fmt.Println("-remove-originals requires -outdir")
		os.Exit(1)
	}

	if *verbose && *quiet {
		fmt.Println("-quiet and -verbose are mutually exclusive")
		os.Exit(1)
	}
}
