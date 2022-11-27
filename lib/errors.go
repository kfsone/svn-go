package svn

import (
	"errors"
)

var (
	BadFormatHeaderError   = errors.New("bad format header")
	BadRevisionHeaderError = errors.New("bad revision header")
)
