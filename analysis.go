package main

import (
	"fmt"
	"sort"
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

func pathMatch(path string, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return len(path) == len(prefix) || path[len(prefix)] == '/'
}

func has(array []string, key string) (int, bool) {
	for idx := range array {
		if pathMatch(key, array[idx]) {
			return idx, true
		}
	}
	return -1, false
}

type Analysis struct {
	rules        *Rules
	revisions    []*svn.Revision
	pathsPending []string
	creations    map[string]*svn.Node
	migrations   map[string]*svn.Node
	replacements map[ /*old*/ string] /*new*/ string
	fixes        []*svn.Node
}

func NewAnalysis(df *svn.DumpFile, rules *Rules) *Analysis {
	a := &Analysis{
		rules:     rules,
		revisions: df.Revisions,

		// Make a copy of the paths we still need fixing.
		pathsPending: append([]string{}, rules.FixPaths...),

		// Track where the top level nodes are actually added, because we'll need to move that
		// back to the start of history.
		creations: make(map[string]*svn.Node),

		// Track instances where something outside a fixpath was moved into it,
		// e.g. /repos/Trunk -> /repos/Project/Trunk, but not within it.
		migrations: make(map[string]*svn.Node),

		// Make it easier to come back and replace old paths with their new form.
		replacements: make(map[string]string),

		fixes: make([]*svn.Node, 0, 1024),
	}

	// For each fixpath, go thru the svn history and find where we see it branched,
	// and find folders that were moved into it from outside.
	a.collectCreationsAndMigrations()

	// Now find all the references to old paths.
	a.collectFixups()

	// Make sure none of the creations already start at rev 1.
	creations := make(map[string]*svn.Node, len(a.creations))
	for path, node := range a.creations {
		if node.Revision.Number > rules.CreateAt {
			creations[path] = node
		}
	}
	a.creations = creations

	return a
}

func (a *Analysis) collectCreationsAndMigrations() {
	for _, rev := range a.revisions {
		for _, node := range rev.Nodes {
			// Deletions are no use to us.
			if node.Action == svn.NodeActionDelete {
				continue
			}
			if len(a.pathsPending) > 0 {
				a.checkPendingPaths(node)
			}

			a.checkPathMigration(node)
		}
	}
}

func (a *Analysis) collectFixups() {
	// Old paths we need to replace.
	replacing := getReplacementsList(a)

	for _, rev := range a.revisions {
		for _, node := range rev.Nodes {
			_, present := has(replacing, node.Path)
			if !present {
				present = propertiesContains(node.Properties, replacing)
			}
			if present {
				a.fixes = append(a.fixes, node)
			}
		}
	}
}

func propertiesContains(props *svn.Properties, keys []string) bool {
	if props != nil {
		for _, value := range *props {
			for _, key := range keys {
				if strings.Contains(value, key) {
					return true
				}
			}
		}
	}
	return false
}

func getReplacementsList(a *Analysis) []string {
	replacing := make([]string, 0, len(a.replacements))
	for old := range a.replacements {
		replacing = append(replacing, old)
	}

	// Refactor the list so it's sorted by length with the longest first, so that
	// we always apply most-specific replacements first.
	sort.Slice(replacing, func(i, j int) bool {
		return len(replacing[i]) > len(replacing[j])
	})
	return replacing
}

func (a *Analysis) checkPendingPaths(node *svn.Node) {
	idx, present := has(a.pathsPending, node.Path)
	if !present {
		return
	}

	if node.History != nil {
		fmt.Printf("%s created in r%d from r%d %s\n", node.Path, node.Revision.Number, node.History.Rev, node.History.Path)
	} else {
		fmt.Printf("%s created in r%d by %s %s\n", node.Path, node.Revision.Number, *node.Action, *node.Kind)
	}

	// Remove this path from the list since we've discovered it.
	a.pathsPending = append(a.pathsPending[:idx], a.pathsPending[idx+1:]...)
	a.creations[node.Path] = node
}

func (a *Analysis) checkPathMigration(node *svn.Node) {
	// Normal in-tree event.
	if node.History == nil {
		return
	}

	// Look for files whose current path is within one of the fixpaths,
	// but whose previous path was not.
	if _, present := has(a.rules.FixPaths, node.Path); !present {
		return
	}

	if _, present := has(a.rules.FixPaths, node.History.Path); present {
		return
	}

	// If we already think this path has been migrated...
	if _, migrated := a.migrations[node.Path]; migrated {
		return
	}
	if node.Kind == svn.NodeKindFile {
		panic(fmt.Errorf("unexpected file migration: r%d %s %s %s", node.Revision.Number, node.Path, *node.Action, *node.Kind))
	}

	a.migrations[node.Path] = node
	if original, present := a.replacements[node.History.Path]; !present {
		fmt.Printf("migrated %s -> %s in %d\n", node.History.Path, node.Path, node.Revision.Number)
		a.replacements[node.History.Path] = node.Path
	} else {
		fmt.Printf("also %s'd %s (%s) -> %s in %d\n", *node.Action, node.History.Path, original, node.Path, node.Revision.Number)
	}
}
