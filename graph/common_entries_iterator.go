package graph

import (
	"sync/atomic"
)

type arrayEntriesIterator struct {
	cancelFlag int32
	current    int
	nodes      []Node
	names      []string
	metadata   []MetadataMap
}

func newArrayEntriesIterator(
	nodes []Node,
	names []string,
	metadata []MetadataMap,
) EntriesIterator {
	return &arrayEntriesIterator{
		cancelFlag: 0,
		current:    -1,
		nodes:      nodes,
		names:      names,
		metadata:   metadata,
	}
}

func (m *arrayEntriesIterator) isCancelled() bool {
	return atomic.LoadInt32(&m.cancelFlag) != 0
}

func (m *arrayEntriesIterator) Next() bool {
	if m.isCancelled() {
		return true
	}
	m.current++
	if m.current < len(m.nodes) {
		return true
	} else if m.current == len(m.nodes) {
		return false
	} else {
		panic("EntriesIterator: Next() called after previous Next() returned false")
	}
}

func (m *arrayEntriesIterator) GetEntry() (Node, string, MetadataMap, error) {
	if m.isCancelled() {
		return nil, "", nil, ErrIterationCancelled
	}
	panicOn(m.current < 0, "EntriesIterator: GetEntry() called before Next()")
	panicOn(m.current >= len(m.nodes), "EntriesIterator: GetEntry() called after Next() returned false")
	return m.nodes[m.current],
		m.names[m.current],
		m.metadata[m.current],
		nil
}

func (m *arrayEntriesIterator) Cancel() {
	atomic.StoreInt32(&m.cancelFlag, 1)
}

type errorEntriesIterator struct {
	err error
}

func newErrorEntriesIterator(err error) EntriesIterator {
	return &errorEntriesIterator{
		err: err,
	}
}

func (e *errorEntriesIterator) Next() bool {
	return true
}

func (e *errorEntriesIterator) GetEntry() (Node, string, MetadataMap, error) {
	return nil, "", nil, e.err
}

func (e *errorEntriesIterator) Cancel() {
}
