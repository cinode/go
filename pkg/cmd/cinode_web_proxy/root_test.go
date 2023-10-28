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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/cinodefs/uploader"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"github.com/cinode/go/testvectors/testblobs"
	"github.com/jbenet/go-base58"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	os.Clearenv()

	t.Run("default config", func(t *testing.T) {
		cfg, err := getConfig()
		require.ErrorContains(t, err, "ENTRYPOINT")
		require.Nil(t, cfg)
	})

	t.Run("default config with entrypoint", func(t *testing.T) {
		t.Setenv("CINODE_ENTRYPOINT", "12345")
		cfg, err := getConfig()
		require.NoError(t, err)
		require.Equal(t, "12345", cfg.entrypoint)
		require.Equal(t, "memory://", cfg.mainDSLocation)
		require.Empty(t, cfg.additionalDSLocations)
		require.Equal(t, 8080, cfg.port)
	})

	t.Run("entrypoint file", func(t *testing.T) {
		t.Run("valid", func(t *testing.T) {
			entrypointFile := filepath.Join(t.TempDir(), "ep.txt")
			err := os.WriteFile(entrypointFile, []byte("54321"), 0666)
			require.NoError(t, err)

			t.Setenv("CINODE_ENTRYPOINT_FILE", entrypointFile)
			cfg, err := getConfig()
			require.NoError(t, err)
			require.Equal(t, "54321", cfg.entrypoint)
		})
		t.Run("invalid", func(t *testing.T) {
			entrypointFile := filepath.Join(t.TempDir(), "ep.txt")
			t.Setenv("CINODE_ENTRYPOINT_FILE", entrypointFile)
			cfg, err := getConfig()
			require.ErrorContains(t, err, "read")
			require.Nil(t, cfg)
		})
	})

	t.Setenv("CINODE_ENTRYPOINT", "000000")

	t.Run("set main datastore", func(t *testing.T) {
		t.Setenv("CINODE_MAIN_DATASTORE", "testdatastore")
		cfg, err := getConfig()
		require.NoError(t, err)
		require.Equal(t, cfg.mainDSLocation, "testdatastore")
	})

	t.Run("set additional datastores", func(t *testing.T) {
		t.Setenv("CINODE_ADDITIONAL_DATASTORE", "additional")
		t.Setenv("CINODE_ADDITIONAL_DATASTORE_3", "additional3")
		t.Setenv("CINODE_ADDITIONAL_DATASTORE_2", "additional2")
		t.Setenv("CINODE_ADDITIONAL_DATASTORE_1", "additional1")

		cfg, err := getConfig()
		require.NoError(t, err)
		require.Equal(t, cfg.additionalDSLocations, []string{
			"additional",
			"additional1",
			"additional2",
			"additional3",
		})
	})
}

func TestWebProxyHandlerInvalidEntrypoint(t *testing.T) {
	n, err := common.BlobNameFromHashAndType(
		make([]byte, sha256.Size),
		blobtypes.Static,
	)
	require.NoError(t, err)

	key := cipherfactory.NewKeyGenerator(blobtypes.Static).Generate()

	handler, err := setupCinodeProxy(
		context.Background(),
		datastore.InMemory(),
		[]datastore.DS{},
		cinodefs.EntrypointFromBlobNameAndKey(n, key),
	)
	require.NoError(t, err)

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

	ep := func() *cinodefs.Entrypoint {
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

		fs, err := cinodefs.New(context.Background(), be, cinodefs.NewRootDynamicLink())
		require.NoError(t, err)

		err = uploader.UploadStaticDirectory(
			context.Background(),
			os.DirFS(dir),
			fs,
		)
		require.NoError(t, err)

		ep, err := fs.RootEntrypoint()
		require.NoError(t, err)
		return ep
	}()

	handler, err := setupCinodeProxy(context.Background(), ds, []datastore.DS{}, ep)
	require.NoError(t, err)

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

func TestExecuteWithConfig(t *testing.T) {
	t.Run("invalid main datastore", func(t *testing.T) {
		err := executeWithConfig(context.Background(), &config{
			mainDSLocation: "memory://invalid",
		})
		require.ErrorContains(t, err, "main datastore")
	})

	t.Run("invalid additional datastore", func(t *testing.T) {
		err := executeWithConfig(context.Background(), &config{
			mainDSLocation:        "memory://",
			additionalDSLocations: []string{"memory://", "memory://invalid"},
		})
		require.ErrorContains(t, err, "additional datastores")
	})

	t.Run("invalid entrypoint", func(t *testing.T) {
		err := executeWithConfig(context.Background(), &config{
			mainDSLocation: "memory://",
			entrypoint:     "!@#$",
		})
		require.ErrorContains(t, err, "could not parse")
	})

	t.Run("invalid entrypoint bytes", func(t *testing.T) {
		err := executeWithConfig(context.Background(), &config{
			mainDSLocation: "memory://",
			entrypoint:     base58.Encode([]byte("1234567890")),
		})
		require.ErrorContains(t, err, "could not parse")
	})

	t.Run("successful run", func(t *testing.T) {
		ep := testblobs.DynamicLink.Entrypoint()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := executeWithConfig(ctx, &config{
			mainDSLocation: "memory://",
			entrypoint:     ep.String(),
		})
		require.NoError(t, err)
	})
}

func TestExecute(t *testing.T) {
	os.Clearenv()

	t.Run("valid configuration", func(t *testing.T) {
		ep := testblobs.DynamicLink.Entrypoint()

		t.Setenv("CINODE_ENTRYPOINT", ep.String())
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		err := Execute(ctx)
		require.NoError(t, err)
	})

	t.Run("invalid configuration", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		err := Execute(ctx)
		require.ErrorContains(t, err, "CINODE_ENTRYPOINT")
	})
}
