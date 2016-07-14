package cas

import (
	"bytes"
	"io"
	"io/ioutil"
	"sync"
)

type memory struct {

	// All known blobs
	bmap map[string][]byte

	// Mutext to blobs
	rw sync.RWMutex
}

// InMemory returns simple in-memory CAS implementation
func InMemory() CAS {
	return &memory{
		bmap: make(map[string][]byte),
	}
}

func (m *memory) Kind() string {
	return "Memory"
}

func (m *memory) Open(n string) (io.ReadCloser, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	b, ok := m.bmap[n]
	if !ok {
		return nil, ErrNotFound
	}

	return ioutil.NopCloser(bytes.NewReader(b)), nil
}

func (m *memory) saveInternal(r io.ReadCloser, checkName func(string) bool) (string, error) {
	h := newHasher()
	b := new(bytes.Buffer)

	_, err := io.Copy(b, io.TeeReader(r, h))
	if err != nil {
		r.Close()
		return "", err
	}

	err = r.Close()
	if err != nil {
		return "", err
	}

	name := h.Name()
	if !checkName(name) {
		return "", ErrNameMismatch
	}

	// Store buffer inside CAS data
	m.rw.Lock()
	defer m.rw.Unlock()

	m.bmap[name] = b.Bytes()
	return name, nil
}

func (m *memory) Save(name string, r io.ReadCloser) error {
	_, err := m.saveInternal(r, func(n string) bool { return name == n })
	return err
}

func (m *memory) SaveAutoNamed(r io.ReadCloser) (string, error) {
	return m.saveInternal(r, func(string) bool { return true })
}

func (m *memory) Exists(n string) (bool, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	if _, ok := m.bmap[n]; !ok {
		return false, nil
	}

	return true, nil
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
