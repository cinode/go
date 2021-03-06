package datastore

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebConnectorInvalidUrl(t *testing.T) {
	c := FromWeb("://bad.url", &http.Client{})
	_, err := c.Open("test")
	if err == nil {
		t.Fatal("Did not get error for Open")
	}
	_, err = c.Exists("test")
	if err == nil {
		t.Fatal("Did not get error for Exists")
	}
	err = c.Delete("test")
	if err == nil {
		t.Fatal("Did not get error for Delete")
	}
	err = c.Save(emptyBlobName, emptyBlobReader())
	if err == nil {
		t.Fatal("Did not get error for Delete")
	}
	_, err = c.SaveAutoNamed(emptyBlobReader())
	if err == nil {
		t.Fatal("Did not get error for Delete")
	}
}

func TestWebConnectorServerSideError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	c := FromWeb(server.URL+"/", &http.Client{})

	_, err := c.Open("test")
	if err == nil {
		t.Fatal("Did not get error for Open")
	}
	_, err = c.Exists("test")
	if err == nil {
		t.Fatal("Did not get error for Exists")
	}
	err = c.Delete("test")
	if err == nil {
		t.Fatal("Did not get error for Delete")
	}
	err = c.Save(emptyBlobName, emptyBlobReader())
	if err == nil {
		t.Fatal("Did not get error for Delete")
	}
	_, err = c.SaveAutoNamed(emptyBlobReader())
	if err == nil {
		t.Fatal("Did not get error for Delete")
	}
}

func TestWebConnectorDetectInvalidBlobRead(t *testing.T) {

	// Create memory stream without consistency check - that's to catch the
	// manipulation at the connector level, not the original datastore level
	ds := newMemoryNoConsistencyCheck()

	// Test web interface and web connector
	server := httptest.NewServer(WebInterface(ds))
	defer server.Close()

	ds2 := FromWeb(server.URL+"/", &http.Client{})

	blob := testBlobs[0]
	putBlob(blob.name, blob.data, ds)

	// Modify data
	ds.bmap[blob.name][0]++

	r, err := ds2.Open(blob.name)
	errPanic(err)

	_, err = ioutil.ReadAll(r)
	r.Close()
	if err != ErrNameMismatch {
		t.Fatalf("Didn't detect local file manipulation, got error: %v instead of %v",
			err, ErrNameMismatch)
	}

}

func TestWebConnectorDetectInvalidBlobSaveAutoNamed(t *testing.T) {

	// Create memory stream without consistency check - that's to catch the
	// manipulation at the connector level, not the original datastore level
	ds := newMemoryBrokenAutoNamed(func(n string) string {
		// Just kick out the first letter to the end of the name, it's still
		// the same length and uses valid alphabet but won't match the data
		return n[1:] + n[:1]
	})

	server := httptest.NewServer(WebInterface(ds))
	defer server.Close()

	ds2 := FromWeb(server.URL+"/", &http.Client{})

	blob := testBlobs[0]

	_, err := ds2.SaveAutoNamed(bReader(blob.data, nil, nil, nil))
	if err != ErrNameMismatch {
		t.Fatalf("Didn't detect invalid name returned from the server, error: '%v'",
			err,
		)
	}
}
