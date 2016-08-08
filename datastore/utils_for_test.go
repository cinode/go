package datastore

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
)

const (
	emptyBlobName = "ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4"
)

var testBlobs = []struct {
	name string
	data []byte
}{
	{"Pq2UxZQcWw2rN8iKPcteaSd4LeXYW2YphibQjmj3kUQC", []byte("Test")},
	{"TZ4M9KMpYgLEPBxvo36FR4hDpgvuoxqiu1BLzeT3xLAr", []byte("Test1")},
	{"ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4", []byte("")},
}

func emptyBlobReader() io.ReadCloser {
	return ioutil.NopCloser(bytes.NewBuffer([]byte{}))
}

type errorOnExists struct {
	memory
}

func (a *errorOnExists) Exists(name string) (bool, error) {
	return false, errors.New("Error")
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

func errPanic(e error) {
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
}

func putBlob(n string, b []byte, c CAS) {
	e := c.Save(n, bReader(b, nil, nil, nil))
	errPanic(e)
	if !exists(c, n) {
		panic("Blob does not exist: " + n)
	}
}

func getBlob(n string, c CAS) []byte {
	r, e := c.Open(n)
	errPanic(e)
	d, e := ioutil.ReadAll(r)
	errPanic(e)
	e = r.Close()
	errPanic(e)
	return d
}

func exists(c CAS, n string) bool {
	exists, err := c.Exists(n)
	if err != nil {
		panic("Invalid error detected when testing blob's existance: " + err.Error())
	}
	return exists
}
