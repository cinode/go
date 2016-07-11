package cas

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"sync"
	"testing"
)

var testBlobs = []struct {
	name string
	data []byte
}{
	{"Pq2UxZQcWw2rN8iKPcteaSd4LeXYW2YphibQjmj3kUQC", []byte("Test")},
	{"TZ4M9KMpYgLEPBxvo36FR4hDpgvuoxqiu1BLzeT3xLAr", []byte("Test1")},
	{"ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4", []byte("")},
}

func allCAS(f func(c CAS)) {
	f(InMemory())
	/*
		path, err := ioutil.TempDir("", "cinodetest")
		if err != nil {
			panic(fmt.Sprintf("Error while creating temporary directory: %s", err))
		}
		defer os.RemoveAll(path)
		f(InFileSystem(path))*/
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

type helperReader struct {
	buf     io.Reader
	onRead  func() error
	onEOF   func() error
	onClose func() error
}

func bReader(b []byte, onRead func() error, onEOF func() error, onClose func() error) *helperReader {

	nop := func() error {
		return nil
	}

	if onRead == nil {
		onRead = nop
	}
	if onEOF == nil {
		onEOF = nop
	}
	if onClose == nil {
		onClose = nop
	}

	return &helperReader{
		buf:     bytes.NewReader(b),
		onRead:  onRead,
		onEOF:   onEOF,
		onClose: onClose,
	}
}

func (h *helperReader) Read(b []byte) (n int, err error) {
	err = h.onRead()
	if err != nil {
		return 0, err
	}

	n, err = h.buf.Read(b)
	if err == io.EOF {
		err = h.onEOF()
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}

	return n, err
}

func (h *helperReader) Close() error {
	return h.onClose()
}

func TestSaveNameMismatch(t *testing.T) {
	allCAS(func(c CAS) {
		e := c.Save("invalidname", bReader([]byte("Test"), nil, nil, nil))
		if e == nil || e != ErrNameMismatch {
			t.Fatalf("CAS %s: Didn't detect name mismatch: %s", c.Kind(), e)
		}
	})
}

func TestSaveSuccessful(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {

			if c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should not exist", c.Kind())
			}

			closeCalled := false

			rdr := bReader(b.data, func() error {
				if c.Exists(b.name) {
					t.Fatalf("CAS %s: Blob should not exist before saving data", c.Kind())
				}
				return nil
			}, func() error {
				if c.Exists(b.name) {
					t.Fatalf("CAS %s: Blob should not exist before saving data", c.Kind())
				}
				return nil
			}, func() error {
				closeCalled = true
				return nil
			})

			e := c.Save(b.name, rdr)
			if e != nil {
				t.Fatalf("CAS %s: Couldn't write CAS data: %s", c.Kind(), e)
			}

			if !closeCalled {
				t.Fatalf("CAS %s: Stream was not closed", c.Kind())
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

func errPanic(e error) {
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
}

func putBlob(n string, b []byte, c CAS) {
	e := c.Save(n, bReader(b, nil, nil, nil))
	errPanic(e)
	if !c.Exists(n) {
		panic("Blob does not exist: " + n)
	}
}

func getBlob(n string, c CAS) []byte {
	r, e := c.Open(n)
	errPanic(e)
	d, e := ioutil.ReadAll(r)
	errPanic(e)
	e = r.Close()
	errPanic(e)
	return d
}

func TestCancelWhileSaving(t *testing.T) {
	allCAS(func(c CAS) {
		for _, b := range testBlobs {
			errRet := errors.New("Test error")
			e := c.Save(b.name, bReader(b.data, func() error {
				return errRet
			}, nil, nil))
			if e != errRet {
				t.Fatalf("CAS %s: Incorrect error returned: %v", c.Kind(), e)
			}
			if c.Exists(b.name) {
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
			if e != errRet {
				t.Fatalf("CAS %s: Incorrect error returned: %v", c.Kind(), e)
			}
			if c.Exists(b.name) {
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
			if e != errRet {
				t.Fatalf("CAS %s: Incorrect error returned: %v", c.Kind(), e)
			}
			if n != "" {
				t.Fatalf("CAS %s: Should get empty name, got '%s'", c.Kind(), n)
			}
			if c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
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
			if e != errRet {
				t.Fatalf("CAS %s: Incorrect error returned: %v", c.Kind(), e)
			}
			if n != "" {
				t.Fatalf("CAS %s: Should get empty name, got '%s'", c.Kind(), n)
			}
			if c.Exists(b.name) {
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
			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, func() error {
			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, nil))

		if e != nil {
			t.Fatalf("CAS %s: Couldn't save correct blob: %s", c.Kind(), e)
		}

		if !c.Exists(b.name) {
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

			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, func() error {

			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, nil))

		if e != ErrNameMismatch {
			t.Fatalf("CAS %s: Saved incorrect blob: %s", c.Kind(), e)
		}

		if !c.Exists(b.name) {
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
			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return errors.New("Cancel")
		}, nil, nil))

		if e == nil {
			t.Fatalf("CAS %s: Didn't get error although cancelled", c.Kind())
		}

		if !c.Exists(b.name) {
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
			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return errors.New("Cancel")
		}))

		if e == nil {
			t.Fatalf("CAS %s: Didn't get error although cancelled", c.Kind())
		}

		if !c.Exists(b.name) {
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
			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}
			return nil
		}, func() error {

			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist", c.Kind())
			}

			err := c.Delete(b.name)
			if err != nil {
				t.Fatalf("CAS %s: Couldn't delete blob: %v", c.Kind(), err)
			}

			if c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should not exist", c.Kind())
			}

			return nil
		}, nil))

		if e != nil {
			t.Fatalf("CAS %s: Couldn't save correct blob: %s", c.Kind(), e)
		}

		if !c.Exists(b.name) {
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

		if !c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		if !bytes.Equal(b.data, getBlob(b.name, c)) {
			t.Fatalf("CAS %s: Did read invalid data", c.Kind())
		}

		err := c.Delete(b.name)
		if err != nil {
			t.Fatalf("CAS %s: Couldn't delete blob: %v", c.Kind(), err)
		}

		if c.Exists(b.name) {
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
	threadCnt := 100
	readCnt := 1000

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
			readCalled, eofCalled, closeCalled := false, false, false
			e := c.Save(n, bReader([]byte{}, func() error {
				readCalled = true
				return nil
			}, func() error {
				eofCalled = true
				return nil
			}, func() error {
				closeCalled = true
				return nil
			}))
			if e != ErrNameMismatch {
				t.Fatalf("CAS %s: Got error when opening invalid name write stream: %v", c.Kind(), e)
			}
			if !eofCalled {
				t.Fatalf("CAS %s: Didn't get EOF when reading incorrect blob data", c.Kind())
			}
			if !readCalled {
				t.Fatalf("CAS %s: Didn't call read for incorrect blob name", c.Kind())
			}
			if !closeCalled {
				t.Fatalf("CAS %s: Didn't call close for incorrect blob name", c.Kind())
			}
		}
	})
}

func TestExistsInvalidName(t *testing.T) {
	allCAS(func(c CAS) {
		for _, n := range invalidNames {
			e := c.Exists(n)
			if e {
				t.Fatalf("CAS %s: Blob with invalid name exists: %v", c.Kind(), e)
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
		}
	})
}
