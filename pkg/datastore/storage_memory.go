/*
Copyright © 2025 Bartłomiej Święcki (byo)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datastore

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/cinode/go/pkg/common"
)

type memory struct {

	// All known blobs
	bmap map[string][]byte

	// Currently locked blobs (write in progress)
	block map[string]struct{}

	// Mutex to blobs
	rw sync.RWMutex
}

var _ storage = (*memory)(nil)

func newStorageMemory() *memory {
	return &memory{
		bmap:  make(map[string][]byte),
		block: make(map[string]struct{}),
	}
}

func (m *memory) kind() string {
	return "Memory"
}

func (m *memory) address() string {
	return memoryPrefix
}

func (m *memory) openReadStream(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	b, ok := m.bmap[name.String()]
	if !ok {
		return nil, ErrNotFound
	}

	return io.NopCloser(bytes.NewReader(b)), nil
}

type memoryWriteCloser struct {
	m *memory
	b *bytes.Buffer
	n string
}

func (w *memoryWriteCloser) Write(b []byte) (int, error) {
	return w.b.Write(b)
}

func (w *memoryWriteCloser) Cancel() {
	w.m.rw.Lock()
	defer w.m.rw.Unlock()

	delete(w.m.block, w.n)
}

func (w *memoryWriteCloser) Close() error {
	w.m.rw.Lock()
	defer w.m.rw.Unlock()

	delete(w.m.block, w.n)
	w.m.bmap[w.n] = w.b.Bytes()
	return nil
}

func (m *memory) openWriteStream(ctx context.Context, name *common.BlobName) (WriteCloseCanceller, error) {
	m.rw.Lock()
	defer m.rw.Unlock()

	ns := name.String()

	if _, found := m.block[ns]; found {
		return nil, ErrUploadInProgress
	}

	m.block[ns] = struct{}{}

	return &memoryWriteCloser{
		b: bytes.NewBuffer(nil),
		n: ns,
		m: m,
	}, nil
}

func (m *memory) exists(ctx context.Context, n *common.BlobName) (bool, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	if _, ok := m.bmap[n.String()]; !ok {
		return false, nil
	}

	return true, nil
}

func (m *memory) delete(ctx context.Context, n *common.BlobName) error {
	m.rw.Lock()
	defer m.rw.Unlock()

	_, ok := m.bmap[n.String()]
	if !ok {
		return ErrNotFound
	}

	delete(m.bmap, n.String())
	return nil
}
