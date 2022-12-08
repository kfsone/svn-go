package main

import (
	"os"

	svn "github.com/kfsone/svn-go/lib"
	yml "gopkg.in/yaml.v3"
)

// yamlHelper describes each revision to a file as yaml.
func yamlHelper(rev *svn.Revision, into *os.File) {
	// Treat each revision as an array of one, and create an encoder for each,
	// so that the resulting document looks like a single array of revisions.
	// If we didn't do this, there'd be a document separator ('---') between
	// each revision.
	data := append([]*svn.Revision{}, rev)
	ymlenc := yml.NewEncoder(into)
	ymlenc.SetIndent(2)
	ymlenc.Encode(data)
}

type FirstAndLast struct {
	First int `yaml:"first,omitempty"`
	Last  int `yaml:"last,omitempty"`
}

func mapFirstAndLast(dict map[string]FirstAndLast, news map[string]*svn.Node, adds map[string]*svn.Node) {
	var detail bool = detailYaml != nil && *detailYaml

	for path, node := range news {
		if _, ok := dict[path]; !ok {
			first := node.Revision.Number
			last := adds[path].Revision.Number
			if detail || first != last {
				dict[path] = FirstAndLast{
					First: node.Revision.Number,
					Last:  adds[path].Revision.Number,
				}
			}
		}
	}
}

func writeReport(status *Status) error {
	// Open the file for writing.
	f, err := os.Create(*yamlFile)
	if err != nil {
		return err
	}
	defer f.Close()

	type YamlStatus struct {
		Rules    *Rules                  `yaml:"rules,omitempty"`
		Branches map[string]FirstAndLast `yaml:"branches,omitempty"`
		Folders  map[string]FirstAndLast `yaml:"folders,omitempty"`
	}
	yamlStatus := YamlStatus{
		Rules:    status.rules,
		Folders:  make(map[string]FirstAndLast),
		Branches: make(map[string]FirstAndLast),
	}

	mapFirstAndLast(yamlStatus.Folders, status.folderNews, status.folderAdds)
	mapFirstAndLast(yamlStatus.Branches, status.branchNews, status.branchAdds)

	ymlenc := yml.NewEncoder(f)
	ymlenc.SetIndent(2)
	ymlenc.Encode(yamlStatus)
	ymlenc.Close()

	if detailYaml != nil && *detailYaml {
		for _, rev := range status.df.Revisions {
			yamlHelper(rev, f)
		}
	}

	return nil
}
