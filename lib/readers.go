package svn

import (
	"strconv"
)

type HeaderLine struct {
	Label    string
	Optional bool
}

type Headers map[string]string

func (h Headers) Int(key string) (value int, err error) {
	if str, ok := h[key]; ok {
		return strconv.Atoi(str)
	}
	return value, ErrMissingField
}
