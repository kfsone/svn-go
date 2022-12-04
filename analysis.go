package main

import (
	"fmt"
	"strings"

	svn "github.com/kfsone/svn-go/lib"
)

// HasOneOfPrefixes returns true if the string has one of the listed prefixes.
func HasOneOfPrefixes(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func analyze(df *svn.DumpFile, rules *Rules) {
	origins := make(map[string]*svn.FileHistory)
	replacements := make(map[string]string)

	// For each fixpath, go thru the svn history and find out where it is *branched*
	for _, rev := range df.Revisions {
		for _, node := range rev.Nodes {
			// Look for nodes that came from somewhere else and aren't a deletion.
			if node.History == nil || node.Action == svn.NodeActionDelete {
				continue
			}
			// Ignore paths we already figured out.
			if _, present := origins[node.Path]; present {
				continue
			}
			if HasOneOfPrefixes(node.Path, rules.FixPaths) && !HasOneOfPrefixes(node.History.Path, rules.FixPaths) {
				if _, present := replacements[node.History.Path]; !present {
					replacements[node.History.Path] = node.Path
				}
				origins[node.Path] = node.History
				break
			}
		}
	}

	for path, origin := range origins {
		if path == replacements[origin.Path] {
			fmt.Printf("%-32s copied at %7d from %-20s\n", path, origin.Rev, origin.Path)
		}
		for rev := origin.Rev; rev > 0; rev-- {
			_, err := df.GetRevision(rev)
			if err != nil {
				panic(err)
			}
			// for _, node := range r.Nodes {

			// }
		}
	}
}
