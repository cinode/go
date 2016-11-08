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
	e map[string]DirEntry
}

func (m *memoryDirNode) Child(name string) (DirEntry, error) {
	defer m.rlock()()
	e, ok := m.e[name]
	if !ok {
		return e, ErrNotFound
	}
	return e, nil
}

func (m *memoryDirNode) List() (entries map[string]DirEntry, err error) {
	defer m.rlock()()
	ret := make(map[string]DirEntry)
	for k, v := range m.e {
		ret[k] = v.clone()
	}
	return ret, nil
}

func getMemoryNodeBase(n Node) *memoryNodeBase {
	switch n := n.(type) {
	case *memoryDirNode:
		return &n.memoryNodeBase
	case *memoryFileNode:
		return &n.memoryNodeBase
	default:
		return nil
	}
}

func (m *memoryDirNode) AttachChild(name string, entry DirEntry) (DirEntry, error) {

	if mnb := getMemoryNodeBase(entry.Node); mnb == nil || mnb.m != m.m {
		return DirEntry{}, ErrIncompatibleNode
	}

	defer m.lock()()
	// TODO: Recursion check?
	clone := entry.clone()
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
	d := &memoryDirNode{e: make(map[string]DirEntry)}
	d.init(m.m)
	for n, e := range m.e {
		d.e[n] = e.clone()
	}
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
