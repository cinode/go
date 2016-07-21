package cas

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

func allCAS(f func(c CAS)) {

	func() {
		// Test raw memory stream
		f(InMemory())
	}()

	func() {
		// Test basic filesystem storage
		path, err := ioutil.TempDir("", "cinodetest")
		if err != nil {
			panic(fmt.Sprintf("Error while creating temporary directory: %s", err))
		}
		defer os.RemoveAll(path)
		f(InFileSystem(path))
	}()

	func() {
		// Test web interface and web connector
		server := httptest.NewServer(WebInterface(InMemory()))
		defer server.Close()

		f(FromWeb(server.URL+"/", &http.Client{}))

	}()

}

func TestOpenNonExisting(t *testing.T) {
	allCAS(func(c CAS) {

		s, e := c.Open("nonexistingname")
		if s != nil {
			t.Fatalf("CAS %s: Opened non-existing blob", c.Kind())
		}
		if e != ErrNotFound {
			t.Fatalf("CAS %s: Invalid error returned for non-existing blob: %s", c.Kind(), e)
		}
	})
}

func TestSaveNameMismatch(t *testing.T) {
	allCAS(func(c CAS) {
		e := c.Save("invalidname", bReader([]byte("Test"), nil, nil, nil))
		if e == nil || e != ErrNameMismatch {
			t.Fatalf("CAS %s: Didn't detect name mismatch: %v", c.Kind(), e)
		}
	})
}

func TestSaveSuccessful(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {

			if exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should not exist", c.Kind())
			}

			closeCalledCnt := 0

			rdr := bReader(b.data, func() error {
				if closeCalledCnt > 0 {
					t.Fatalf("CAS %s: Read after Close()", c.Kind())
				}
				if exists(c, b.name) {
					t.Fatalf("CAS %s: Blob should not exist before saving data", c.Kind())
				}
				return nil
			}, func() error {
				if closeCalledCnt > 0 {
					t.Fatalf("CAS %s: EOF after Close()", c.Kind())
				}
				if exists(c, b.name) {
					t.Fatalf("CAS %s: Blob should not exist before saving data", c.Kind())
				}
				return nil
			}, func() error {
				closeCalledCnt++
				return nil
			})

			e := c.Save(b.name, rdr)
			if e != nil {
				t.Fatalf("CAS %s: Couldn't write CAS data: %s", c.Kind(), e)
			}

			if closeCalledCnt == 0 {
				t.Fatalf("CAS %s: Stream was not closed", c.Kind())
			}
			if closeCalledCnt > 1 {
				t.Fatalf("CAS %s: Stream was closed multiple times (%d)", c.Kind(), closeCalledCnt)
			}

			// Reading blob
			r, e := c.Open(b.name)
			if e != nil {
				t.Fatalf("CAS %s: Couldn't open blob for reading: %s", c.Kind(), e)
			}
			d, e := ioutil.ReadAll(r)
			if e != nil {
				t.Fatalf("CAS %s: Couldn't read blob data: %s", c.Kind(), e)
			}
			if !bytes.Equal(b.data, d) {
				t.Fatalf("CAS %s: Did read invalid data", c.Kind())
			}
		}
	})
}

func TestCancelWhileSaving(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {
			errRet := errors.New("Test error")
			e := c.Save(b.name, bReader(b.data, func() error {
				return errRet
			}, nil, nil))
			if e == nil {
				t.Fatalf("CAS %s: No error returned", c.Kind())
			}
			if exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should not exist", c.Kind())
			}
		}
	})
}

func TestCancelWhileClosingSave(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {
			errRet := errors.New("Test error")
			e := c.Save(b.name, bReader(b.data, nil, nil, func() error {
				return errRet
			}))
			if e == nil {
				t.Fatalf("CAS %s: No error returned", c.Kind())
			}
			if exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should not exist", c.Kind())
			}
		}
	})
}

func TestCancelWhileSavingAutoNamed(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {
			errRet := errors.New("Test error")
			n, e := c.SaveAutoNamed(bReader(b.data, func() error {
				return errRet
			}, nil, nil))
			if e == nil {
				t.Fatalf("CAS %s: No error returned", c.Kind())
			}
			if n != "" {
				t.Fatalf("CAS %s: Should get empty name, got '%s'", c.Kind(), n)
			}
			if exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should not exist", c.Kind())
			}
		}
	})
}

func TestCancelWhileClosingAutoNamed(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {
			errRet := errors.New("Test error")
			n, e := c.SaveAutoNamed(bReader(b.data, nil, nil, func() error {
				return errRet
			}))
			if e == nil {
				t.Fatalf("CAS %s: No error returned", c.Kind())
			}
			if n != "" {
				t.Fatalf("CAS %s: Should get empty name, got '%s'", c.Kind(), n)
			}
			if exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
		}
	})
}

