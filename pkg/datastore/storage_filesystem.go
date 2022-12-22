/*
Copyright © 2022 Bartłomiej Święcki (byo)

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
	"io"
	"os"
	"path/filepath"

	"github.com/cinode/go/pkg/common"
)

const (
	fsSuffixCurrent = ".c"
	fsSuffixUpload  = ".u"
)

type fileSystem struct {
	path string
}

func newStorageFilesystem(path string) *fileSystem {
	return &fileSystem{
		path: path,
	}
}

func (fs *fileSystem) kind() string {
	return "FileSystem"
}
func (fs *fileSystem) openReadStream(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	rc, err := os.Open(fs.getFileName(name, fsSuffixCurrent))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	return rc, err
}

func (fs *fileSystem) createTemporaryWriteStream(name common.BlobName) (*os.File, error) {
	tempName := fs.getFileName(name, fsSuffixUpload)

	// Ensure dir exists
	err := os.MkdirAll(filepath.Dir(tempName), 0755)
	if err != nil {
		return nil, err
	}

	// Open file in exclusive mode, allow only a single upload at a time
	fh, err := os.OpenFile(
		tempName,
		os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if os.IsExist(err) {
		return nil, ErrUploadInProgress
	}

	if err != nil {
		// Some OS error
		return nil, err
	}

	// Got temporary file handle
	return fh, nil
}

type fileSystemWriteCloser struct {
	fs       *os.File
	destName string
}

func (w *fileSystemWriteCloser) Write(b []byte) (int, error) {
	return w.fs.Write(b)
}

func (w *fileSystemWriteCloser) Cancel() {
	w.fs.Close()
	os.Remove(w.fs.Name())
}

func (w *fileSystemWriteCloser) Close() error {
	err := w.fs.Close()
	if err != nil {
		// This is not covered by tests, I have no idea how to trigger that
		os.Remove(w.fs.Name())
		return err
	}

	return os.Rename(w.fs.Name(), w.destName)
}

func (fs *fileSystem) openWriteStream(ctx context.Context, name common.BlobName) (WriteCloseCanceller, error) {

	fl, err := fs.createTemporaryWriteStream(name)
	if err != nil {
		return nil, err
	}

	return &fileSystemWriteCloser{
		fs:       fl,
		destName: fs.getFileName(name, fsSuffixCurrent),
	}, nil
}

func (fs *fileSystem) exists(ctx context.Context, name common.BlobName) (bool, error) {
	_, err := os.Stat(fs.getFileName(name, fsSuffixCurrent))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (fs *fileSystem) delete(ctx context.Context, name common.BlobName) error {
	err := os.Remove(fs.getFileName(name, fsSuffixCurrent))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

func (fs *fileSystem) getFileName(name common.BlobName, suffix string) string {
	fNameParts := []string{fs.path}

	nameStr := name.String()
	for i := 0; i < 3; i++ {
		if len(nameStr) > 3 {
			fNameParts = append(fNameParts, nameStr[:3])
			nameStr = nameStr[3:]
		}
	}
	fNameParts = append(fNameParts, nameStr+suffix)

	return filepath.Join(fNameParts...)
}
