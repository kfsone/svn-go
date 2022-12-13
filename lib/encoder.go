package svn

import (
	"flag"
	"fmt"
	"io"
)

type Encoder struct {
	sink chan []byte
	ok   chan bool
}

var rawWrites = flag.Bool("raw-writes", false, "use buffered io for writing dumps")

func NewEncoder(w io.Writer) *Encoder {
	e := &Encoder{
		sink: make(chan []byte, 8),
		ok:   make(chan bool),
	}

	go func() {
		e := e
		defer func() { e.ok <- true }()

		var err error
		if *rawWrites {
			err = rawWriter(w, e.sink)
		} else {
			err = bufferedWriter(w, e.sink)
		}
		if err != nil {
			panic(err)
		}
	}()

	return e
}

func rawWriter(w io.Writer, sink chan []byte) error {
	for data := range sink {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}

	return nil
}

func bufferedWriter(w io.Writer, sink chan []byte) error {
	// Create a 4kb scratch buffer into which we'll copy
	// things to write so we can batch up our writes.
	buffer := make([]byte, 0, 4*1024)

	for data := range sink {
		// If there's stuff in the buffer, try to add to it unless
		// that would push us over the cap. If it pushes us over
		// the cap, write the buffer and leave data outstanding.
		if len(buffer) > 0 {
			if len(buffer)+len(data) < cap(buffer) {
				buffer = append(buffer, data...)
				continue
			}
			if len(data) < cap(buffer) {
				cut := cap(buffer) - len(buffer)
				buffer, data = append(buffer, data[:cut]...), data[cut:]
			}
			if _, err := w.Write(buffer); err != nil {
				return err
			}
			buffer = buffer[:0]
		}
		if len(data) <= cap(buffer) {
			buffer = append(buffer, data...)
			continue
		}

		if _, err := w.Write(data); err != nil {
			return err
		}
	}

	if len(buffer) > 0 {
		if _, err := w.Write(buffer); err != nil {
			return err
		}
	}

	return nil
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
