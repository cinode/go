package cas

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
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
/*
func InFileSystem(path string) CAS {
	return &fileSystem{path: path}
}
*/

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
	fl       *os.File
	destName string
	name     string
	hasher   *hasher
	auto     bool
}

func (w *writeWrapper) Write(b []byte) (n int, err error) {
	w.hasher.Write(b)
	return w.fl.Write(b)
}

func (w *writeWrapper) Close() error {

	// Ensure to cleanup the mess
	defer func() {
		if w.fl != nil {
			os.Remove(w.fl.Name())
		}
	}()

	// End of write
	w.fl.Close()

	// Test if name does match
	n := w.hasher.Name()
	if w.auto {
		w.name = n
	} else {
		if n != w.name {
			return ErrNameMismatch
		}
	}

	// Move to destination location
	err := os.Rename(w.fl.Name(), w.destName)
	if err != nil {
		return err
	}

	w.fl = nil
	w.hasher = nil

	return nil
}

func (w *writeWrapper) Cancel() {
	if w.fl == nil {
		panic("Called Cancel multiple times or after calling Close()")
	}

	w.fl.Close()
	os.Remove(w.fl.Name())
	w.fl = nil
	w.hasher = nil
}

func (w *writeWrapper) Name() string {
	if w.fl != nil {
		panic("Called Name() with no successfull call to Close()")
	}
	if w.name == "" {
		panic("Called Name() after Cancel()")
	}
	return w.name
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

func (n *nullWriter) Cancel() {
}

func (n *nullWriter) Close() error {
	return ErrNameMismatch
}

func (fs *fileSystem) Save(name string) (io.Writer, error) {
	destName, err := fs.getFileName(name)
	if err != nil {
		return &nullWriter{}, nil
	}
	return fs.saveInternal(name, destName, false)
}

func (fs *fileSystem) SaveAutoNamed() (io.Writer, error) {
	destName, err := fs.getTempName()
	if err != nil {
		return nil, err
	}

	return fs.saveInternal("", destName, true)
}

func (fs *fileSystem) saveInternal(name, destName string, auto bool) (io.Writer, error) {
	err := os.MkdirAll(filepath.Dir(destName), 0777)
	if err != nil {
		return nil, err
	}

	fl, err := fs.createTemporaryWriteStream(destName)
	if err != nil {
		return nil, err
	}

	return &writeWrapper{
			fl:       fl,
			destName: destName,
			name:     name,
			hasher:   newHasher(),
			auto:     auto,
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

func (fs *fileSystem) getTempName() (string, error) {
	fn := fmt.Sprintf("%s/_temporary/%d.upload", fs.path, rand.Int())
	return fn, nil
}
