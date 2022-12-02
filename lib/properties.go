package svn

import (
	"fmt"
	"io"
)

type Properties map[string][]byte

func NewProperties(r *DumpReader, length int) (*Properties, error) {
	// If there are no properties, there are no properties.
	if length <= 0 {
		return &Properties{}, nil
	}

	propertyData, err := r.Read(length)
	if err != nil {
		return nil, err
	}

	r = NewDumpReader(propertyData)
	props := &Properties{}
	for !r.Empty() {
		// Properties ends with a line reading just "PROPS-END" and a newline.
		if _, ok := r.LineAfter(PropsEnd); ok {
			return props, nil
		}

		key, err := r.ReadSized('K')
		if err != nil {
			return nil, err
		}
		value, err := r.ReadSized('V')
		if err != nil {
			return nil, err
		}

		keyStr := string(key)
		if _, ok := (*props)[keyStr]; ok {
			return nil, fmt.Errorf("duplicate property: %s", keyStr)
		}

		(*props)[keyStr] = value
	}

	return nil, io.ErrUnexpectedEOF
}
