package svn

import (
	"fmt"
	"io"
)

type Encoder struct {
	sink chan []byte
}

func NewEncoder(w io.Writer) *Encoder {
	e := &Encoder{
		sink: make(chan []byte, 32),
	}

	go func() {
		e := e
		for data := range e.sink {
			if _, err := w.Write(data); err != nil {
				panic(err)
			}
		}
	}()

	return e
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
