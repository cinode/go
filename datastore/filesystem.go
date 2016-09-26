package datastore

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
)

const fileSystemMaxSimultaneousUploads = 0x100

var (
	errToManySimultaneousUploads = errors.New("To many simultaneous uploads")
)

type fileSystem struct {
	path string
}

// InFileSystem returns filesystem-based datastore implementation
func InFileSystem(path string) DS {
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
		return hashValidatingReader(rc, name), nil
	}

	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}

	return nil, err
}

func (fs *fileSystem) temporaryWriteStreamFileName(destName string, seqNo int) string {
	return fmt.Sprintf("%s.upload_%d", destName, seqNo)
}

func (fs *fileSystem) createTemporaryWriteStream(destName string) (*os.File, error) {
	for i := 0; i < fileSystemMaxSimultaneousUploads; i++ {
		tempName := fs.temporaryWriteStreamFileName(destName, i)
		fh, err := os.OpenFile(
			tempName,
			os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_WRONLY,
			0644,
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

func (fs *fileSystem) Save(name string, r io.ReadCloser) error {
	destName, err := fs.getFileName(name)
	if err != nil {
		// Act as if we didn't know the name is incorrect
		io.Copy(ioutil.Discard, r)
		r.Close()
		return ErrNameMismatch
	}
	_, err = fs.saveInternal(r, destName, nameCheckForSave(name))
	return err
}

func (fs *fileSystem) SaveAutoNamed(r io.ReadCloser) (string, error) {
	destName := fs.getTempName()
	return fs.saveInternal(r, destName, func(n string) bool { return true })
}

func (fs *fileSystem) saveInternal(r io.ReadCloser, destName string, nameCheck func(string) bool) (string, error) {

	defer func() {
		if r != nil {
			r.Close()
		}
	}()

	err := os.MkdirAll(filepath.Dir(destName), 0755)
	if err != nil {
		return "", err
	}

	fl, err := fs.createTemporaryWriteStream(destName)
	if err != nil {
		return "", err
	}

	defer func() {
		if fl != nil {
			fl.Close()
			os.Remove(fl.Name())
		}
	}()

	h := newHasher()
	_, err = io.Copy(fl, io.TeeReader(r, h))
	if err != nil {
		return "", err
	}

	err = r.Close()
	r = nil
	if err != nil {
		return "", err
	}

	name := h.Name()
	if !nameCheck(name) {
		return "", ErrNameMismatch
	}

	err = fl.Close()
	if err != nil {
		os.Remove(fl.Name())
		fl = nil
		return "", err
	}

	blobFileName, _ := fs.getFileName(name)
	err = os.MkdirAll(filepath.Dir(blobFileName), 0755)
	if err != nil {
		return "", err
	}

	err = os.Rename(fl.Name(), blobFileName)
	if err != nil {
		os.Remove(fl.Name())
		fl = nil
		return "", err
	}

	fl = nil
	return name, nil
}

func (fs *fileSystem) Exists(name string) (bool, error) {
	fn, err := fs.getFileName(name)
	if err != nil {
		return false, nil
	}

	fh, err := os.Open(fn)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	fh.Close()
	return true, nil
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

func (fs *fileSystem) getTempName() string {
	return fmt.Sprintf("%s/_temporary/%d.upload", fs.path, rand.Int())
}