func TestOverwriteValidContents(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]
		putBlob(b.name, b.data, c)

		e := c.Save(b.name, bReader(b.data, func() error {
			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, func() error {
			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, nil))

		if e != nil {
			t.Fatalf("CAS %s: Couldn't save correct blob: %s", c.Kind(), e)
		}

		if !exists(c, b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		if !bytes.Equal(b.data, getBlob(b.name, c)) {
			t.Fatalf("CAS %s: Did read invalid data", c.Kind())
		}
	})
}

func TestOverwriteInvalidContents(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]
		putBlob(b.name, b.data, c)

		e := c.Save(b.name, bReader(append(b.data, []byte("extra")...), func() error {

			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, func() error {

			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, nil))

		if e != ErrNameMismatch {
			t.Fatalf("CAS %s: Saved incorrect blob: %s", c.Kind(), e)
		}

		if !exists(c, b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		if !bytes.Equal(b.data, getBlob(b.name, c)) {
			t.Fatalf("CAS %s: Did read invalid data", c.Kind())
		}
	})
}

func TestCancelWhileOverwriting(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]
		putBlob(b.name, b.data, c)

		e := c.Save(b.name, bReader(b.data, func() error {
			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return errors.New("Cancel")
		}, nil, nil))

		if e == nil {
			t.Fatalf("CAS %s: Didn't get error although cancelled", c.Kind())
		}

		if !exists(c, b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		if !bytes.Equal(b.data, getBlob(b.name, c)) {
			t.Fatalf("CAS %s: Did read invalid data", c.Kind())
		}
	})
}

func TestCancelCloseWhileOverwriting(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]
		putBlob(b.name, b.data, c)

		e := c.Save(b.name, bReader(b.data, nil, nil, func() error {
			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return errors.New("Cancel")
		}))

		if e == nil {
			t.Fatalf("CAS %s: Didn't get error although cancelled", c.Kind())
		}

		if !exists(c, b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		if !bytes.Equal(b.data, getBlob(b.name, c)) {
			t.Fatalf("CAS %s: Did read invalid data", c.Kind())
		}
	})
}

func TestOverwriteWhileDeleting(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]
		putBlob(b.name, b.data, c)

		e := c.Save(b.name, bReader(b.data, func() error {
			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, func() error {

			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}

			err := c.Delete(b.name)
			if err != nil {
				t.Fatalf("CAS %s: Couldn't delete blob: %v", c.Kind(), err)
			}

			if exists(c, b.name) {
				t.Fatalf("CAS %s: Blob should not exist", c.Kind())
			}

			return nil
		}, nil))

		if e != nil {
			t.Fatalf("CAS %s: Couldn't save correct blob: %s", c.Kind(), e)
		}

		if !exists(c, b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		if !bytes.Equal(b.data, getBlob(b.name, c)) {
			t.Fatalf("CAS %s: Did read invalid data", c.Kind())
		}
	})
}

func TestDeleteNonExisting(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]
		putBlob(b.name, b.data, c)

		err := c.Delete("non-existing")
		if err != ErrNotFound {
			t.Fatalf("CAS %s: Did not get ErrNotFound while deleting non-existing blob: %v", c.Kind(), err)
		}

	})
}

func TestDeleteExisting(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]
		putBlob(b.name, b.data, c)

		if !exists(c, b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		if !bytes.Equal(b.data, getBlob(b.name, c)) {
			t.Fatalf("CAS %s: Did read invalid data", c.Kind())
		}

		err := c.Delete(b.name)
		if err != nil {
			t.Fatalf("CAS %s: Couldn't delete blob: %v", c.Kind(), err)
		}

		if exists(c, b.name) {
			t.Fatalf("CAS %s: Blob should not exist", c.Kind())
		}

		r, err := c.Open(b.name)
		if err != ErrNotFound {
			t.Fatalf("CAS %s: Did not get ErrNotFound error after blob deletion", c.Kind())
		}
		if r != nil {
			t.Fatalf("CAS %s: Got reader for deleted blob", c.Kind())
		}

	})
}

func TestGetKind(t *testing.T) {
	allCAS(func(c CAS) {
		k := c.Kind()
		if len(k) == 0 {
			t.Fatalf("Invalid kind - empty string")
		}
	})
}

func TestSimultaneousReads(t *testing.T) {
	threadCnt := 10
	readCnt := 200

	allCAS(func(c CAS) {

		// Prepare data
		for _, b := range testBlobs {
			putBlob(b.name, b.data, c)
		}

		wg := sync.WaitGroup{}
		wg.Add(threadCnt)

		for i := 0; i < threadCnt; i++ {
			go func(i int) {
				defer wg.Done()
				for n := 0; n < readCnt; n++ {
					b := testBlobs[(i+n)%len(testBlobs)]
					if !bytes.Equal(b.data, getBlob(b.name, c)) {
						t.Fatalf("CAS %s: Did read invalid data", c.Kind())
					}
				}
			}(i)
		}

		wg.Wait()
	})
}

