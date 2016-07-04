package cas

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	errToManySimultaneousUploads = errors.New("To many simultaneous uploads")
)

type fileSystem struct {
	path string
}

// InFileSystem returns filesystem-based CAS implementation
func InFileSystem(path string) CAS {
	return &fileSystem{path: path}
}

func (fs *fileSystem) Kind() string {
	return "FileSystem"
}

func (fs *fileSystem) Open(name string) (io.ReadCloser, error) {
	fn, err := fs.getFileName(name)
	if err != nil {
		return nil, err
	}

	rc, err := os.Open(fn)
	if err == nil {
		return rc, nil
	}

	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}

	return nil, err
}

type writeWrapper struct {
	fl *os.File
	d  string
	n  string
	h  *hasher
}

func (w *writeWrapper) Write(b []byte) (n int, err error) {
	w.h.Write(b)
	return w.fl.Write(b)
}

func (w *writeWrapper) Close() error {
	_ = "breakpoint"
	moved := false

	// Ensure to cleanup the mess
	defer func() {
		if !moved {
			os.Remove(w.fl.Name())
		}
	}()

	// End of write
	w.fl.Close()

	// Test if name does match
	n := w.h.Name()
	if n != w.n {
		return ErrNameMismatch
	}

	// Move to destination location
	err := os.Rename(w.fl.Name(), w.d)
	if err != nil {
		return err
	}

	moved = true
	return nil
}

func (fs *fileSystem) createTemporaryWriteStream(destName string) (*os.File, error) {
	for i := 0; i < 0x1000; i++ {
		tempName := fmt.Sprintf("%s.upload_%d", destName, i)
		fh, err := os.OpenFile(
			tempName,
			os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_WRONLY,
			0666,
		)
		if os.IsExist(err) {
			// Temporary file exists, more simultaneous uploads?
			// Try again with another temporary file name
			continue
		}

		if err != nil {
			// Some os error
			return nil, err
		}

		// Got temporary file handle
		return fh, nil
	}

	return nil, errToManySimultaneousUploads
}

type nullWriter struct {
}

func (n *nullWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (n *nullWriter) Close() error {
	return ErrNameMismatch
}

func (fs *fileSystem) Save(name string) (io.WriteCloser, error) {
	destName, err := fs.getFileName(name)
	if err != nil {
		return &nullWriter{}, nil
	}
	err = os.MkdirAll(filepath.Dir(destName), 0777)
	if err != nil {
		return nil, err
	}

	fl, err := fs.createTemporaryWriteStream(destName)
	if err != nil {
		return nil, err
	}

	return &writeWrapper{
			fl: fl,
			d:  destName,
			n:  name,
			h:  newHasher(),
		},
		nil
}

func (fs *fileSystem) Exists(name string) bool {
	fn, err := fs.getFileName(name)
	if err != nil {
		return false
	}

	fh, err := os.Open(fn)
	if err != nil {
		return false
	}
	fh.Close()
	return true
}

func (fs *fileSystem) Delete(name string) error {
	fn, err := fs.getFileName(name)
	if err != nil {
		return err
	}
	err = os.Remove(fn)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

func (fs *fileSystem) getFileName(name string) (string, error) {
	if !validateBlobName(name) {
		return "", ErrNotFound
	}
	fn := fs.path + "/" + name[0:3] + "/" + name[3:6] + "/" + name[6:]
	return fn, nil
}
