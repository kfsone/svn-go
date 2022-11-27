package main

import (
	"fmt"
	"os"

	svn "kfsone/svn/lib"
)

var TestFile = "test.dmp"

func main() {
	source, err := os.ReadFile(TestFile)
	if err != nil {
		panic(err)
	}

	df, err := svn.NewDumpFile(TestFile, source)
	if err != nil {
		panic(err)
	}

	fmt.Printf("loaded: %+#v", df)
}
