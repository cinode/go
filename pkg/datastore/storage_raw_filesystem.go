/*
Copyright © 2023 Bartłomiej Święcki (byo)

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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/cinode/go/pkg/common"
)

type rawFileSystem struct {
	path        string
	tempFileNum uint64
}

var _ storage = (*rawFileSystem)(nil)

func newStorageRawFilesystem(path string) *rawFileSystem {
	return &rawFileSystem{
		path: path,
	}
}

func (fs *rawFileSystem) kind() string {
	return "RawFileSystem"
}

func (fs *rawFileSystem) openReadStream(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	rc, err := os.Open(filepath.Join(fs.path, name.String()))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	return rc, err
}

type rawFilesystemWriter struct {
	file         *os.File
	destFileName string
}

func (w *rawFilesystemWriter) Write(b []byte) (int, error) {
	return w.file.Write(b)
}

func (w *rawFilesystemWriter) Close() error {
	err := w.file.Close()
	if err != nil {
		return err
	}

	return os.Rename(w.file.Name(), w.destFileName)
}

func (w *rawFilesystemWriter) Cancel() {
	w.file.Close()
	os.Remove(w.file.Name())
}

func (fs *rawFileSystem) openWriteStream(ctx context.Context, name common.BlobName) (WriteCloseCanceller, error) {
	// Ensure dir exists
	err := os.MkdirAll(fs.path, 0755)
	if err != nil {
		return nil, err
	}

	tempNum := atomic.AddUint64(&fs.tempFileNum, 1)

	tempFileName := filepath.Join(fs.path, fmt.Sprintf("tempfile_%d", tempNum))

	fl, err := os.Create(tempFileName)
	if err != nil {
		return nil, err
	}

	return &rawFilesystemWriter{
		file:         fl,
		destFileName: filepath.Join(fs.path, name.String()),
	}, nil
}

func (fs *rawFileSystem) exists(ctx context.Context, name common.BlobName) (bool, error) {
	_, err := os.Stat(filepath.Join(fs.path, name.String()))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (fs *rawFileSystem) delete(ctx context.Context, name common.BlobName) error {
	err := os.Remove(filepath.Join(fs.path, name.String()))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}
