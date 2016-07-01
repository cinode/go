package cas

import (
	"bytes"
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
}

func TestOpenNonExisting(t *testing.T) {
	allCAS(func(c CAS) {

		s, e := c.Open("non-existing")
		if s != nil {
			t.Fatalf("CAS %s: Opened non-existing blob", c.Kind())
		}
		if e != ErrNotFound {
			t.Fatalf("CAS %s: Invalid error returned for non-existing blob: %s", c.Kind(), e)
		}
	})
}

func TestSaveInvalidName(t *testing.T) {
	allCAS(func(c CAS) {

		w, e := c.Save("invalid-name")
		if e != nil {
			t.Fatalf("CAS %s: Couldn't create CAS writer: %s", c.Kind(), e)
		}
		if w == nil {
			t.Fatalf("CAS %s: Didn't get blob writer", c.Kind())
		}

		n, e := w.Write([]byte("Test"))
		if e != nil {
			t.Fatalf("CAS %s: Couldn't write CAS data: %s", c.Kind(), e)
		}
		if n != 4 {
			t.Fatalf("CAS %s: Invalid number of bytes written: %d", c.Kind(), n)
		}

		e = w.Close()
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

			// Saving blob
			w, e := c.Save(b.name)
			if e != nil {
				t.Fatalf("CAS %s: Couldn't create CAS writer: %s", c.Kind(), e)
			}
			if w == nil {
				t.Fatalf("CAS %s: Didn't get blob writer", c.Kind())
			}
			if c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should not exist before saving data", c.Kind())
			}
			n, e := w.Write(b.data)
			if e != nil {
				t.Fatalf("CAS %s: Couldn't write CAS data: %s", c.Kind(), e)
			}
			if n != len(b.data) {
				t.Fatalf("CAS %s: Invalid number of bytes written: %d", c.Kind(), n)
			}
			if c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should not exist before stream close", c.Kind())
			}
			e = w.Close()
			if e != nil {
				t.Fatalf("CAS %s: Couldn't save correct blob: %s", c.Kind(), e)
			}
			if !c.Exists(b.name) {
				t.Fatalf("CAS %s: Blob should exist after stream close", c.Kind())
			}
			w = nil

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
	w, e := c.Save(n)
	errPanic(e)
	cnt, e := w.Write(b)
	errPanic(e)
	if cnt != len(b) {
		panic("Invalid data size written")
	}
	e = w.Close()
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

func TestOverwriteValidContents(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]
		putBlob(b.name, b.data, c)

		w, _ := c.Save(b.name)

		if !c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		w.Write(b.data)
		if !c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		e := w.Close()
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

		w, _ := c.Save(b.name)
		if !c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		w.Write(b.data)
		if !c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		w.Write([]byte("Extra"))
		e := w.Close()
		if e != ErrNameMismatch {
			t.Fatalf("CAS %s: Invalid blob save error: %s", c.Kind(), e)
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

		w, _ := c.Save(b.name)

		if !c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}

		w.Write(b.data)
		if !c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should exist", c.Kind())
		}
		e := c.Delete(b.name)
		if e != nil {
			t.Fatalf("CAS %s: Can't remove blob", c.Kind())
		}
		if c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should not exist", c.Kind())
		}

		e = w.Close()
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
