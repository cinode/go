package graph

import (
	"bytes"
	"io"
	"io/ioutil"

	"sync"
)

// InMemory returns in-memory implementation of entry point. This data is not
// persisted anywhere and is lost whenever instance of this EntryPoint is
// deleted. It's purpose is mostly for tests and prototypes.
func InMemory() EntryPoint {
	ret := &memory{}
	ret.root.m = ret
	ret.root.e = make(map[string]DirEntry)
	return ret
}

type memory struct {
	root  memoryDirNode
	mutex sync.RWMutex
}

func (m *memoryNodeBase) rlock() func() {
	m.m.mutex.RLock()
	return func() { m.m.mutex.RUnlock() }
}

func (m *memoryNodeBase) lock() func() {
	m.m.mutex.Lock()
	return func() { m.m.mutex.Unlock() }
}

func (m *memoryNodeBase) init(mem *memory) *memoryNodeBase {
	m.m = mem
	return m
}

func (m *memory) Root() (DirNode, error) {
	return &(m.root), nil
}

func (m *memory) NewDetachedDirNode() (DirNode, error) {
	ret := &memoryDirNode{e: make(map[string]DirEntry)}
	ret.init(m)
	return ret, nil
}

func (m *memory) NewDetachedFileNode() (FileNode, error) {
	ret := &memoryFileNode{}
	ret.init(m)
	return ret, nil
}

type memoryNodeBase struct {
	m *memory
}

type memoryDirNode struct {
	memoryNodeBase
	e DirEntryMap
}

func (m *memoryNodeBase) toMNB() *memoryNodeBase {
	return m
}

func (m *memoryDirNode) Child(name string) (DirEntry, error) {
	defer m.rlock()()
	e, ok := m.e[name]
	if !ok {
		return e, ErrNotFound
	}
	return e.clone(false), nil
}

func (m *memoryDirNode) List() (entries DirEntryMap, err error) {
	defer m.rlock()()
	return m.e.clone(false), nil
}

func (m *memoryDirNode) AttachChild(name string, entry DirEntry) (DirEntry, error) {

	mnb, ok := entry.Node.(interface {
		toMNB() *memoryNodeBase
	})
	if !ok || mnb.toMNB().m != m.m {
		return DirEntry{}, ErrIncompatibleNode
	}

	defer m.lock()()
	// TODO: Recursion check?
	clone := entry.clone(true)
	m.e[name] = clone
	return clone, nil
}

func (m *memoryDirNode) DetachChild(name string) error {
	defer m.lock()()
	if _, ok := m.e[name]; !ok {
		return ErrNotFound
	}
	delete(m.e, name)
	return nil
}

func (m *memoryDirNode) clone() Node {
	d := &memoryDirNode{e: m.e.clone(true)}
	d.init(m.m)
	return d
}

type memoryFileNode struct {
	memoryNodeBase
	data []byte
}

func (m *memoryFileNode) Open() (io.ReadCloser, error) {
	defer m.rlock()()
	return ioutil.NopCloser(bytes.NewReader(m.data)), nil
}

func (m *memoryFileNode) Save(r io.ReadCloser) error {
	b, err := ioutil.ReadAll(r)
	err2 := r.Close()
	if err != nil {
		return err
	}
	if err2 != nil {
		return err2
	}
	defer m.lock()()
	m.data = b
	return nil
}

func (m *memoryFileNode) clone() Node {
	ret := &memoryFileNode{data: m.data}
	ret.init(m.m)
	return ret
}
