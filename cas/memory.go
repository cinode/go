package cas

import (
	"bytes"
	"io"
	"sync"
)

type memory struct {

	// All known blobs
	bmap map[string][]byte

	// Mutext to blobs
	rw sync.RWMutex
}

func (m *memory) Kind() string {
	return "Memory"
}

type memoryReader struct {
	r io.Reader
}

func (m *memoryReader) Read(p []byte) (n int, err error) {
	return m.r.Read(p)
}

func (m *memoryReader) Close() error {
	m.r = nil
	return nil
}

func (m *memory) Open(n string) (io.ReadCloser, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	b, ok := m.bmap[n]
	if !ok {
		return nil, ErrNotFound
	}

	return &memoryReader{r: bytes.NewReader(b)}, nil
}

type memoryWriter struct {
	b bytes.Buffer
	h *hasher
	m *memory
	n string
}

func (m *memoryWriter) Write(p []byte) (n int, err error) {
	m.h.Write(p)
	return m.b.Write(p)
}

func (m *memoryWriter) Close() error {
	// Test if name does match
	n := m.h.Name()
	if n != m.n {
		return ErrNameMismatch
	}

	// Save inside CAS data
	m.m.rw.Lock()
	defer m.m.rw.Unlock()
	m.m.bmap[n] = m.b.Bytes()
	m.h = nil
	m.m = nil
	return nil
}

func (m *memory) Save(n string) (io.WriteCloser, error) {
	return &memoryWriter{
		h: newHasher(),
		m: m, n: n,
	}, nil
}

func (m *memory) Exists(n string) bool {
	m.rw.RLock()
	defer m.rw.RUnlock()

	_, ok := m.bmap[n]
	return ok
}

func (m *memory) Delete(n string) error {
	m.rw.Lock()
	defer m.rw.Unlock()

	_, ok := m.bmap[n]
	if !ok {
		return ErrNotFound
	}

	delete(m.bmap, n)
	return nil
}

// InMemory returns simple in-memory CAS implementation
func InMemory() CAS {
	return &memory{
		bmap: make(map[string][]byte),
	}
}
