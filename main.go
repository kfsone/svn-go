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

var dumpFileName = flag.String("dump", "deltas.dmp", "path to dump file")
var stopRevision = flag.Int("stop", -1, "stop after loading this revision")
var rulesFile = flag.String("rules", "rules.yml", "path to rules file")

type WorkConfig struct {
	into  *os.File
	rules *Rules
	done  chan bool
}

func (c *WorkConfig) Close() {
	c.into.Close()
	c.done <- true
}

// encoderWorker finishes the process of describing a revision, by generating the
// yaml representation into the output file.
func encoderWorker(work <-chan *svn.Revision, cfg *WorkConfig) {
	defer cfg.Close()

	for rev := range work {
		// Treat eat revision as an array of one, and create an encoder for each,
		// so that the resulting document looks like a single array of revisions.
		// If we didn't do this, there'd be a document separator ('---') between
		// each revision.
		data := append([]*svn.Revision{}, rev)
		ymlenc := yml.NewEncoder(cfg.into)
		ymlenc.SetIndent(2)
		ymlenc.Encode(data)
	}
}

// describeWorker starts the process of describing a revision, by populating the
// PropertyData structures, and then forwarding the revision to the next stage.
func describeWorker(work <-chan *svn.Revision, cfg *WorkConfig) {
	helper := make(chan *svn.Revision, 64)
	defer close(helper)

	// Fire-up a goroutine to handle the encoding.
	go encoderWorker(helper, cfg)

	var err error
	for rev := range work {
		// Generate property data for the revision header.
		if rev.Properties, err = svn.NewProperties(rev.PropertyData); err != nil {
			fmt.Printf("ERROR: r%d properties: %s\n", rev.Number, err)
		}

		// And generate property data for all of our revisions.
		for _, node := range rev.Nodes {
			if node.Properties, err = svn.NewProperties(node.PropertyData); err != nil {
				fmt.Printf("ERROR: r%d node properties: %s\n", rev.Number, err)
			}
		}
		helper <- rev
	}
}

func main() {
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

	out, err := os.Create("out.plan")
	if err != nil {
		fmt.Println(fmt.Errorf("error: %w", err))
		os.Exit(1)
	}

	// Create a worker thread to receive and describe revisions,
	// so that the loader thread can focus on just fetching
	// and parsing svn data.
	cfg := &WorkConfig{
		into:  out,
		rules: rules,
		done:  make(chan bool, 1),
	}
	helper := make(chan *svn.Revision, 100)
	go describeWorker(helper, cfg)

	var rev *svn.Revision
	for df.GetHead() < stopAt {
		if rev, err = df.NextRevision(); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(fmt.Errorf("error: %w", err))
			os.Exit(1)
		}

		helper <- rev
	}

	close(helper)
	<-cfg.done

	if *stopRevision >= 0 && df.GetHead() != stopAt {
		fmt.Printf("error: stop revision %d not reached\n", *stopRevision)
		os.Exit(1)
	}

	if len(rules.FixPaths) > 0 && rules.CreateAt >= df.GetHead() {
		fmt.Printf("creation-revision (%d) >= head revision (%d)\n", rules.CreateAt, df.GetHead())
		os.Exit(1)
	}

	a := NewAnalysis(df, rules)

	if len(a.creations) > 0 {
		fmt.Printf("- %d creation events to move to r%d\n", len(a.creations), rules.CreateAt)
	}
	if len(a.migrations) > 0 {
		fmt.Printf("- %d migration events\n", len(a.migrations))
	}
	if len(a.fixes) > 0 {
		fmt.Printf("- %d nodes to fixup\n", len(a.fixes))
	}

	fmt.Printf("loaded: %d revisions\n", df.GetHead()+1)
}
