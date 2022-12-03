package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"

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

func describeWorker(into *os.File, _ []string, work <-chan *svn.Revision, done chan<- bool) {
	defer func() {
		into.Close()
		done <- true
	}()

	for rev := range work {
		fmt.Fprintf(into, "%d:\n", rev.Number)
		fmt.Fprintf(into, "  data: [%d, %d]\n", rev.StartOffset, rev.EndOffset)
		if len(rev.Nodes) > 0 {
			fmt.Fprintf(into, "  nodes:\n")
			for _, node := range rev.Nodes {
				fmt.Fprintf(into, "  - path: %s\n", node.Path)
				fmt.Fprintf(into, "    action: %s\n", node.Action)
				fmt.Fprintf(into, "    kind: %s\n", node.Kind)
				if node.History != nil {
					fmt.Fprintf(into, "    from: %d\n", node.History.Rev)
					fmt.Fprintf(into, "    source: %s\n", node.History.Path)
				}
			}
		}

		// for _, fixpath := range fixpaths {
		// 	for _, node := range rev.Nodes {
		// 		if strings.HasSuffix(node.Path, fixpath) {
		// 			if node.History != nil {
		// 				fmt.Printf("r%d %s %s %s <- %d:%s\n", rev.Number, node.Action, node.Kind, node.Path, node.History.Rev, node.History.Path)
		// 			} else {
		// 				fmt.Printf("r%d %s %s %s\n", rev.Number, node.Action, node.Kind, node.Path)
		// 			}
		// 		}
		// 	}
		// }
	}
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
	fixpaths := append([]string{}, rules.FixPaths...)

	df, err := svn.NewDumpFile(*dumpFileName)
	if err != nil {
		fmt.Println(fmt.Errorf("error: %w", err))
		os.Exit(1)
	}

	stopAt := *stopRevision
	if stopAt < 0 {
		stopAt = math.MaxInt
	}

	out, err := os.Create("out.plan")
	if err != nil {
		fmt.Println(fmt.Errorf("error: %w", err))
		os.Exit(1)
	}

	work := make(chan *svn.Revision, 8)
	done := make(chan bool)

	go describeWorker(out, fixpaths, work, done)

	var rev *svn.Revision
	for df.GetHead() < stopAt {
		if rev, err = df.NextRevision(); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(fmt.Errorf("error: %w", err))
			os.Exit(1)
		}

		work <- rev
	}

	close(work)
	<-done

	if *stopRevision >= 0 && df.GetHead() != stopAt {
		fmt.Println(fmt.Errorf("error: stop revision %d not reached", *stopRevision))
		os.Exit(1)
	}

	fmt.Printf("loaded: %d revisions\n", df.GetHead()+1)
}
