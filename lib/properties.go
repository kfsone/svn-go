package svn

import (
	"bytes"
	"fmt"
	"strconv"
)

// Properties captures an svn-dump property table (revision and
// node level properties). These appear in the dump in a sort
// of pascal style.
//
//	properties      := property+ 'PROPS-END' '\n' '\n'
//	property        := ( property-key property-value | property-deleted )
//	property-key    := 'K' property-body
//	property-value  := 'V' property-body
//	proprty-deleted := 'D' property-body
//	property-body   := ' ' length '\n' <byte>{length} '\n'
//
// To represent {"count": "1\n"} the dump would contain:
//
//	K 5<lf>
//	count<lf>
//	V 2<lf>
//	1<lf>
//	<lf>
//	PROPS-END<lf>
//	<lf>
type Properties struct {
	// List of the keys in the order they originally appeared.
	// Deleted keys can be detected by their absence in the table.
	index []string

	// Key->value of retained keys.
	table map[string][]byte

	// Has anything been changed?
	modified bool

	// The original content
	raw []byte
}

var propertiesSuffix = []byte(PropsEnd + "\n")

func NewProperties(dump *DumpReader, propLength int) (*Properties, error) {
	// Start the object.
	props := &Properties{
		index:    make([]string, 0),
		table:    make(map[string][]byte),
		modified: false,
	}

	data, err := dump.Read(propLength)
	if err != nil {
		return nil, err
	}

	if !bytes.HasSuffix(data, propertiesSuffix) {
		return nil, fmt.Errorf("properties block does not end with %s\\n", PropsEnd)
	}

	return props, nil
}

func (p *Properties) Load() (err error) {
	data := p.raw[:len(p.raw)-len(propertiesSuffix)]

	var prefix byte
	var key, value []byte
	for len(data) > 0 {
		prefix, key, data, err = readSizedField(data, 'K', 'D')
		if err != nil {
			return err
		}
		if prefix == 'K' {
			_, value, data, err = readSizedField(data, 'V')
			if err != nil {
				return err
			}

			p.table[string(key)] = value
		}

		p.index = append(p.index, string(key))
	}

	return nil
}

func find[T comparable](list []T, key T) int {
	for i, v := range list {
		if v == key {
			return i
		}
	}
	return -1
}

func readSizedField(data []byte, prefixes ...byte) (prefix byte, field []byte, body []byte, err error) {
	eol := bytes.IndexByte(data, '\n')
	if eol == -1 {
		return 0, nil, nil, fmt.Errorf("incomplete property field (missing linefeed)")
	}
	line, body := data[:eol], data[eol+1:]
	if len(line) < 3 || find(prefixes, line[0]) == -1 || line[1] != ' ' {
		return 0, nil, nil, fmt.Errorf("invalid property sizer")
	}
	length, err := strconv.Atoi(string(line[2:]))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("invalid property length: %w", err)
	}
	field, body = body[:length], body[length+1:]

	return data[0], field, body, nil
}

func (p *Properties) Bytes() (data []byte) {
	data = p.raw

	if p.modified == true {
		data = make([]byte, 0, len(p.raw))
		for _, key := range p.index {
			value, present := p.table[key]
			if present {
				data = encodeKeyValueField(data, 'K', []byte(key))
				data = encodeKeyValueField(data, 'V', value)
			} else {
				data = encodeKeyValueField(data, 'D', []byte(key))
			}
		}

		data = append(data, propertiesSuffix...)
	}

	return data
}

func encodeKeyValueField(data []byte, prefix byte, field []byte) []byte {
	length := fmt.Sprintf("%c %d\n", prefix, len(field))

	data = append(data, []byte(length)...)
	data = append(data, field...)
	data = append(data, '\n')

	return data
}

// HasKeyValues returns true if there are any extant keys in this
// property table, aside deletions.
func (p *Properties) HasKeyValues() bool {
	return len(p.table) > 0
}

// Empty returns true if there are no property assignments or
// deletions in this instance.
func (p *Properties) Empty() bool {
	return len(p.index) == 0
}

func (p *Properties) Get(key string) (value []byte, present bool) {
	value, present = p.table[key]
	return value, present
}

func (p *Properties) Set(key string, value []byte) {
	_, present := p.table[key]
	if !present {
		panic("shouldn't be any case to set a property that isn't already present.")
	}
	p.table[key] = value
}

func (p *Properties) Remove(key string) bool {
	if idx := find(p.index, key); idx != -1 {
		p.index = append(p.index[:idx], p.index[idx+1:]...)

		// It may not be in the Table but all we needed to know
		// was whether it was in the index or not, delete will
		// be a no-op if it wasn't present in the table.
		delete(p.table, key)

		p.modified = true

		return true
	}

	// It wasn't present.
	return false
}
