package graph

import (
	"io"
	"sync/atomic"
)

type arrayEntriesIterator struct {
	cancelFlag int32
	current    int
	nodes      []Node
	names      []string
}

func newArrayEntriesIterator(nodes []Node, names []string) EntriesIterator {
	return &arrayEntriesIterator{
		cancelFlag: 0,
		current:    -1,
		nodes:      nodes,
		names:      names,
	}
}

func (m *arrayEntriesIterator) isCancelled() bool {
	return atomic.LoadInt32(&m.cancelFlag) != 0
}

func (m *arrayEntriesIterator) Next() bool {
	if m.isCancelled() {
		return true
	}
	if m.current+1 >= len(m.nodes) {
		return false
	}
	m.current++
	return true
}

func (m *arrayEntriesIterator) GetEntry() (Node, string, error) {
	if m.isCancelled() {
		return nil, "", ErrIterationCancelled
	}
	if m.current < 0 || m.current >= len(m.nodes) {
		return nil, "", io.EOF
	}
	return m.nodes[m.current], m.names[m.current], nil
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

func (e *errorEntriesIterator) GetEntry() (Node, string, error) {
	return nil, "", e.err
}

func (e *errorEntriesIterator) Cancel() {
}
