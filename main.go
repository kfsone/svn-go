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
	"bytes"
	"fmt"
	svn "github.com/kfsone/svn-go/lib"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type IterDirection int

const (
	IterFwd IterDirection = iota
	IterRev
)

type Status struct {
	*svn.Repos
	rules      *Rules
	folderNews map[string]*svn.Node
	folderAdds map[string]*svn.Node
	branchNews map[string]*svn.Node
	branchAdds map[string]*svn.Node
}

func NewStatus() (status *Status, err error) {
	status = &Status{
		Repos: svn.NewRepos(),
		// The FIRST creation of every folder.
		folderNews: make(map[string]*svn.Node),
		// The LAST creation of every folder.
		folderAdds: make(map[string]*svn.Node),
		// The FIRST creation of every branch.
		branchNews: make(map[string]*svn.Node),
		// The LAST creation of every branch.
		branchAdds: make(map[string]*svn.Node),
	}

	if status.rules, err = NewRules(*rulesFile); err != nil {
		return nil, err
	}

	return status, nil
}

func main() {
	parseCommandLine()

	if err := run(); err != nil {
		fmt.Println(fmt.Errorf("error: %w", err))
		os.Exit(1)
	}
}

func Log(format string, args ...any) {
	if *verbose {
		s := fmt.Sprintf("-- "+format, args...)
		s = strings.ReplaceAll(s, "\r", "<cr>")
		s = strings.ReplaceAll(s, "\n", "<lf>")
		fmt.Println(s)
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
	// Determine what files we're going to read.
	filenames, err := filepath.Glob(*dumpFileName)
	if err != nil {
		return fmt.Errorf("invalid dump file/glob: %s: %w", *dumpFileName, err)
	}
	if len(filenames) == 0 {
		return fmt.Errorf("no matching dump files found: %s", *dumpFileName)
	}

	// Prepare a repository view to load dumps into.
	status, err := NewStatus()
	if err != nil {
		return err
	}

	Info("Loading %d dump files", len(filenames))
	for _, filename := range filenames {
		Log("Loading dump file: %s", filename)
		dumpfile, err := svn.NewDumpFile(filename)
		if err != nil {
			return err
		}
		if err := dumpfile.LoadRevisions(); err != nil {
			return err
		}
		if err := status.AddDumpFile(dumpfile); err != nil {
			return err
		}
	}

	if *pathInfo {
		dumpPathInfo(status)
	}

	Info("Normalizing")
	for _, rev := range status.Revisions {
		processRevHelper(rev, status)
	}

	Info("Analyzing")
	if err = analyze(status); err != nil {
		return err
	}

	if *outFilename != "" {
		err = singleDump(*outFilename, status, 0, status.GetHead())
		if err == nil {
			fmt.Println("100% Complete")
		}
	} else if *outDir != "" {
		err = multiDump(*outDir, status)
		if err == nil {
			fmt.Println("100% Complete")
		}
	}

	if err != nil {
		return err
	}

	Info("Finished")

	return nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func singleDump(filename string, status *Status, start, end int) error {
	Info("Dumping r%9d:%9d -> %s", start, end, filename)

	out, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		must(out.Close())
	}()

	enc := svn.NewEncoder(out)
	defer enc.Close()

	for progress := range status.Encode(enc, start, end) {
		fmt.Printf("%5.2f%% r%d\r", progress.Percent, progress.Revision)
	}
	fmt.Printf("%6s %11s\r", "", "")

	return nil
}

func multiDump(outPath string, status *Status) error {
	// MkdirAll does nothing if the path already exists as a directory.
	if err := os.MkdirAll(outPath, 0700); err != nil {
		return err
	}

	for _, dumpfile := range status.DumpFiles {
		dumpFilename := filepath.Join(outPath, filepath.Base(dumpfile.Filename))
		start, end := dumpfile.Revisions[0].Number, dumpfile.Revisions[len(dumpfile.Revisions)-1].Number
		if err := singleDump(dumpFilename, status, start, end); err != nil {
			return err
		}
	}

	return nil
}

func analyze(status *Status) error {
	// Find branches that end up in a retrofit path but started out outside of it.
	refits := 0
	for refitNode := range getRefitBranches(status) {
		oldRev, oldPath, _ := refitNode.Branched()
		newPath := refitNode.Path()
		Info("Refit: %s becomes %s at r%d", oldPath, newPath, oldRev)
		refits++

		// Work backwards until we find where this node was created, and replace any references to it with the new path.
		refitRev := refitNode.Revision
		for rno := refitRev.Number - 1; rno >= 0; rno-- {
			rev := status.Revisions[rno]
			creation := rev.FindNode(func(node *svn.Node) bool {
				return node.Kind == svn.NodeKindDir && node.Action == svn.NodeActionAdd && node.Path() == oldPath
			})
			replaceAll(oldPath, newPath, rev, status)

			if creation != nil {
				Info("| -> replaced creation at %d", rno)
				creation.Headers.Set(svn.NodePathHeader, newPath)
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
		var deletionRemoved = false
		for rno := refitRev.Number; rno < status.GetHead(); rno++ {
			rev := status.Revisions[rno]
			// Remove any attempts to delete the old branch.
			deletion := svn.IndexFunc(rev.Nodes, func(node *svn.Node) bool {
				return node.Kind == svn.NodeKindDir && node.Action == svn.NodeActionDelete && node.Path() == oldPath
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

	if refits > 0 {
		Info("%d branch refits required", refits)
	} else {
		Info("No branch refits required")
	}

	return nil
}

// Replace all paths that begin with oldPath with newPath.
func replaceAll(oldPath, newPath string, rev *svn.Revision, status *Status) {
	for _, node := range rev.Nodes {
		nodePath := node.Path()
		if changed := svn.ReplacePathPrefix(nodePath, oldPath, newPath); changed != nodePath {
			node.Headers.Set(svn.NodePathHeader, changed)
		}

		if _, branchPath, branched := node.Branched(); branched {
			if changed := svn.ReplacePathPrefix(branchPath, oldPath, newPath); changed != branchPath {
				node.Headers.Set(svn.NodeCopyfromPathHeader, branchPath)
			}
		}

		if !node.Properties.HasKeyValues() {
			continue
		}

		oldBytes, newBytes := []byte(oldPath), []byte(newPath)
		for _, prop := range status.rules.RetroProps {
			if value, ok := node.Properties.Get(prop); ok {
				newVal := bytes.ReplaceAll(value, oldBytes, newBytes)
				if !bytes.Equal(newVal, value) {
					node.Properties.Set(prop, newVal)
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
				if svn.MatchPathPrefix(node.Path(), prefix) {
					_, branchPath, _ := node.Branched()
					if !svn.MatchPathPrefix(branchPath, prefix) {
						out <- node
						break
					}
				}
			}
		}
	}()

	return out
}

func dumpPathInfo(status *Status) {
	originals := make(map[string]int)
	finals := make(map[string]int)

	for _, rev := range status.Revisions {
		for _, node := range rev.Nodes {
			if node.Kind == svn.NodeKindDir && node.Action == svn.NodeActionAdd {
				path := node.Path()
				if _, present := originals[path]; !present {
					originals[path] = rev.Number
				}
				finals[path] = rev.Number
			}
		}
	}

	if len(originals) == 0 {
		fmt.Println("-- No directories created.")
		return
	}

	paths := make([]string, 0, len(originals))
	maxLen := 0
	for _, path := range paths {
		paths = append(paths, path)
		if len(path) > maxLen {
			maxLen = len(path)
		}
	}
	sort.Strings(paths)

	format := fmt.Sprintf("%%-%ds: %%s\n", maxLen)
	for _, path := range paths {
		original, final := originals[path], finals[path]
		detail := ""
		if original == final {
			detail = fmt.Sprintf("%d and done", original)
		} else {
			detail = fmt.Sprintf("%d, ..., %d", original, final)
		}

		fmt.Printf(format, path, detail)
	}
}
