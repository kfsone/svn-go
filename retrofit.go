package main

import (
	"fmt"
	svn "github.com/kfsone/svn-go/lib"
	"strings"
)

func retroFit(session *Session) error {
	Info("Retrofit")

	for _, retroPath := range session.rules.RetroPaths {
		Info("+ %s", retroPath)

		if err := retroFitPath(session, retroPath); err != nil {
			return err
		}
	}

	return nil
}

func findBranchesInto(intoNode *TreeNode, rootPath string) <-chan *svn.Node {
	ch := make(chan *svn.Node, 8)
	go func() {
		defer close(ch)
		for node := range intoNode.Walk() {
			for idx := len(node.Revisions) - 1; idx >= 0; idx-- {
				candidate := node.Revisions[idx]
				// Ignore items that are implicitly copied, since we don't modify those.
				if _, explicit := node.Explicit[candidate.Revision.Number]; !explicit {
					continue
				}
				if candidate.BranchRev > 0 {
					if !strings.HasPrefix(candidate.BranchPath, rootPath) {
						ch <- candidate
					}
				}
			}
		}
	}()

	return ch
}

func retroFitPath(session *Session, retroPath string) (err error) {
	// Check if the branch exists in the model, and if so determine the range
	// of revisions where things are being moved into it.
	retroNode, match := session.tree.Lookup(retroPath)
	if !match {
		return nil
	}

	// Build a list of the folders that need to be created back at rev1.
	branchedFolders := make(map[string]bool)
	for svnNode := range findBranchesInto(retroNode, retroPath) {
		fmt.Printf("->> %s branched from %s at r%d\n", svnNode.Path(), svnNode.BranchPath, svnNode.BranchRev)
		branchedFolders[svnNode.BranchPath] = true
	}

	return
}
