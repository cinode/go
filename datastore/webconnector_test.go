package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cinode/go/internal/blobtypes"
	"github.com/stretchr/testify/require"
)

func TestWebConnectorInvalidUrl(t *testing.T) {
	c := FromWeb("://bad.url", &http.Client{})

	err := c.Read(context.Background(), emptyBlobName, bytes.NewBuffer(nil))
	require.IsType(t, &url.Error{}, err)

	_, err = c.Exists(context.Background(), emptyBlobName)
	require.IsType(t, &url.Error{}, err)

	err = c.Delete(context.Background(), emptyBlobName)
	require.IsType(t, &url.Error{}, err)

	err = c.Update(context.Background(), emptyBlobName, bytes.NewBuffer(nil))
	require.IsType(t, &url.Error{}, err)
}

func TestWebConnectorServerSideError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	c := FromWeb(server.URL+"/", &http.Client{})

	err := c.Read(context.Background(), emptyBlobName, bytes.NewBuffer(nil))
	require.ErrorIs(t, err, ErrWebConnectionError)

	_, err = c.Exists(context.Background(), emptyBlobName)
	require.ErrorIs(t, err, ErrWebConnectionError)

	err = c.Delete(context.Background(), emptyBlobName)
	require.ErrorIs(t, err, ErrWebConnectionError)

	err = c.Update(context.Background(), emptyBlobName, bytes.NewBuffer(nil))
	require.ErrorIs(t, err, ErrWebConnectionError)
}

func TestWebConnectorDetectInvalidBlobRead(t *testing.T) {

	// Test web interface and web connector
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, I should not be here!"))
	}))
	defer server.Close()

	ds2 := FromWeb(server.URL+"/", &http.Client{})

	data := bytes.NewBuffer(nil)
	err := ds2.Read(context.Background(), emptyBlobName, data)
	require.ErrorIs(t, err, blobtypes.ErrValidationFailed)

}

func TestWebConnectorInvalidErrorCode(t *testing.T) {

	// Test web interface and web connector
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&webErrResponse{
			Code: "SOME_UNKNOWN_CODE",
		})
	}))
	defer server.Close()

	ds2 := FromWeb(server.URL+"/", &http.Client{})

	data := bytes.NewBuffer(nil)
	err := ds2.Read(context.Background(), emptyBlobName, data)
	require.ErrorIs(t, err, ErrWebConnectionError)
}
