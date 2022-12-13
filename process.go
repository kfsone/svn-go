package main

import (
	"fmt"

	svn "github.com/kfsone/svn-go/lib"
)

func mapDirectoryCreations(rev *svn.Revision, status *Status) {
	// Track when new directories are created, keeping both the first and last
	// instance for each.
	for _, node := range rev.Nodes {
		if node.Kind == svn.NodeKindDir && node.Action == svn.NodeActionAdd {
			var newDict *map[string]*svn.Node
			path := node.Path()
			// Track "last" creation of every dir/branch
			_, _, branched := node.Branched()
			if !branched { // creation
				status.folderAdds[path] = node
				newDict = &status.folderNews
			} else {
				status.branchAdds[path] = node
				newDict = &status.branchNews
			}

			// Track "first" creation of every dir/branch
			if _, ok := (*newDict)[path]; !ok {
				(*newDict)[path] = node
			}
		}
	}
}

// applyReplace applies the 'replace' rules to the revision header and all of it's nodes,
// such nodes path names, ancestor path and property values apply the replace operations.
// This even factors in the elimination of a top-level node, e.g. if you were
// replacing /svn/repos -> /, then you would have bogus 'add' operations. It also
// checks for out-of-bounds conditions like an attempt to delete such a directory,
// since you just can't.
func applyReplace(rev *svn.Revision, replacements map[string]string) {
	// Apply 'replace' rules to the revision header.
	rev.Properties.ApplyReplacements(replacements)

	deadNodes := make([]*svn.Node, 0)

	// And apply 'replace' rules to all of our revisions.
	for _, node := range rev.Nodes {
		// Fix the paths of every node in this revision.
		path := node.Path()
		changedPath := false
		if changed := svn.ReplacePathPrefixes(path, replacements); changed != path {
			node.Headers.Set(svn.NodePathHeader, changed)
			path = changed
			changedPath = true
		}

		if _, branchPath, branched := node.Branched(); branched {
			if changed := svn.ReplacePathPrefixes(branchPath, replacements); changed != branchPath {
				node.Headers.Set(svn.NodeCopyfromPathHeader, changed)
			}
		}

		// Did that bump us up to an invalid operation on root?
		if changedPath && path == "" {
			if isChangedNodePathDefunct(node) {
				deadNodes = append(deadNodes, node)
			}
		}

		node.Properties.ApplyReplacements(replacements)
	}

	if len(deadNodes) > 0 {
		Info("%d replaced paths eliminated from revision %d", len(deadNodes), rev.Number)
		nodes := make([]*svn.Node, 0, len(rev.Nodes)-len(deadNodes))
		for _, node := range rev.Nodes {
			////TODO: Audit that we don't have references to removed nodes.
			if node.Path() != "" {
				nodes = append(nodes, node)
			}
		}
		rev.Nodes = nodes
	}
}

// isChangedNodePathDefunct returns true if the node would now be defunct because it tries
// to apply an impossible operation to the root directory such as add or delete.
func isChangedNodePathDefunct(node *svn.Node) bool {
	switch node.Action {
	case svn.NodeActionDelete:
		/// Or maybe we should just ignore it?
		panic(fmt.Errorf("replace results in an attempt to delete / at r%d", node.Revision.Number))

	case svn.NodeActionReplace:
		panic(fmt.Errorf("replace results in an attempt to replace / at r%d", node.Revision.Number))

	case svn.NodeActionAdd:
		if _, _, branched := node.Branched(); branched {
			panic(fmt.Errorf("replace results in an attempt to add / with history at r%d", node.Revision.Number))
		}
		return true

	case svn.NodeActionChange:
		if _, branchPath, branched := node.Branched(); branched && branchPath != "" {
			panic(fmt.Errorf("replace results in an attempt to add / with history at r%d", node.Revision.Number))
		}
	}

	return false
}

// parsePropertiesWorker reads any properties in each revision and its nodes,
// and expands them into a Properties object.
func processRevHelper(rev *svn.Revision, status *Status) {
	// Apply 'replace'.
	applyReplace(rev, status.rules.Replace)

	// Find where all the directories are created.
	mapDirectoryCreations(rev, status)

	// Apply 'filter'.
	applyFilter(rev, status.rules.Filter)

	// Apply 'strip-props'.
	applyStripProps(rev, status.rules.StripProps)
}

func applyFilter(rev *svn.Revision, filters []string) {
	// We're not going to bother applying filters to metadata at this point.
	filtered := make(map[int]bool)
	for _, filter := range filters {
		for _, nodeIdx := range rev.GetNodeIndexesWithPrefix(filter) {
			filtered[nodeIdx] = true
		}

		// Check any filtered history nodes.
		for _, node := range rev.Nodes {
			_, branchedPath, branched := node.Branched()
			if branched && svn.MatchPathPrefix(branchedPath, filter) {
				panic(fmt.Errorf("filter:%s would break history of %s %s %s at r%d", filter, *node.Action, *node.Kind, node.Path(), rev.Number))
			}
		}
	}

	// Remove any filtered nodes from the revision's node list.
	if len(filtered) > 0 {
		nodes := make([]*svn.Node, 0, len(rev.Nodes)-len(filtered))
		for i, node := range rev.Nodes {
			if !filtered[i] {
				nodes = append(nodes, node)
			} else {
				Info("r%d: filtering node %s %s %s", rev.Number, *node.Action, *node.Kind, node.Path())
			}
		}
		rev.Nodes = nodes
		Info("r%d: filtered %d node(s)", rev.Number, len(filtered))
	}
}

func applyStripProps(rev *svn.Revision, stripProps []StripProp) {
	for _, node := range rev.Nodes {
		if !node.Properties.HasKeyValues() {
			continue
		}
		// Find properties we have.
		for _, stripProp := range stripProps {
			// Limit to files matching the given regexp.
			if !stripProp.fileRegexp.MatchString(node.Path()) {
				continue
			}
			for _, prop := range stripProp.Props {
				node.Properties.Remove(prop)
			}
		}
	}
}
