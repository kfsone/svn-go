package svn

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var MissingFieldError = errors.New("required field missing")
var MissingNewlineError = errors.New("missing newline")

type HeaderLine struct {
	Label     string
	Optional  bool
	Paragraph bool
	String    string
}
type Headers map[string]HeaderLine

func (r *HeaderLine) Read(source []byte) (remainder []byte, err error) {
	if bytes.HasPrefix(source, []byte(r.Label)) {
		remainder = source[len(r.Label):]
		if bytes.HasPrefix(remainder, []byte(": ")) {
			var text []byte
			text, remainder = EndLine(remainder[2:])
			if r.Paragraph {
				var ok bool
				remainder, ok = SkipNewline(remainder)
				if !ok {
					return remainder, MissingNewlineError
				}
			}
			r.String = strings.TrimSpace(string(text))
			return remainder, nil
		}
	}
	if !r.Optional {
		err = fmt.Errorf("%w: %s", MissingFieldError, r.Label)
	}
	return source, err
}

func (r HeaderLine) Int64() (int64, error) {
	return strconv.ParseInt(r.String, 10, 64)
}

func ReadItems(source []byte, items ...HeaderLine) (headers Headers, remainder []byte, err error) {
	headers = Headers{}
	for _, item := range items {
		remainder, err = item.Read(source)
		if err != nil {
			return
		}
		if len(remainder) == len(source) {
			continue
		}
		source = remainder
		headers[item.Label] = item
	}

	return headers, remainder, nil
}
