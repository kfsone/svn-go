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

type IterDirection int

const (
	IterFwd IterDirection = iota
	IterRev
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
	if err = analyze(status); err != nil {
		return err
	}

	if *outDumpName != "" {
		out, err := os.OpenFile(*outDumpName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		defer out.Close()

		Info("Writing to %s", *outDumpName)
		if err := writeDump(df, out); err != nil {
			return err
		}
	}

	Info("Finished")

	return nil
}

func writeDump(df *svn.DumpFile, w io.Writer) error {
	encoder, err := svn.NewEncoder(df)
	if err != nil {
		return err
	}
	if err := encoder.Encode(w); err != nil {
		return err
	}

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

func analyze(status *Status) error {
	// Find branches that end up in a retrofit path but started out outside of it.
	for refitNode := range getRefitBranches(status) {
		Info("Refit: %s becomes %s at r%d", refitNode.History.Path, refitNode.Path, refitNode.History.Rev)

		// Work backwards until we find where this node was created, and replace any references to it with the new path.
		oldPath, newPath := refitNode.History.Path, refitNode.Path
		refitRev := refitNode.Revision

		for rno := refitRev.Number - 1; rno >= 0; rno-- {
			rev := status.df.Revisions[rno]
			creation := rev.FindNode(func(node *svn.Node) bool {
				return node.Kind == svn.NodeKindDir && node.Action == svn.NodeActionAdd && node.Path == oldPath
			})
			replaceAll(oldPath, newPath, rev, status)

			if creation != nil {
				Info("| -> replaced creation at %d", rno)
				creation.Path = newPath
				creation.Modified = true
				break
			}
		}

		// Remove the merge itself from later parent node.
		if nodeNo := svn.Index(refitRev.Nodes, refitNode); nodeNo != -1 {
			refitRev.Nodes = append(refitRev.Nodes[:nodeNo], refitRev.Nodes[nodeNo+1:]...)
		} else {
			panic("refit node has gone away")
		}

		// Now seek forward to find references to the old path
		var deletionRemoved bool = false
		for rno := refitRev.Number; rno < status.df.GetHead(); rno++ {
			rev := status.df.Revisions[rno]
			// Remove any attempts to delete the old branch.
			deletion := svn.IndexFunc(rev.Nodes, func(node *svn.Node) bool {
				return node.Kind == svn.NodeKindDir && node.Action == svn.NodeActionDelete && node.Path == oldPath
			})
			if deletion != -1 {
				rev.Nodes = append(rev.Nodes[:deletion], rev.Nodes[deletion+1:]...)
				deletionRemoved = true
			}

			replaceAll(oldPath, newPath, rev, status)

			// stop once that folder is deleted.
			if deletionRemoved {
				break
			}
		}
	}

	return nil
}

// Replace all paths that begin with oldPath with newPath.
func replaceAll(oldPath, newPath string, rev *svn.Revision, status *Status) {
	for _, node := range rev.Nodes {
		if svn.ReplacePathPrefix(&node.Path, oldPath, newPath) {
			node.Modified = true
		}

		if node.History != nil && svn.ReplacePathPrefix(&node.History.Path, oldPath, newPath) {
			node.Modified = true
		}

		if !node.Properties.HasKeyValues() {
			continue
		}

		for _, prop := range status.rules.RetroProps {
			if value, ok := node.Properties.Table[prop]; ok {
				newVal := strings.ReplaceAll(value, oldPath, newPath)
				if newVal != value {
					node.Properties.Table[prop] = newVal
				}
			}
		}
	}
}

func getRefitBranches(status *Status) <-chan *svn.Node {
	out := make(chan *svn.Node, 16)

	go func() {
		out := out
		defer close(out)
		for _, node := range status.branchAdds {
			for _, prefix := range status.rules.RetroPaths {
				if svn.MatchPathPrefix(node.Path, prefix) {
					if !svn.MatchPathPrefix(node.History.Path, prefix) {
						out <- node
						break
					}
				}
			}
		}
	}()

	return out
}
