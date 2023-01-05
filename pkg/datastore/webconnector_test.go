/*
Copyright © 2022 Bartłomiej Święcki (byo)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/stretchr/testify/require"
)

func TestWebConnectorInvalidUrl(t *testing.T) {
	c := FromWeb("://bad.url")

	_, err := c.Open(context.Background(), emptyBlobNameStatic)
	require.IsType(t, &url.Error{}, err)

	_, err = c.Exists(context.Background(), emptyBlobNameStatic)
	require.IsType(t, &url.Error{}, err)

	err = c.Delete(context.Background(), emptyBlobNameStatic)
	require.IsType(t, &url.Error{}, err)

	err = c.Update(context.Background(), emptyBlobNameStatic, bytes.NewBuffer(nil))
	require.IsType(t, &url.Error{}, err)
}

func TestWebConnectorServerSideError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	c := FromWeb(server.URL + "/")

	_, err := c.Open(context.Background(), emptyBlobNameStatic)
	require.ErrorIs(t, err, ErrWebConnectionError)

	_, err = c.Exists(context.Background(), emptyBlobNameStatic)
	require.ErrorIs(t, err, ErrWebConnectionError)

	err = c.Delete(context.Background(), emptyBlobNameStatic)
	require.ErrorIs(t, err, ErrWebConnectionError)

	err = c.Update(context.Background(), emptyBlobNameStatic, bytes.NewBuffer(nil))
	require.ErrorIs(t, err, ErrWebConnectionError)
}

func TestWebConnectorDetectInvalidBlobRead(t *testing.T) {

	// Test web interface and web connector
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, I should not be here!"))
	}))
	defer server.Close()

	ds2 := FromWeb(server.URL + "/")

	rc, err := ds2.Open(context.Background(), emptyBlobNameStatic)
	require.NoError(t, err)

	_, err = io.ReadAll(rc)
	require.ErrorIs(t, err, blobtypes.ErrValidationFailed)

	err = rc.Close()
	require.NoError(t, err)

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

	ds2 := FromWeb(server.URL + "/")

	_, err := ds2.Open(context.Background(), emptyBlobNameStatic)
	require.ErrorIs(t, err, ErrWebConnectionError)
}

func TestWebConnectorOptions(t *testing.T) {
	t.Run("http client", func(t *testing.T) {
		cl := &http.Client{}
		ds := FromWeb("http://test.local/", WebOptionHttpClient(cl))
		require.Equal(t, cl, ds.(*webConnector).client)
	})

	t.Run("customize request", func(t *testing.T) {
		testErr := errors.New("test error")
		f := func(r *http.Request) error { return testErr }
		ds := FromWeb("http://test.local/", WebOptionCustomizeRequest(f))
		require.Equal(t, testErr, ds.(*webConnector).customizeRequest(nil))
	})

}
