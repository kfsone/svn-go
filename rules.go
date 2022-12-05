package main

import (
	"os"

	yml "gopkg.in/yaml.v3"
)

// OverFork describes a branching that was accidentally performed as
// a "fork", so that an entire project was copied rather than copying
// a single branch/Trunk.
//
// e.g.
//
//	copy /projects/Project01/* -> /projects/Project02/*
//
// should have been
//
//	add  /projects/Project02/Branches
//	add  /projects/Project02/Tags
//	copy /projects/Project01/Trunk -> /projects/Project02/Trunk
type OverFork struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// Convention describes the naming conventions used within the repos,
// i.e what the local versions of 'trunk', 'branches' and 'tags' are.
type Convention struct {
	Trunk    string `yaml:"trunk,omitempty"`
	Branches string `yaml:"branches,omitempty"`
	Tags     string `yaml:"tags,omitempty"`
}

// Rules captures the yaml description of a ruleset.
type Rules struct {
	Filename   string
	Convention Convention `yaml:"convention,omitempty"`
	CreateAt   int        `yaml:"creation-revision,omitempty"`
	Filter     []string   `yaml:"filter,omitempty"`
	FixPaths   []string   `yaml:"fixpath,omitempty"`
	OverForks  []OverFork `yaml:"overfork,omitempty"`
}

// NewRules returns a new Rules object populated from the yaml
// definition in a given file. If the file is empty, returns
// an empty ruleset.
func NewRules(filename string) (rules *Rules) {
	rules = &Rules{
		Filename: filename,
		CreateAt: 1,
		Convention: Convention{
			Trunk:    "Trunk",
			Branches: "Branches",
			Tags:     "Tags",
		},
	}

	// Only try and load the file if it has a name.
	if filename != "" {
		if f, err := os.ReadFile(filename); err == nil {
			if err = yml.Unmarshal(f, rules); err != nil {
				panic(err)
			}
		}
	}

	rules.Filename = filename

	return
}
