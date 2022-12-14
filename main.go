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
	"os"
	"path/filepath"
	"strings"

	svn "github.com/kfsone/svn-go/lib"
)

type IterDirection int

const (
	IterFwd IterDirection = iota
	IterRev
)

type Session struct {
	*svn.Repos
	rules *Rules
	tree  *Tree
}

func NewSession() (session *Session, err error) {
	session = &Session{
		Repos: svn.NewRepos(),
	}

	if session.rules, err = NewRules(*rulesFile); err != nil {
		return nil, err
	}

	return session, nil
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
	session, err := NewSession()
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
		if err := session.AddDumpFile(dumpfile); err != nil {
			return err
		}
	}

	Info("Normalizing %d revisions", len(session.Revisions))
	for _, rev := range session.Revisions {
		processRevHelper(rev, session)
	}

	Info("Building Tree")
	session.tree = NewTree()
	for r := 1; r < len(session.Revisions); r++ {
		rev := session.Revisions[r]
		for _, node := range rev.Nodes {
			err = session.tree.Insert(node)
			if err != nil {
				return err
			}
		}
	}

	if *outFilename != "" {
		err = singleDump(*outFilename, session, 0, session.GetHead())
		if err == nil {
			fmt.Println("100% Complete")
		}
	} else if *outDir != "" {
		err = multiDump(*outDir, session)
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
