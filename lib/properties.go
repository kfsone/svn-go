package svn

import (
	"fmt"
	"io"
)

type Properties map[string]string

func NewProperties(data []byte) (*Properties, error) {
	// If there are no properties, there are no properties.
	if data == nil || len(data) <= 0 {
		return nil, nil
	}

	r := NewDumpReader(data)
	props := &Properties{}
	var key, value []byte
	var err error
	for !r.AtEOF() {
		// Properties ends with a line reading just "PROPS-END" and a newline.
		if _, ok := r.LineAfter(PropsEnd); ok {
			if len(*props) <= 0 {
				props = nil
			}
			return props, nil
		}

		if key, err = r.ReadSized('D'); err == nil {
			value = nil
		} else {
			key, err = r.ReadSized('K')
			if err != nil {
				return nil, err
			}
			value, err = r.ReadSized('V')
			if err != nil {
				return nil, err
			}
		}

		keyStr := string(key)
		if _, ok := (*props)[keyStr]; ok {
			return nil, fmt.Errorf("duplicate property: %s", keyStr)
		}

		(*props)[keyStr] = string(value)
	}

	return nil, io.ErrUnexpectedEOF
}
