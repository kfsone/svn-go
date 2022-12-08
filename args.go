package main

import (
	"flag"
	"fmt"
	"os"

	svn "github.com/kfsone/svn-go/lib"
)

// -dump: required, specifies name of the dump file toread.
var dumpFileName = flag.String("dump", "svn.dump", "path to dump file")

// -stop: optional, only read upto (including) this revision.
var stopRevision = flag.Int("stop", -1, "stop after loading this revision")

// -rules: optional, specifies a rules file to work with. default: rules.yml
var rulesFile = flag.String("rules", "rules.yml", "path to rules file")

// -yaml: optional, specifies a yaml file to create on completion.
var yamlFile = flag.String("yaml", "", "output yaml report to this file")

// -detailed: optional, increase the verbosity of the yaml output.
var detailYaml = flag.Bool("detailed", false, "Increase the verbosity of the yaml output. (default: false)")

// -quiet: suppress verbose output.
var quiet = flag.Bool("quiet", false, "suppress more output")

func parseCommandLine() {
	// Process command line flags.
	flag.Parse()

	// confirm no unparsed arguments.
	if len(flag.Args()) > 0 {
		fmt.Println("unexpected arguments")
		flag.Usage()
		os.Exit(1)
	}

	if err := svn.CheckArguments(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// '-dump' is required.
	if dumpFileName == nil || *dumpFileName == "" {
		fmt.Println("missing -dump filename")
		os.Exit(1)
	}

	if *svn.Verbose && *quiet {
		fmt.Println("-quiet and -verbose are mutually exclusive")
		os.Exit(1)
	}

	if *detailYaml && (yamlFile == nil || *yamlFile == "") {
		fmt.Println("-detailed requires -yaml")
		os.Exit(1)
	}

	// if -stop is non-negative, replace stopAt.
	if *stopRevision >= 0 {
		stopAt = *stopRevision
	}
}
