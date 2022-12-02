package svn

import (
	"flag"
	"fmt"
	"strings"
)

var Verbose = flag.Bool("v", false, "Show verbose output")
var Quiet = flag.Bool("q", false, "Show less output")

func CheckArguments() error {
	if *Verbose && *Quiet {
		return fmt.Errorf("-verbose and -quiet conflict")
	}

	return nil
}

func log(format string, args ...interface{}) {
	if *Verbose {
		s := fmt.Sprintf(format, args...)
		s = strings.ReplaceAll(s, "\r", "<cr>")
		s = strings.ReplaceAll(s, "\n", "<lf>")
		fmt.Println(s)
	}
}

func info(format string, args ...interface{}) {
	if !*Quiet {
		s := fmt.Sprintf(format, args...)
		s = strings.ReplaceAll(s, "\r", "<cr>")
		s = strings.ReplaceAll(s, "\n", "<lf>")
		fmt.Println(s)
	}
}
