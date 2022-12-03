package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	svn "github.com/kfsone/svn-go/lib"
	yml "gopkg.in/yaml.v3"
)

var TestFile = "test.dmp"

type OverFork struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type Rules struct {
	Filename  string
	OverForks []OverFork `yaml:"overfork"`
	Filter    []string   `yaml:"filter"`
	FixPaths  []string   `yaml:"fixpath"`
}

func NewRules(filename string) (rules *Rules) {
	rules = &Rules{Filename: filename}
	if filename != "" {
		if f, err := os.ReadFile(filename); err == nil {
			if err = yml.Unmarshal(f, rules); err != nil {
				panic(err)
			}
		}
	}

	return
}

func main() {
	dumpFileName := flag.String("dump", "fortress.dmp", "path to dump file")
	stopRevision := flag.Int("stop", -1, "stop after loading this revision")
	rulesFile := flag.String("rules", "rules.yml", "path to rules file")

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

	var rules = NewRules(*rulesFile)

	df, err := svn.NewDumpFile(*dumpFileName)
	if err != nil {
		fmt.Println(fmt.Errorf("error: %w", err))
		os.Exit(1)
	}

	stopAt := *stopRevision
	if stopAt < 0 {
		stopAt = math.MaxInt
	}

	fixpaths := append([]string{}, rules.FixPaths...)

	var rev *svn.Revision
	for df.GetHead() < stopAt {
		if rev, err = df.NextRevision(); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(fmt.Errorf("error: %w", err))
			os.Exit(1)
		}

		for _, fixpath := range fixpaths {
			for _, node := range rev.Nodes {
				if strings.HasSuffix(node.Path, fixpath) {
					fmt.Printf("r %d path %s kind %d action %d\n", rev.Number, node.Path, node.Kind, node.Action)
				}
			}
		}
	}

	if *stopRevision >= 0 && df.GetHead() != stopAt {
		fmt.Println(fmt.Errorf("error: stop revision %d not reached", *stopRevision))
		os.Exit(1)
	}

	fmt.Printf("loaded: %d revisions\n", df.GetHead()+1)
}
