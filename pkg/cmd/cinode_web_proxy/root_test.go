/*
Copyright © 2023 Bartłomiej Święcki (byo)

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

package cinode_web_proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"github.com/cinode/go/pkg/protobuf"
	"github.com/cinode/go/pkg/structure"
	"github.com/stretchr/testify/require"
)

func TestWebProxyHandlerInvalidEntrypoint(t *testing.T) {

	n, err := common.BlobNameFromHashAndType(
		make([]byte, sha256.Size),
		blobtypes.Static,
	)
	require.NoError(t, err)

	handler := setupCinodeProxy(
		datastore.InMemory(),
		[]datastore.DS{},
		&protobuf.Entrypoint{
			BlobName: n,
			MimeType: structure.CinodeDirMimeType,
			KeyInfo: &protobuf.KeyInfo{
				Key: cipherfactory.NewKeyGenerator(blobtypes.Static).Generate(),
			},
		},
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("query invalid entrypoint", func(t *testing.T) {
		resp, err := http.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("invalid method", func(t *testing.T) {
		resp, err := http.Post(server.URL, "application/octet-stream", bytes.NewReader(nil))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

func TestWebProxyHandlerSimplePage(t *testing.T) {

	ds := datastore.InMemory()
	be := blenc.FromDatastore(ds)

	ep := func() *protobuf.Entrypoint {
		dir := t.TempDir()

		for name, content := range map[string]string{
			"index.html":     "index",
			"sub/index.html": "sub-index",
		} {
			err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
			require.NoError(t, err)
		}

		ep, err := structure.UploadStaticDirectory(context.Background(), os.DirFS(dir), be)
		require.NoError(t, err)
		return ep
	}()

	handler := setupCinodeProxy(ds, []datastore.DS{}, ep)

	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("get index", func(t *testing.T) {
		resp, err := http.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, resp.StatusCode, http.StatusOK)
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "index", string(data))
	})

	t.Run("get sub-index", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/sub")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, resp.StatusCode, http.StatusOK)
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "sub-index", string(data))
	})

	t.Run("get sub-index directly", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/sub/index.html")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, resp.StatusCode, http.StatusOK)
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "sub-index", string(data))
	})
}

func TestGetEntrypoint(t *testing.T) {
	t.Run("missing entrypoint data", func(t *testing.T) {
		ep, err := getEntrypoint()
		require.ErrorContains(t, err, "missing")
		require.Nil(t, ep)
	})

	t.Run("invalid env var", func(t *testing.T) {
		for _, d := range []struct {
			ep  string
			err string
		}{
			{"", "decode base58"},
			{"not-a-base58-string", "decode base58"},
			{"12345", "unmarshal"},
		} {
			t.Run(d.ep, func(t *testing.T) {
				t.Setenv("CINODE_ENTRYPOINT", d.ep)
				ep, err := getEntrypoint()
				require.ErrorContains(t, err, d.err)
				require.Nil(t, ep)
			})
		}
	})

	t.Run("invalid file name", func(t *testing.T) {
		t.Setenv("CINODE_ENTRYPOINT_FILE", "?invalid path?")
		ep, err := getEntrypoint()
		require.ErrorContains(t, err, "could not read")
		require.Nil(t, ep)
	})

	t.Run("invalid file content", func(t *testing.T) {
		fl := filepath.Join(t.TempDir(), "ep.txt")

		err := os.WriteFile(fl, []byte("not-a-base58-string"), 0644)
		require.NoError(t, err)

		t.Setenv("CINODE_ENTRYPOINT_FILE", fl)
		ep, err := getEntrypoint()
		require.ErrorContains(t, err, "decode base58")
		require.Nil(t, ep)
	})

	t.Run("valid entrypoint", func(t *testing.T) {
		t.Setenv("CINODE_ENTRYPOINT", "6yQKu4rx2e7CDvUH2tJTk8jJWd2BikSsrnEPT1")

		ep, err := getEntrypoint()
		require.NoError(t, err)
		require.NotNil(t, ep)

		require.Equal(t, []byte{0x01, 0x02, 0x03}, ep.BlobName)
		require.Equal(t, "test/entrypoint", ep.MimeType)
		require.Equal(t, []byte{0x04, 0x05}, ep.KeyInfo.Key)
	})
}

func TestGetMainDS(t *testing.T) {
	t.Run("Use InMemory DS when not specified", func(t *testing.T) {
		_, mainDatastoreSet := os.LookupEnv("CINODE_MAIN_DATASTORE")
		require.False(t, mainDatastoreSet)

		ds, err := getMainDS()
		require.NoError(t, err)
		require.IsType(t, datastore.InMemory(), ds)
	})

	t.Run("Create correct main DS", func(t *testing.T) {
		t.Setenv("CINODE_MAIN_DATASTORE", t.TempDir())

		ds, err := getMainDS()
		require.NoError(t, err)
		require.NotNil(t, ds)
	})
}

func TestGetAdditionalDS(t *testing.T) {
	t.Run("Check preconditions", func(t *testing.T) {
		for _, env := range os.Environ() {
			require.False(t, strings.HasPrefix(env, "CINODE_ADDITIONAL_DATASTORE_"))
		}
	})

	t.Run("No additional datastores set", func(t *testing.T) {
		ds, err := getAdditionalDSs()
		require.NoError(t, err)
		require.Empty(t, ds)
	})

	t.Run("Multiple datastores", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			t.Setenv(fmt.Sprintf("CINODE_ADDITIONAL_DATASTORE_TEST%d", i), t.TempDir())
			ds, err := getAdditionalDSs()
			require.NoError(t, err)
			require.Len(t, ds, i+1)
		}

		t.Setenv("CINODE_ADDITIONAL_DATASTORE_BOGUS", "memory://?invalid-location?")
		ds, err := getAdditionalDSs()
		require.ErrorContains(t, err, "invalid datastore location")
		require.Empty(t, ds)

	})
}
