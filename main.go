package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"

	svn "github.com/kfsone/svn-go/lib"
)

var TestFile = "test.dmp"

func main() {
	dumpFileName := flag.String("dump", "", "path to dump file")
	stopRevision := flag.Int("stop", -1, "stop after loading this revision")

	flag.Parse()
	if dumpFileName == nil || *dumpFileName == "" {
		fmt.Println("missing -dump filename")
		os.Exit(1)
	}
	if len(flag.Args()) > 0 {
		fmt.Println("unexpected arguments")
		flag.Usage()
		return
	}

	df, err := svn.NewDumpFile(*dumpFileName)
	if err != nil {
		fmt.Println(fmt.Errorf("error: %w", err))
		os.Exit(1)
	}

	stopAt := *stopRevision
	if stopAt < 0 {
		stopAt = math.MaxInt
	}

	for df.GetHead() < stopAt {
		if err = df.NextRevision(); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(fmt.Errorf("error: %w", err))
			os.Exit(1)
		}
	}

	if *stopRevision >= 0 && df.GetHead() != stopAt {
		fmt.Println(fmt.Errorf("error: stop revision %d not reached", *stopRevision))
		os.Exit(1)
	}

	fmt.Printf("loaded: %d revisions\n", df.GetHead()+1)
}
