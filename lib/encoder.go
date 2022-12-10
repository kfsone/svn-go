package svn

import (
	"fmt"
	"io"
)

type Encoder struct {
	df *DumpFile
	sink chan []byte
}

func NewEncoder(df *DumpFile) (*Encoder, error) {
	return &Encoder{df: df}, nil
}

func (e *Encoder) Encode(w io.Writer) error {
	e.sink = make(chan []byte, 32)

	go func () {
		e := e
		defer close(e.sink)
	
		e.df.Encode(e)
	} ()

	for chunk := range e.sink {
		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}

	return nil
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
			e.sink <- []byte{ '\n' }
		case 2:
			e.sink <- []byte{ '\n', '\n' }
		default:
			panic("invalid newline count")
	}
}
