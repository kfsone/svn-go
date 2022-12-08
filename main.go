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
//  # use 'retrofit-paths' to specify the path you apply retroactively
//  retrofit-paths:
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
	"strings"

	svn "github.com/kfsone/svn-go/lib"
)

type Status struct {
	df         *svn.DumpFile
	rules      *Rules
	folderNews map[string]*svn.Node
	folderAdds map[string]*svn.Node
	branchNews map[string]*svn.Node
	branchAdds map[string]*svn.Node
}

func NewStatus(df *svn.DumpFile, rules *Rules) *Status {
	return &Status{
		df:    df,
		rules: rules,
		// The FIRST creation of every folder.
		folderNews: make(map[string]*svn.Node),
		// The LAST creation of every folder.
		folderAdds: make(map[string]*svn.Node),
		// The FIRST creation of every branch.
		branchNews: make(map[string]*svn.Node),
		// The LAST creation of every branch.
		branchAdds: make(map[string]*svn.Node),
	}
}

// stopAt is the actual revision number we'll stop at, which will be MaxInt
// unless the user specifies a value via -stop.
var stopAt int = math.MaxInt

func main() {
	parseCommandLine()

	if err := run(); err != nil {
		fmt.Println(fmt.Errorf("error: %w", err))
		os.Exit(1)
	}
}

// Info prints a message if -quiet was not specified.
func Info(format string, args ...interface{}) {
	if !*quiet {
		s := fmt.Sprintf("-- "+format, args...)
		s = strings.ReplaceAll(s, "\r", "<cr>")
		s = strings.ReplaceAll(s, "\n", "<lf>")
		fmt.Println(s)
	}
}

func run() error {
	// The rules file shouldn't take any time to read, so load that first.
	Info("Loading rules")
	var rules = NewRules(*rulesFile)

	// Confirm we can actually read the dump file.
	Info("Opening dump file: %s", *dumpFileName)
	df, err := svn.NewDumpFile(*dumpFileName)
	if err != nil {
		return err
	}
	defer df.Close()

	// load revisions from the dump.
	Info("Loading revisions")
	status := NewStatus(df, rules)
	if err = loadRevisions(status); err != nil {
		return err
	}

	Info("Analyzing")
	if err = analyze(df, status); err != nil {
		return err
	}

	if yamlFile != nil && *yamlFile != "" {
		if err = writeReport(status); err != nil {
			return err
		}
	}

	Info("Finished")

	return nil
}

func loadRevisions(status *Status) (err error) {
	helper := NewHelper(32, processRevHelper, status)
	defer helper.CloseWait()

	// Iterate revisions then forward them to the worker to read
	// the propertydata.
	var rev *svn.Revision
	for status.df.GetHead() < stopAt {
		if rev, err = status.df.NextRevision(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		helper.Queue(rev)
	}

	// If the user specified a -stop revision, check we actually reached it.
	if *stopRevision >= 0 && status.df.GetHead() != stopAt {
		return fmt.Errorf("-stop revision %d not reached", stopAt)
	}

	return err // nil in the nominal case.
}

// IsSortedFunc reports whether x is sorted in ascending order, with less as the
// comparison function.
func IsSortedFunc[E any](x []E, less func(a, b E) bool) bool {
	for i := len(x) - 1; i > 0; i-- {
		if less(x[i], x[i-1]) {
			return false
		}
	}
	return true
}

// IndexFunc returns the first index i satisfying f(s[i]),
// or -1 if none do.
func IndexFunc[E any](s []E, f func(E) bool) int {
	for i, v := range s {
		if f(v) {
			return i
		}
	}
	return -1
}

type NodeLookup map[string]*svn.Node

func analyze(df *svn.DumpFile, status *Status) error {
	// Look for the inflection points where things are moved from outside
	// into the retfit structure.
	refitsCreated := NodeLookup{}

	// Confirm all refits actually exist.
	for _, refit := range status.rules.RetroPaths {
		created, present := status.folderNews[refit]
		if !present {
			// Uh oh, is this actually a branch?
			if created, present = status.branchNews[refit]; present {
				return fmt.Errorf("refit:%s: can't refit a branch, only children of an actual folder.", refit)
			}
			fmt.Printf("refit:%s: folder not found (did you modify paths with 'replace'?)\n", refit)
			for path, node := range status.folderNews {
				if len(path) < len(refit)+5 {
					fmt.Printf(" + %s: r%d\n", path, node.Revision.Number)
				}
			}
		} else {
			refitsCreated[refit] = created
		}
	}

	createAtRev, err := status.df.GetRevision(status.rules.CreateAt)
	if err != nil {
		return fmt.Errorf("createat r%d: %w", status.rules.CreateAt, err)
	}
	movedNodes := make([]*svn.Node, 0, len(refitsCreated))

	for _, refit := range status.rules.RetroPaths {
		created, ok := refitsCreated[refit]
		if !ok {
			continue
		}
		if created.Revision.Number <= status.rules.CreateAt {
			Info("* refit:%s: folder creation at r%d < createat r%d", refit, created.Revision.Number, createAtRev.Number)
			continue
		}

		Info("+ %s moving creation from r%d to r%d", refit, created.Revision.Number, createAtRev.Number)

		// Capture the creation and remove it from the old node.
		movedNodes = append(movedNodes, created)
		idx := IndexFunc(created.Revision.Nodes, func(a *svn.Node) bool {
			return a == created
		})
		if idx == -1 {
			panic("created lookup failed")
		}

		// Remove the node from its original slot.
		created.Revision.Nodes = append(created.Revision.Nodes[:idx], created.Revision.Nodes[idx+1:]...)

		if !IsSortedFunc(created.Revision.Nodes, func(a, b *svn.Node) bool {
			return a.Path < b.Path
		}) {
			panic(fmt.Errorf("r%d paths not sorted", created.Revision.Number))
		}

		// Ensure the node is added after anything that needs creating before it.
		newNodes := make([]*svn.Node, 0, len(createAtRev.Nodes)+1)
		idx = IndexFunc(createAtRev.Nodes, func(a *svn.Node) bool {
			return a.Path >= created.Path
		})
		// Nothing found, append
		if idx == -1 {
			newNodes = append(createAtRev.Nodes, created)
		} else {
			if createAtRev.Nodes[idx].Path == created.Path {
				panic("creation already present???")
			}
			// Insert before the first node with a path greater than the created node.
			newNodes = append(newNodes, createAtRev.Nodes[:idx]...)
			newNodes = append(newNodes, created)
			newNodes = append(newNodes, createAtRev.Nodes[idx:]...)
		}

		createAtRev.Nodes = newNodes
	}

	return nil
}
