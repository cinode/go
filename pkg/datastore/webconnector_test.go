/*
Copyright © 2025 Bartłomiej Święcki (byo)

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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/datastore/testutils"
	"github.com/stretchr/testify/require"
)

func TestWebConnectorInvalidUrl(t *testing.T) {
	_, err := FromWeb("://bad.url")
	require.IsType(t, &url.Error{}, err)

	c, err := FromWeb("httpz://bad.url")
	require.NoError(t, err)

	_, err = c.Open(t.Context(), testutils.EmptyBlobNameStatic)
	require.IsType(t, &url.Error{}, err)

	_, err = c.Exists(t.Context(), testutils.EmptyBlobNameStatic)
	require.IsType(t, &url.Error{}, err)

	err = c.Delete(t.Context(), testutils.EmptyBlobNameStatic)
	require.IsType(t, &url.Error{}, err)

	err = c.Update(t.Context(), testutils.EmptyBlobNameStatic, bytes.NewBuffer(nil))
	require.IsType(t, &url.Error{}, err)
}

func TestWebConnectorInvalidContext(t *testing.T) {
	var nilCtx context.Context

	c, err := FromWeb("http://datastore.local")
	require.NoError(t, err)

	for _, name := range testutils.EmptyBlobNamesOfAllTypes {
		t.Run(fmt.Sprint(name.Type()), func(t *testing.T) {
			_, err = c.Open(nilCtx, name)
			require.ErrorContains(t, err, "nil Context")

			err = c.Update(nilCtx, name, bytes.NewReader(nil))
			require.ErrorContains(t, err, "nil Context")

			_, err = c.Exists(nilCtx, name)
			require.ErrorContains(t, err, "nil Context")

			err = c.Delete(nilCtx, name)
			require.ErrorContains(t, err, "nil Context")
		})
	}
}

func TestWebConnectorServerSideError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	c, err := FromWeb(server.URL + "/")
	require.NoError(t, err)

	for _, name := range testutils.EmptyBlobNamesOfAllTypes {
		t.Run(fmt.Sprint(name.Type()), func(t *testing.T) {
			_, err = c.Open(t.Context(), name)
			require.ErrorIs(t, err, ErrWebConnectionError)

			_, err = c.Exists(t.Context(), name)
			require.ErrorIs(t, err, ErrWebConnectionError)

			err = c.Delete(t.Context(), name)
			require.ErrorIs(t, err, ErrWebConnectionError)

			err = c.Update(t.Context(), name, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, ErrWebConnectionError)
		})
	}
}

func TestWebConnectorDetectInvalidBlobRead(t *testing.T) {
	// Test web interface and web connector
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, I should not be here!"))
	}))
	defer server.Close()

	ds2, err := FromWeb(server.URL + "/")
	require.NoError(t, err)

	for _, name := range testutils.EmptyBlobNamesOfAllTypes {
		t.Run(fmt.Sprint(name.Type()), func(t *testing.T) {
			rc, err := ds2.Open(t.Context(), name)
			if err != nil {
				// Either Open or Read could return an error
				require.ErrorIs(t, err, blobtypes.ErrValidationFailed)
				return
			}

			_, err = io.ReadAll(rc)
			require.ErrorIs(t, err, blobtypes.ErrValidationFailed)

			err = rc.Close()
			require.NoError(t, err)
		})
	}
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

	ds2, err := FromWeb(server.URL + "/")
	require.NoError(t, err)

	for _, name := range testutils.EmptyBlobNamesOfAllTypes {
		t.Run(fmt.Sprint(name.Type()), func(t *testing.T) {
			_, err = ds2.Open(t.Context(), testutils.EmptyBlobNameStatic)
			require.ErrorIs(t, err, ErrWebConnectionError)
		})
	}
}

func TestWebConnectorOptions(t *testing.T) {
	t.Run("http client", func(t *testing.T) {
		cl := &http.Client{}
		ds, err := FromWeb("http://test.local/", WebOptionHTTPClient(cl))
		require.NoError(t, err)
		require.Equal(t, cl, ds.(*webConnector).client)
	})

	t.Run("customize request", func(t *testing.T) {
		testErr := errors.New("test error")
		f := func(r *http.Request) error { return testErr }
		ds, err := FromWeb("http://test.local/", WebOptionCustomizeRequest(f))
		require.NoError(t, err)
		require.Equal(t, testErr, ds.(*webConnector).customizeRequest(nil))
	})
}
