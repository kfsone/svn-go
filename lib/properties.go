package svn

import (
	"fmt"
	"io"
)

// Properties captures an svn-dump property table (revision and
// node level properties). These appear in the dump in a sort
// of pascal style.
//  properties      := property+ 'PROPS-END' '\n' '\n'
//  property        := ( property-key property-value | property-deleted )
//  property-key    := 'K' property-body
//  property-value  := 'V' property-body
//  proprty-deleted := 'D' property-body
//  property-body   := ' ' length '\n' <byte>{length} '\n'
//
// To represent {"count": "1\n"} the dump would contain:
//
//  K 5<lf>
//  count<lf>
//  V 2<lf>
//  1<lf>
//  <lf>
//  PROPS-END<lf>
//  <lf>
//
type Properties struct {
	// List of the keys in the order they originally appeared.
	// Deleted keys can be detected by their absence in the table.
	Index	[]string

	// Key->value of retained keys.
	Table	map[string]string

	// The original content
	original []byte
}

func NewProperties(data []byte) (*Properties, error) {
	// Start the object.
	props := &Properties{
		Index: make([]string, 0),
		Table: make(map[string]string),
		original: data,
	}

	// If there are no properties, there are no properties.
	if data == nil || len(data) <= 0 {
		return props, nil
	}

	// Wrap the data in the same reader we're using elsewhere.
	r := NewDumpReader(data)

	var pfx byte
	var key, value []byte
	var err error
	for !r.AtEOF() {
		// Properties ends with a line reading just "PROPS-END" and a newline.
		if _, ok := r.LineAfter(PropsEnd); ok {
			return props, nil
		}

		if pfx, key, err = r.ReadSized("KD"); err != nil {
			return nil, err
		}
		if pfx == 'K' {
			if _, value, err = r.ReadSized("V"); err != nil {
				return nil, err
			}
			props.Table[string(key)] = string(value)
		}

		props.Index = append(props.Index, string(key))
	}

	return nil, io.ErrUnexpectedEOF
}

func (p *Properties) Bytes() (data []byte) {
	data = make([]byte, 0, len(p.original))
	if len(p.original) == 0 {
		return data
	}

	for _, key := range p.Index {
		value, present := p.Table[key]
		if present {
			data = encodeKeyValueField(data, 'K', key)
			data = encodeKeyValueField(data, 'V', value)
		} else {
			data = encodeKeyValueField(data, 'D', key)
		}
	}

	data = append(data, []byte(PropsEnd + "\n")...)

	return data
}

func encodeKeyValueField(data []byte, prefix byte, field string) []byte {
	length := fmt.Sprintf("%c %d\n", prefix, len(field))

	data = append(data, []byte(length)...)
	data = append(data, []byte(field)...)
	data = append(data, '\n')

	return data
}

func (p *Properties) Remove(key string) bool {
	if idx := Index(p.Index, key); idx != -1 {
		// It has to be in the Index for sure
		p.Index = append(p.Index[:idx], p.Index[idx+1:]...)

		// It may not be in the Table but all we needed to know
		// was whether it was in the index or not,
		delete(p.Table, key)

		return true
	}

	// It wasn't present.
	return false
}

// HasKeyValues returns true if there are any extant keys in this
// property table, aside deletions.
func (p *Properties) HasKeyValues() bool {
	return len(p.Table) > 0
}

// Empty returns true if there are no property assignments or
// deletions in this instance.
func (p *Properties) Empty() bool {
	return len(p.Index) == 0
}