func TestSimultaneousSaves(t *testing.T) {
	threadCnt := 3

	allCAS(func(c CAS) {

		b := testBlobs[0]

		wg := sync.WaitGroup{}
		wg.Add(threadCnt)

		wg2 := sync.WaitGroup{}
		wg2.Add(threadCnt)

		for i := 0; i < threadCnt; i++ {
			go func(i int) {
				firstTime := true
				err := c.Save(b.name, bReader(b.data, func() error {

					if !firstTime {
						return nil
					}
					firstTime = false

					// Blob must not exist now
					if exists(c, b.name) {
						t.Fatalf("CAS %s: Blob exists although no writter finished yet", c.Kind())
					}

					// Wait for all writes to start
					wg.Done()
					wg.Wait()

					return nil

				}, nil, nil))
				errPanic(err)

				if !exists(c, b.name) {
					t.Fatalf("CAS %s: Blob does not exist yet", c.Kind())
				}

				wg2.Done()
			}(i)
		}

		wg2.Wait()
	})
}

// Invalid names behave just as if there was no blob with such name.
// Writing such blob would always fail on close (similarly to how invalid name
// when writing behaves)
var invalidNames = []string{
	"",
	"short",
	"invalid-character",
}

func TestOpenInvalidName(t *testing.T) {
	allCAS(func(c CAS) {
		for _, n := range invalidNames {
			_, e := c.Open(n)
			if e != ErrNotFound {
				t.Fatalf("CAS %s: Incorrect error for invalid name: %v", c.Kind(), e)
			}
		}
	})
}

func TestSaveInvalidName(t *testing.T) {
	allCAS(func(c CAS) {
		for _, n := range invalidNames {
			readCalled, eofCalledCnt, closeCalledCnt := false, 0, 0
			e := c.Save(n, bReader([]byte{}, func() error {
				if closeCalledCnt > 0 {
					t.Fatalf("CAS %s: Read after Close()", c.Kind())
				}
				if eofCalledCnt > 0 {
					t.Fatalf("CAS %s: Read after EOF", c.Kind())
				}
				readCalled = true
				return nil
			}, func() error {
				if closeCalledCnt > 0 {
					t.Fatalf("CAS %s: EOF after Close()", c.Kind())
				}
				eofCalledCnt++
				return nil
			}, func() error {
				closeCalledCnt++
				return nil
			}))
			if e != ErrNameMismatch {
				t.Fatalf("CAS %s: Got error when opening invalid name write stream: %v", c.Kind(), e)
			}
			if !readCalled {
				t.Fatalf("CAS %s: Didn't call read for incorrect blob name", c.Kind())
			}
			if eofCalledCnt == 0 {
				t.Fatalf("CAS %s: Didn't get EOF when reading incorrect blob data", c.Kind())
			}
			if eofCalledCnt > 1 {
				t.Fatalf("CAS %s: Did get EOF multiple times (%d)", c.Kind(), eofCalledCnt)
			}
			if closeCalledCnt == 0 {
				t.Fatalf("CAS %s: Didn't call close for incorrect blob name", c.Kind())
			}
			if closeCalledCnt > 1 {
				t.Fatalf("CAS %s: Close called multiple times for incorrect blob name", c.Kind())
			}
		}
	})
}

func TestExistsInvalidName(t *testing.T) {
	allCAS(func(c CAS) {
		for _, n := range invalidNames {
			ex, err := c.Exists(n)
			errPanic(err)
			if ex {
				t.Fatalf("CAS %s: Blob with invalid name exists", c.Kind())
			}
		}
	})
}

func TestDeleteInvalidNonExisting(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {
			e := c.Delete(b.name)
			if e != ErrNotFound {
				t.Fatalf("CAS %s: Incorrect error for invalid name: %v", c.Kind(), e)
			}
		}
	})
}

func TestDeleteInvalidName(t *testing.T) {
	allCAS(func(c CAS) {
		for _, n := range invalidNames {
			e := c.Delete(n)
			if e != ErrNotFound {
				t.Fatalf("CAS %s: Incorrect error for invalid name: %v", c.Kind(), e)
			}
		}
	})
}

func TestAutoNamedWriter(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {
			name, err := c.SaveAutoNamed(bReader(b.data, nil, nil, nil))
			if err != nil {
				t.Fatalf("CAS %s: Can not create auto named writer: %v", c.Kind(), err)
			}

			if name != b.name {
				t.Fatalf("CAS %s: Invalid name from auto named writer: "+
					"'%s' instead of '%s'", c.Kind(), name, b.name)
			}

			if !exists(c, b.name) {
				t.Fatalf("CAS %s: Blob does not exist", c.Kind())
			}
		}
	})
}
