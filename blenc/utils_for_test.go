package blenc

import (
	"bytes"
	"io"
)

func errPanic(e error) {
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
}

type helperReader struct {
	buf     io.Reader
	onRead  func() error
	onEOF   func() error
	onClose func() error
}

func bReader(b []byte, onRead func() error, onEOF func() error, onClose func() error) *helperReader {

	nop := func() error {
		return nil
	}

	if onRead == nil {
		onRead = nop
	}
	if onEOF == nil {
		onEOF = nop
	}
	if onClose == nil {
		onClose = nop
	}

	return &helperReader{
		buf:     bytes.NewReader(b),
		onRead:  onRead,
		onEOF:   onEOF,
		onClose: onClose,
	}
}

func (h *helperReader) Read(b []byte) (n int, err error) {
	err = h.onRead()
	if err != nil {
		return 0, err
	}

	n, err = h.buf.Read(b)
	if err == io.EOF {
		err = h.onEOF()
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}

	return n, err
}

func (h *helperReader) Close() error {
	return h.onClose()
}
