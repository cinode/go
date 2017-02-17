package graph

import (
	"bytes"
	"io"
	"io/ioutil"

	"sync"
)

type memoryDirEntry struct {
	n Node
}

func (m *memoryDirEntry) clone() memoryDirEntry {
	n, err := m.n.clone()
	if err != nil {
		panic("Memory-based nodes must not return errors while cloning")
	}
	return memoryDirEntry{n: n}
}

type memoryDirEntryMap map[string]memoryDirEntry

func (m *memoryDirEntryMap) clone() memoryDirEntryMap {
	ret := make(memoryDirEntryMap)
	for name, entry := range *m {
		ret[name] = entry.clone()
	}
	return ret
}

// InMemory returns in-memory implementation of entry point. This data is not
// persisted anywhere and is lost whenever instance of this EntryPoint is
// deleted. It's purpose is mostly for tests and prototypes.
func InMemory() EntryPoint {
	ret := &memory{}
	ret.root.m = ret
	ret.root.e = make(memoryDirEntryMap)
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
	ret := &memoryDirNode{e: make(memoryDirEntryMap)}
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

func (m *memoryNodeBase) toMNB() *memoryNodeBase {
	return m
}

type memoryDirNode struct {
	memoryNodeBase
	e memoryDirEntryMap
}

func (m *memoryDirNode) GetEntry(name string) (Node, error) {
	defer m.rlock()()
	e, ok := m.e[name]
	if !ok {
		return nil, ErrEntryNotFound
	}
	return e.n, nil
}

func (m *memoryDirNode) HasEntry(name string) (bool, error) {
	defer m.rlock()()
	_, ok := m.e[name]
	return ok, nil
}

func (m *memoryDirNode) SetEntry(name string, node Node) (Node, error) {

	mnb, ok := node.(interface {
		toMNB() *memoryNodeBase
	})
	if !ok || mnb.toMNB().m != m.m {
		return nil, ErrIncompatibleNode
	}

	defer m.lock()()
	// TODO: Recursion check?
	clone, _ := node.clone()
	m.e[name] = memoryDirEntry{n: clone}
	return clone, nil
}

func (m *memoryDirNode) DeleteEntry(name string) error {
	defer m.lock()()
	if _, ok := m.e[name]; !ok {
		return ErrEntryNotFound
	}
	delete(m.e, name)
	return nil
}

func (m *memoryDirNode) ListEntries() EntriesIterator {
	defer m.rlock()()

	nodes := make([]Node, len(m.e))
	names := make([]string, len(m.e))

	i := 0
	for name, node := range m.e {
		nodes[i] = node.n
		names[i] = name
		i++
	}
	return newArrayEntriesIterator(nodes, names)
}

func (m *memoryDirNode) clone() (Node, error) {
	d := &memoryDirNode{e: m.e.clone()}
	d.init(m.m)
	return d, nil
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

func (m *memoryFileNode) clone() (Node, error) {
	ret := &memoryFileNode{data: m.data}
	ret.init(m.m)
	return ret, nil
}
