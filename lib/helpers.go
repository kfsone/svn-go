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
func Log(format string, args ...interface{}) {
	if *Verbose {
		s := fmt.Sprintf(format, args...)
		s = strings.ReplaceAll(s, "\r", "<cr>")
		s = strings.ReplaceAll(s, "\n", "<lf>")
		fmt.Println(s)
	}
}

// IndexFunc returns the first index i satisfying f(s[i]),
// or -1 if none do.
func IndexFunc[E any](s []E, f func(E) bool) int {
	for i, v := range s {
		if f(v) {
			return i
		}
	}
	return -1
}

// Index returns the first index of the array satisfying s[i] == e,
// or -1 if none do.
func Index[E comparable](s []E, e E) int {
	return IndexFunc(s, func(x E) bool { return x == e })
}

// ReplacePathPrefixes returns the given path with any replacements defined in the ruleset.
func ReplacePathPrefixes(path *string, replacements map[string]string) (changed bool) {
	for prefix, replacement := range replacements {
		if len(prefix) == 0 {
			continue
		}
		if ReplacePathPrefix(path, prefix, replacement) {
			changed = true
		}
	}

	return changed
}

func ReplacePathPrefix(path *string, prefix, replacement string) (changed bool) {
	// Remove trailing slashes from the right side.
	trimmedPrefix := strings.TrimRight(prefix, "/")

	if strings.HasPrefix(*path, trimmedPrefix) {
		// Makes sure that it's a path-component match, so we don't match
		// "Model/" and "Models/".
		if len(*path) == len(trimmedPrefix) || (*path)[len(trimmedPrefix)] == '/' {
			result := strings.TrimLeft(replacement+(*path)[len(trimmedPrefix):], "/")
			if result != *path {
				*path = result
				return true
			}
		}
	}

	return false
}

// MatchPathPrefix returns true if the given path begins with the same path *components* as prefix.
// MatchPathPrefix checks if the given path matches the given prefix.
// If the path does not match the prefix, it returns false.
// If the path does match the prefix, it returns true if the path and the prefix are equal,
// or if the path has a trailing slash and the prefix is a prefix of the path.
// For example, if path is "foo/bar" and prefix is "foo", then this function returns true.
// If path is "foo/bar" and prefix is "foo/bar", then this function returns true.
// If path is "foo/bar" and prefix is "foo/bar/", then this function returns false.
// If path is "foo/bar/" and prefix is "foo/bar", then this function returns true.
// Always returns false if prefix is "/" or empty.
func MatchPathPrefix(path, prefix string) bool {
	path = strings.Trim(path, "/")
	prefix = strings.Trim(prefix, "/")

	if len(path) < len(prefix) {
		return false
	}
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	if len(path) == len(prefix) {
		return true
	}
	return path[len(prefix)] == '/'
}
