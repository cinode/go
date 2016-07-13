package cas

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestFilesystemOpenFailure(t *testing.T) {

	tmpPath, err := ioutil.TempDir("", "cinode_filesystem_test")
	errPanic(err)
	defer os.RemoveAll(tmpPath)

	// Put file where directory should be
	err = os.MkdirAll(tmpPath+"/ZZ8/FaU/", 0777)
	errPanic(err)
	fName := tmpPath + "/ZZ8/FaU/wURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4"
	fl, err := os.Create(fName)
	errPanic(err)
	fl.Close()

	os.Chmod(fName, 0)
	defer os.Chmod(fName, 0644)

	fs := InFileSystem(tmpPath)
	stream, err := fs.Open("ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4")
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

	tmpPath, err := ioutil.TempDir("", "cinode_filesystem_test")
	errPanic(err)

	defer os.RemoveAll(tmpPath)

	// Put file where directory should be
	fl, err := os.Create(tmpPath + "/ZZ8")
	errPanic(err)
	fl.Close()

	fs := InFileSystem(tmpPath)
	err = fs.Save(
		"ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4",
		ioutil.NopCloser(bytes.NewBuffer([]byte{})))
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
}

func TestFilesystemToManySimultaneousSaves(t *testing.T) {
	threadCnt := 0x100 + 1

	tmpPath, err := ioutil.TempDir("", "cinode_filesystem_test")
	errPanic(err)
	defer os.RemoveAll(tmpPath)

	c := InFileSystem(tmpPath)

	b := testBlobs[0]

	wg := sync.WaitGroup{}
	wg.Add(threadCnt)

	wg2 := sync.WaitGroup{}
	wg2.Add(threadCnt)

	errorFound := int32(0)
	for i := 0; i < threadCnt; i++ {
		go func(i int) {
			err := c.Save(b.name, bReader(b.data, nil, func() error {
				// Wait for all writes to start
				wg.Done()
				wg.Wait()
				return errors.New("Don't finish")
			}, nil))
			if err == errToManySimultaneousUploads {
				wg.Done()
				atomic.StoreInt32(&errorFound, 1)
			}
			wg2.Done()
		}(i)
	}

	wg2.Wait()

	if errorFound == 0 {
		t.Fatalf("Did not get errToManySimultaneousUploads error")
	}

}

func TestFilesystemSaveFailureTempFile(t *testing.T) {

	tmpPath, err := ioutil.TempDir("", "cinode_filesystem_test")
	errPanic(err)

	defer os.RemoveAll(tmpPath)

	// Create blob's directory as unmodifiable
	dirPath := tmpPath + "/ZZ8/FaU/"
	err = os.MkdirAll(dirPath, 0777)
	errPanic(err)
	os.Chmod(dirPath, 0)
	defer os.Chmod(dirPath, 0755)

	fs := InFileSystem(tmpPath)
	err = fs.Save(
		"ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4",
		ioutil.NopCloser(bytes.NewBuffer([]byte{})))
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
}

func TestFilesystemSaveAutoNamedFailureTempFile(t *testing.T) {

	tmpPath, err := ioutil.TempDir("", "cinode_filesystem_test")
	errPanic(err)

	defer os.RemoveAll(tmpPath)

	// Create blob's directory as unmodifiable
	dirPath := tmpPath + "/_temporary"
	err = os.MkdirAll(dirPath, 0777)
	errPanic(err)
	os.Chmod(dirPath, 0)
	defer os.Chmod(dirPath, 0755)

	fs := InFileSystem(tmpPath)
	name, err := fs.SaveAutoNamed(ioutil.NopCloser(bytes.NewBuffer([]byte{})))
	if err == nil {
		t.Fatal("Did not get error while trying to save blob")
	}
	if name != "" {
		t.Fatal("Should get empty file name")
	}
}
