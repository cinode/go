package cas

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const (
	filesystemTestBlobName = "ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4"
)

func temporaryFS() (*fileSystem, func()) {
	tmpPath, err := ioutil.TempDir("", "cinode_filesystem_test")
	errPanic(err)
	return &fileSystem{path: tmpPath},
		func() { os.RemoveAll(tmpPath) }
}

func touchFile(fName string) string {
	err := os.MkdirAll(filepath.Dir(fName), 0777)
	errPanic(err)
	fl, err := os.Create(fName)
	errPanic(err)
	fl.Close()
	return fName
}

func protect(fName string) func() {
	fi, err := os.Stat(fName)
	errPanic(err)
	os.Chmod(fName, 0)
	mode := fi.Mode()
	return func() { os.Chmod(fName, mode) }
}

func testData() io.ReadCloser {
	return ioutil.NopCloser(bytes.NewBuffer([]byte{}))
}

func TestFilesystemOpenFailure(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	// Put file where directory should be
	fName, err := fs.getFileName(filesystemTestBlobName)
	errPanic(err)
	defer protect(touchFile(fName))()

	stream, err := fs.Open(filesystemTestBlobName)
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
	if err == ErrNotFound {
		t.Fatal("Incorrect error code, file exists but is unreadable")
	}
	if stream != nil {
		t.Fatal("Got non-nil stream along with error")
	}
}

func TestFilesystemSaveFailureDir(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	// Put file where directory should be
	fName, err := fs.getFileName(filesystemTestBlobName)
	errPanic(err)
	fName = filepath.Dir(fName)
	defer protect(touchFile(fName))()

	err = fs.Save(filesystemTestBlobName, testData())
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
	if err == ErrNotFound || err == ErrNameMismatch {
		t.Fatalf("Incorrect error received: %v", err)
	}
}

func TestFilesystemToManySimultaneousSaves(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	fName, err := fs.getFileName(filesystemTestBlobName)
	errPanic(err)
	for i := 0; i < fileSystemMaxSimultaneousUploads; i++ {
		touchFile(fs.temporaryWriteStreamFileName(fName, i))
	}
	err = fs.Save(filesystemTestBlobName, testData())
	if err != errToManySimultaneousUploads {
		t.Fatalf("Incorrect error received: %v", err)
	}

}

func TestFilesystemSaveFailureTempFile(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	// Create blob's directory as unmodifiable
	fName, err := fs.getFileName(filesystemTestBlobName)
	errPanic(err)
	dirPath := filepath.Dir(fName)
	errPanic(os.MkdirAll(dirPath, 0777))
	defer protect(dirPath)()

	err = fs.Save(filesystemTestBlobName, testData())
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
	if err == ErrNotFound || err == ErrNameMismatch {
		t.Fatalf("Incorrect error received: %v", err)
	}
}

func TestFilesystemSaveAutoNamedFailureTempFile(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	// Create blob's directory as unmodifiable
	dirPath := fs.path + "/_temporary"
	errPanic(os.MkdirAll(dirPath, 0777))
	defer protect(dirPath)()

	name, err := fs.SaveAutoNamed(testData())
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
	if err == ErrNotFound || err == ErrNameMismatch {
		t.Fatalf("Incorrect error received: %v", err)
	}
	if name != "" {
		t.Fatal("Should get empty file name")
	}
}

func TestFilesystemRenameFailure(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	// Create directory where blob should be
	fName, err := fs.getFileName(filesystemTestBlobName)
	os.MkdirAll(fName, 0777)

	err = fs.Save(filesystemTestBlobName, testData())
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
	if err == ErrNotFound || err == ErrNameMismatch {
		t.Fatalf("Incorrect error received: %v", err)
	}
}

func TestFilesystemDeleteFailure(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	// Create directory where blob should be with some file inside
	fName, err := fs.getFileName(filesystemTestBlobName)
	os.MkdirAll(fName, 0777)
	touchFile(fName + "/keep.me")

	err = fs.Delete(filesystemTestBlobName)
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
	if err == ErrNotFound || err == ErrNameMismatch {
		t.Fatalf("Incorrect error received: %v", err)
	}
}

func TestFilesystemDeleteNotFound(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	err := fs.Delete(filesystemTestBlobName)
	if err != ErrNotFound {
		t.Fatalf("Incorrect error received: %v", err)
	}
}

func TestFilesystemExistsFailure(t *testing.T) {

	fs, d := temporaryFS()
	defer d()

	// Create blob's directory as unmodifiable
	fName, err := fs.getFileName(filesystemTestBlobName)
	errPanic(err)
	dirPath := filepath.Dir(fName)
	errPanic(os.MkdirAll(dirPath, 0777))
	defer protect(dirPath)()

	_, err = fs.Exists(filesystemTestBlobName)
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
	if err == ErrNotFound || err == ErrNameMismatch {
		t.Fatalf("Incorrect error received: %v", err)
	}
}
