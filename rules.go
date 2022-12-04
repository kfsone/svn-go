package main

import (
	"fmt"
	"os"
	"strings"

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

// Rules captures the yaml description of a ruleset.
type Rules struct {
	Filename  string
	OverForks []OverFork `yaml:"overfork"`
	Filter    []string   `yaml:"filter"`
	FixPaths  []string   `yaml:"fixpath"`
}

// NewRules returns a new Rules object populated from the yaml
// definition in a given file. If the file is empty, returns
// an empty ruleset.
func NewRules(filename string) (rules *Rules) {
	rules = &Rules{}

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

func (r *Rules) TestPropertyPaths(label string, props map[string]string) bool {
	for k, v := range props {
		for _, fixpath := range r.FixPaths {
			if strings.Contains(v, fixpath) {
				fmt.Printf("%s %s references %s\n", label, k, fixpath)
				return true
			}
		}
	}

	return false
}
