package main

// This is a tool for retconning structural changes to a Subversion repository.
//
// Say your repository started out with /Trunk and /Branches, and later on you
// moved to the standard svn multi-project model: /Project/Trunk and
// /Project/Branches.
//
// This tool is intended to apply the new structure retroactively.
//
// Use "rules.yml" to configure the tool.
//
//  # use 'fixpath' to specify the path you apply retroactively
//  fixpath:
//    - /Project/Trunk
//
//  # use 'create-revision' to specify what revision you want the
//  # structure to be created at, usually 1.
//  create-revision: 1
//
//  # like svndumpfilter, this will elide certain paths from the
//  # repository entirely
//  filter:
//  - /BadProject
//  - /Project/BadProject
//
//  # TBI:
//  # When you create /Project2 from /Project1 by copying the entire
//  # '/Project1' directory, you are in effect forking, and what you
//  # probably really wanted was just to copy /Project1/Trunk.
//  # Use overfork to specify that the commit where "from" is copied
//  # to "to" should actually just be copying the trunk branch.
//
//

import (
	"fmt"
	"io"
	"math"
	"os"

	svn "github.com/kfsone/svn-go/lib"
	yml "gopkg.in/yaml.v3"
)

// stopAt is the actual revision number we'll stop at, which will be MaxInt
// unless the user specifies a value via -stop.
var stopAt int = math.MaxInt

// WorkConfig tracks parameters passed between the encoder workers.
type WorkConfig struct {
	into *os.File
	done chan bool
}

// Release the Worker resources and signal completion via done.
func (c *WorkConfig) Close() {
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
	parseCommandLine()

	if err := run(); err != nil {
		fmt.Println(fmt.Errorf("error: %w", err))
		os.Exit(1)
	}
}

func run() error {
	// The rules file shouldn't take any time to read, so load that first.
	var rules = NewRules(*rulesFile)

	// Confirm we can actually read the dump file.
	df, err := svn.NewDumpFile(*dumpFileName)
	if err != nil {
		return err
	}
	defer df.Close()

	// Create a file to write out the analysis to.
	out, err := os.Create("out.plan")
	if err != nil {
		return err
	}
	defer out.Close()

	// load revisions from the dump.
	if err := loadRevisions(); err != nil {
		return err
	}

	// Analyze the effect of the rules on the loaded revisions.
	if err := analyze(df, rules); err != nil {
		return err
	}

	return nil
}

func loadRevisions(out *os.File, df *svn.DumpFile) (err error) {
	// Create a worker thread to receive and describe revisions,
	// so that the loader thread can focus on just fetching
	// and parsing svn data.
	cfg := &WorkConfig{
		into: out,
		done: make(chan bool, 1), // signal from worker it's finished.
	}
	defer func() { <-cfg.done }()

	// Create a channel for us to send revisions to the worker.
	helper := make(chan *svn.Revision, 100)
	defer close(helper)

	// Start the worker pipeline.
	go describeWorker(helper, cfg)

	// Iterate revisions then forward them to the worker to read
	// the propertydata.
	var rev *svn.Revision
	for df.GetHead() < stopAt {
		if rev, err = df.NextRevision(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		helper <- rev
	}

	// If the user specified a -stop revision, check we actually reached it.
	if *stopRevision >= 0 && df.GetHead() != stopAt {
		return fmt.Errorf("-stop revision %d not reached", stopAt)
	}

	return err // nil in the nominal case.
}

func analyze(df *svn.DumpFile, rules *Rules) error {
	if len(rules.FixPaths) > 0 && rules.CreateAt >= df.GetHead() {
		return fmt.Errorf("create-revision %d >= head revision %d", rules.CreateAt, df.GetHead())
	}

	a := NewAnalysis(df, rules)

	if len(a.creations) > 0 {
		fmt.Printf("- %d creation events to move to r%d\n", len(a.creations), rules.CreateAt)
	}
	if len(a.migrations) > 0 {
		fmt.Printf("- %d migration events\n", len(a.migrations))
	}
	if a.lastRev > 0 {
		fmt.Printf("- last fixer revision: %d\n", a.lastRev)
	}

	fmt.Printf("loaded: %d revisions\n", df.GetHead()+1)

	return nil
}
