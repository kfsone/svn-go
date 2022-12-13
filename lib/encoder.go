package svn

import (
	"fmt"
	"io"
)

type Encoder struct {
	sink chan []byte
	ok   chan bool
}

func NewEncoder(w io.Writer) *Encoder {
	e := &Encoder{
		sink: make(chan []byte, 4),
		ok:   make(chan bool),
	}

	go func() {
		e := e
		defer func() { e.ok <- true }()
		for data := range e.sink {
			if _, err := w.Write(data); err != nil {
				panic(err)
			}
		}
	}()

	return e
}

func (e *Encoder) Close() {
	close(e.sink)
	// Wait for the writer to finish.
	<-e.ok
}

func (e *Encoder) Write(data []byte) {
	e.sink <- data
}

func (e *Encoder) Fprintf(format string, args ...any) {
	e.sink <- []byte(fmt.Sprintf(format, args...))
}

func (e *Encoder) Newlines(n int) {
	switch n {
	case 0:
		return
	case 1:
		e.sink <- []byte{'\n'}
	case 2:
		e.sink <- []byte{'\n', '\n'}
	default:
		panic("invalid newline count")
	}
}
