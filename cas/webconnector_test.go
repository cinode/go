package cas

import (
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
