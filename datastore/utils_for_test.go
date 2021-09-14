package datastore

import (
	"bytes"
	"errors"
	"io"
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
	return io.NopCloser(bytes.NewBuffer([]byte{}))
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

func putBlob(n string, b []byte, c DS) {
	e := c.Save(n, bReader(b, nil, nil, nil))
	errPanic(e)
	if !exists(c, n) {
		panic("Blob does not exist: " + n)
	}
}

func getBlob(n string, c DS) []byte {
	r, e := c.Open(n)
	errPanic(e)
	d, e := io.ReadAll(r)
	errPanic(e)
	e = r.Close()
	errPanic(e)
	return d
}

func exists(c DS, n string) bool {
	exists, err := c.Exists(n)
	if err != nil {
		panic("Invalid error detected when testing blob's existence: " + err.Error())
	}
	return exists
}

type memoryNoConsistencyCheck struct {
	memory
}

func (m *memoryNoConsistencyCheck) Open(n string) (io.ReadCloser, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	b, ok := m.bmap[n]
	if !ok {
		return nil, ErrNotFound
	}

	return io.NopCloser(bytes.NewReader(b)), nil
}

func newMemoryNoConsistencyCheck() *memoryNoConsistencyCheck {
	return &memoryNoConsistencyCheck{
		memory: memory{
			bmap: make(map[string][]byte),
		},
	}
}

type memoryBrokenAutoNamed struct {
	memory
	breaker func(string) string
}

func (m *memoryBrokenAutoNamed) SaveAutoNamed(r io.ReadCloser) (string, error) {
	n, err := m.memory.SaveAutoNamed(r)
	if err != nil {
		return "", err
	}
	return m.breaker(n), nil
}

func newMemoryBrokenAutoNamed(breaker func(string) string) *memoryBrokenAutoNamed {
	return &memoryBrokenAutoNamed{
		memory: memory{
			bmap: make(map[string][]byte),
		},
		breaker: breaker,
	}
}
