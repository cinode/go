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
	"os"
	"path/filepath"
	"testing"

	"github.com/cinode/go/pkg/datastore/testutils"
	"github.com/stretchr/testify/require"
)

func temporaryFS(t *testing.T) *fileSystem {
	ds, err := newStorageFilesystem(t.TempDir())
	require.NoError(t, err)
	return ds
}

func touchFile(t *testing.T, fName string) string {
	err := os.MkdirAll(filepath.Dir(fName), 0777)
	require.NoError(t, err)
	fl, err := os.Create(fName)
	require.NoError(t, err)
	fl.Close()
	return fName
}

func protect(t *testing.T, fName string) func() {
	fi, err := os.Stat(fName)
	require.NoError(t, err)
	os.Chmod(fName, 0)
	mode := fi.Mode()
	return func() { os.Chmod(fName, mode) }
}

func TestFilesystemKind(t *testing.T) {
	fs := temporaryFS(t)
	require.Equal(t, "FileSystem", fs.kind())
}

func TestFilesystemSaveFailureDir(t *testing.T) {
	fs := temporaryFS(t)

	// Create file at directory location preventing
	// creation of a directory
	fName := fs.getFileName(testutils.EmptyBlobNameStatic, fsSuffixCurrent)
	fName = filepath.Dir(fName)
	touchFile(t, fName)

	w, err := fs.openWriteStream(t.Context(), testutils.EmptyBlobNameStatic)
	require.IsType(t, &os.PathError{}, err)
	require.Nil(t, w)
}

func TestFilesystemSaveFailureTempFile(t *testing.T) {

	fs := temporaryFS(t)

	// Create blob's directory as unmodifiable
	fName := fs.getFileName(testutils.EmptyBlobNameStatic, fsSuffixCurrent)
	dirPath := filepath.Dir(fName)
	err := os.MkdirAll(dirPath, 0777)
	require.NoError(t, err)
	defer protect(t, dirPath)()

	w, err := fs.openWriteStream(t.Context(), testutils.EmptyBlobNameStatic)
	require.IsType(t, &os.PathError{}, err)
	require.Nil(t, w)
}

func TestFilesystemRenameFailure(t *testing.T) {

	fs := temporaryFS(t)

	// Create directory where blob should be
	fName := fs.getFileName(testutils.EmptyBlobNameStatic, fsSuffixCurrent)
	os.MkdirAll(fName, 0777)

	w, err := fs.openWriteStream(t.Context(), testutils.EmptyBlobNameStatic)
	require.NoError(t, err)

	err = w.Close()
	require.IsType(t, &os.LinkError{}, err)
}

func TestFilesystemDeleteFailure(t *testing.T) {
	fs := temporaryFS(t)

	// Create directory where blob should be with some file inside
	fName := fs.getFileName(testutils.EmptyBlobNameStatic, fsSuffixCurrent)
	os.MkdirAll(fName, 0777)
	touchFile(t, fName+"/keep.me")

	err := fs.delete(t.Context(), testutils.EmptyBlobNameStatic)
	require.IsType(t, &os.PathError{}, err)
}

func TestFilesystemDeleteNotFound(t *testing.T) {
	fs := temporaryFS(t)

	err := fs.delete(t.Context(), testutils.EmptyBlobNameStatic)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestFilesystemExistsFailure(t *testing.T) {
	fs := temporaryFS(t)

	// Create blob's directory as unmodifiable
	fName := fs.getFileName(testutils.EmptyBlobNameStatic, fsSuffixCurrent)
	dirPath := filepath.Dir(fName)
	err := os.MkdirAll(dirPath, 0777)
	require.NoError(t, err)
	defer protect(t, dirPath)()

	_, err = fs.exists(t.Context(), testutils.EmptyBlobNameStatic)
	require.IsType(t, &os.PathError{}, err)
}
