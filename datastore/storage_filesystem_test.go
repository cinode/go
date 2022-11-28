package datastore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func temporaryFS(t *testing.T) *fileSystem {
	return newStorageFilesystem(t.TempDir())
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
	fName := fs.getFileName(emptyBlobName, fsSuffixCurrent)
	fName = filepath.Dir(fName)
	touchFile(t, fName)

	w, err := fs.openWriteStream(context.Background(), emptyBlobName)
	require.IsType(t, &os.PathError{}, err)
	require.Nil(t, w)
}

func TestFilesystemSaveFailureTempFile(t *testing.T) {

	fs := temporaryFS(t)

	// Create blob's directory as unmodifiable
	fName := fs.getFileName(emptyBlobName, fsSuffixCurrent)
	dirPath := filepath.Dir(fName)
	err := os.MkdirAll(dirPath, 0777)
	require.NoError(t, err)
	defer protect(t, dirPath)()

	w, err := fs.openWriteStream(context.Background(), emptyBlobName)
	require.IsType(t, &os.PathError{}, err)
	require.Nil(t, w)
}

func TestFilesystemRenameFailure(t *testing.T) {

	fs := temporaryFS(t)

	// Create directory where blob should be
	fName := fs.getFileName(emptyBlobName, fsSuffixCurrent)
	os.MkdirAll(fName, 0777)

	w, err := fs.openWriteStream(context.Background(), emptyBlobName)
	require.NoError(t, err)

	err = w.Close()
	require.IsType(t, &os.LinkError{}, err)
}

func TestFilesystemDeleteFailure(t *testing.T) {
	fs := temporaryFS(t)

	// Create directory where blob should be with some file inside
	fName := fs.getFileName(emptyBlobName, fsSuffixCurrent)
	os.MkdirAll(fName, 0777)
	touchFile(t, fName+"/keep.me")

	err := fs.delete(context.Background(), emptyBlobName)
	require.IsType(t, &os.PathError{}, err)
}

func TestFilesystemDeleteNotFound(t *testing.T) {
	fs := temporaryFS(t)

	err := fs.delete(context.Background(), emptyBlobName)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestFilesystemExistsFailure(t *testing.T) {
	fs := temporaryFS(t)

	// Create blob's directory as unmodifiable
	fName := fs.getFileName(emptyBlobName, fsSuffixCurrent)
	dirPath := filepath.Dir(fName)
	err := os.MkdirAll(dirPath, 0777)
	require.NoError(t, err)
	defer protect(t, dirPath)()

	_, err = fs.exists(context.Background(), emptyBlobName)
	require.IsType(t, &os.PathError{}, err)
}
