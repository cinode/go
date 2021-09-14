package datastore

import (
	"io"
	"testing"
)

func TestMemoryDataManipulation(t *testing.T) {

	ds := InMemory().(*memory)

	blob := testBlobs[0]
	putBlob(blob.name, blob.data, ds)

	ds.bmap[blob.name][0]++

	r, err := ds.Open(blob.name)
	errPanic(err)

	_, err = io.ReadAll(r)
	r.Close()
	if err != ErrNameMismatch {
		t.Fatalf("Didn't detect local file manipulation, got error: %v instead of %v",
			err, ErrNameMismatch)
	}

}
