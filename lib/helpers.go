package svn

// Small helper functions.

import (
	"flag"
	"fmt"
	"strings"
)

// Whether to log additional output.
var Verbose = flag.Bool("v", false, "Show verbose output")

// Any argument validation.
func CheckArguments() error {
	return nil
}

// Log a message if verbose output is enabled.
func log(format string, args ...interface{}) {
	if *Verbose {
		s := fmt.Sprintf(format, args...)
		s = strings.ReplaceAll(s, "\r", "<cr>")
		s = strings.ReplaceAll(s, "\n", "<lf>")
		fmt.Println(s)
	}
}
