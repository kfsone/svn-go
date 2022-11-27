package svn

import (
	"bytes"
	"fmt"
	"io"
)

type Properties map[string][]byte

func NewProperties() Properties {
	return Properties{}
}

func (r *Properties) Read(source []byte) (remainder []byte, crs int64, err error) {
	for len(source) > 0 {
		if bytes.HasPrefix(source, PropsEnd) {
			source = source[len(PropsEnd):]
			if bytes.HasPrefix(source, []byte{'\r'}) {
				crs++
				source = source[1:]
			}
			if bytes.HasPrefix(source, []byte{'\n'}) {
				source = source[1:]
			}
			return source, crs, nil
		}

		key, remainder, addCr, err := readSized(source, 'K')
		if err != nil {
			return source, crs, err
		}
		crs += addCr
		value, remainder, addCr, err := readSized(remainder, 'V')
		if err != nil {
			return source, crs, err
		}
		crs += addCr
		keyStr := string(key)
		if _, ok := (*r)[keyStr]; ok {
			return remainder, crs, fmt.Errorf("duplicate property: %s", keyStr)
		}
		(*r)[keyStr] = value
		source = remainder
	}
	return source, crs, io.EOF
}
