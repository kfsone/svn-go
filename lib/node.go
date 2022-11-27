package svn

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Node struct {
	Headers    map[string]string
	Properties Properties
	Data       []byte
}

func NewNode(source []byte) (*Node, []byte, error) {
	if !bytes.HasPrefix(source, []byte("Node-path:")) {
		return nil, source, nil
	}

	node := &Node{
		Headers:    make(map[string]string),
		Properties: Properties{},
	}

	var line []byte
	for {
		if len(source) == 0 {
			return nil, nil, io.ErrUnexpectedEOF
		}
		line, source = EndLine(source)
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			break
		}
		colon := bytes.Index(line, []byte(": "))
		if colon == -1 {
			panic(fmt.Errorf("malformed header: %s", line))
		}
		key, value := string(line[:colon]), strings.TrimSpace(string(line[colon+2:]))
		node.Headers[key] = value
	}

	var crs int64
	var err error
	var originalLength = int64(len(source))
	if source, crs, err = node.Properties.Read(source); err != nil {
		return nil, nil, err
	}
	var pcl, tcl, cl int64
	if pcl, err = strconv.ParseInt(node.Headers["Prop-content-length"], 10, 64); err != nil {
		panic(err)
	}
	if tcl, err = strconv.ParseInt(node.Headers["Text-content-length"], 10, 64); err != nil {
		panic(err)
	}
	if cl, err = strconv.ParseInt(node.Headers["Content-length"], 10, 64); err != nil {
		panic(err)
	}
	pcl += crs
	cl += crs

	if originalLength != int64(len(source))+pcl {
		panic("mismatch")
	}

	node.Data = source[:tcl]
	source = source[tcl:]
	var ok bool
	source, ok = SkipNewline(source)
	if !ok {
		panic("missing newline")
	}

	return node, source, nil
}
