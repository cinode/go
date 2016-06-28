package cas

import (
	"bytes"
	"io/ioutil"
	"testing"
)

var testBlobs = []struct {
	name string
	data []byte
}{
	{"s6bi7Tmf1itJ5o3vuASiMPDpY69tDkpUjDfe5srJHEX9v", []byte("Test")},
	{"sAKjyeXcDkdbTp7BWZruxDqthaCHb4kmdQxE28j2HSNva", []byte("Test1")},
	{"sGKot5hBsd81kMupNCXHaqbhv3huEbxAFMLnpcX2hniwn", []byte("")},
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

func TestResave(t *testing.T) {
	allCAS(func(c CAS) {

		b := testBlobs[0]

		if c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should not exist", c.Kind())
		}

		// Write blob once
		func() {
			w, _ := c.Save(b.name)
			w.Write(b.data)
			e := w.Close()
			if e != nil {
				t.Fatalf("CAS %s: Couldn't save correct blob: %s", c.Kind(), e)
			}
		}()

		if !c.Exists(b.name) {
			t.Fatalf("CAS %s: Blob should not exist", c.Kind())
		}

		// Overwrite blob
		func() {
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

		}()

		// Overwrite with malformed data
		func() {
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

			r, _ := c.Open(b.name)
			d, _ := ioutil.ReadAll(r)
			if !bytes.Equal(b.data, d) {
				t.Fatalf("CAS %s: Did read invalid data", c.Kind())
			}
		}()

		// Overwrite blob with deletion inside
		func() {
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

		}()

	})
}
