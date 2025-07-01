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

package public_node

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cinode/go/testvectors/testblobs"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	os.Clearenv()

	t.Run("default config", func(t *testing.T) {
		cfg, err := getConfig()
		require.NoError(t, err)
		require.Equal(t, "memory://", cfg.mainDSLocation)
		require.Empty(t, cfg.additionalDSLocations)
		require.Equal(t, 8080, cfg.port)
	})

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

	t.Run("set listen port", func(t *testing.T) {
		t.Setenv("CINODE_LISTEN_PORT", "12345")
		cfg, err := getConfig()
		require.NoError(t, err)
		require.Equal(t, 12345, cfg.port)
	})

	t.Run("invalid port - not a number", func(t *testing.T) {
		t.Setenv("CINODE_LISTEN_PORT", "123-45")
		_, err := getConfig()
		require.ErrorContains(t, err, "invalid listen port")
	})

	t.Run("invalid port - outside range", func(t *testing.T) {
		t.Setenv("CINODE_LISTEN_PORT", "-1")
		_, err := getConfig()
		require.ErrorContains(t, err, "invalid listen port")
	})
}

func TestBuildHttpHandler(t *testing.T) {
	t.Run("Successfully created handler", func(t *testing.T) {
		h, err := buildHttpHandler(&config{
			mainDSLocation: t.TempDir(),
			additionalDSLocations: []string{
				t.TempDir(),
				t.TempDir(),
				t.TempDir(),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, h)

		t.Run("check the server", func(t *testing.T) {
			server := httptest.NewServer(h)
			defer server.Close()

			err := testblobs.DynamicLink.Put(server.URL)
			require.NoError(t, err)

			_, err = testblobs.DynamicLink.Get(server.URL)
			require.NoError(t, err)
		})
	})

	t.Run("Upload auth", func(t *testing.T) {

		const VALID_USERNAME = "Alice"
		const INVALID_USERNAME = "Bob"
		const VALID_PASSWORD = "secret"
		const INVALID_PASSWORD = "plaintext"

		h, err := buildHttpHandler(&config{
			mainDSLocation: t.TempDir(),
			additionalDSLocations: []string{
				t.TempDir(),
				t.TempDir(),
				t.TempDir(),
			},
			uploadUsername: VALID_USERNAME,
			uploadPassword: VALID_PASSWORD,
		})
		require.NoError(t, err)
		require.NotNil(t, h)

		server := httptest.NewServer(h)
		defer server.Close()

		err = testblobs.DynamicLink.Put(server.URL)
		require.ErrorContains(t, err, "403")

		err = testblobs.DynamicLink.PutWithAuth(server.URL, VALID_USERNAME, VALID_PASSWORD)
		require.NoError(t, err)

		err = testblobs.DynamicLink.PutWithAuth(server.URL, VALID_USERNAME, INVALID_PASSWORD)
		require.ErrorContains(t, err, "403")

		err = testblobs.DynamicLink.PutWithAuth(server.URL, INVALID_USERNAME, VALID_PASSWORD)
		require.ErrorContains(t, err, "403")

		err = testblobs.DynamicLink.PutWithAuth(server.URL, INVALID_USERNAME, INVALID_PASSWORD)
		require.ErrorContains(t, err, "403")

		_, err = testblobs.DynamicLink.Get(server.URL)
		require.NoError(t, err)
	})

	t.Run("invalid main datastore", func(t *testing.T) {
		h, err := buildHttpHandler(&config{
			mainDSLocation: "",
		})
		require.ErrorContains(t, err, "could not create main datastore")
		require.Nil(t, h)
	})

	t.Run("invalid additional datastore", func(t *testing.T) {
		h, err := buildHttpHandler(&config{
			mainDSLocation:        "memory://",
			additionalDSLocations: []string{""},
		})
		require.ErrorContains(t, err, "could not create additional datastore")
		require.Nil(t, h)
	})
}

func TestExecuteWithConfig(t *testing.T) {
	t.Run("successful run", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		err := executeWithConfig(ctx, &config{
			mainDSLocation: "memory://",
			log:            slog.Default(),
		})
		require.NoError(t, err)
	})

	t.Run("invalid configuration", func(t *testing.T) {
		err := executeWithConfig(t.Context(), &config{})
		require.ErrorContains(t, err, "datastore")
	})
}

func TestExecute(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		t.Setenv("CINODE_LISTEN_PORT", "0")

		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		err := Execute(ctx)
		require.NoError(t, err)
	})

	t.Run("invalid configuration", func(t *testing.T) {
		t.Setenv("CINODE_MAIN_DATASTORE", "memory://invalid")
		err := Execute(t.Context())
		require.ErrorContains(t, err, "datastore")
	})

	t.Run("invalid configuration - port", func(t *testing.T) {
		t.Setenv("CINODE_LISTEN_PORT", "-1")
		err := Execute(t.Context())
		require.ErrorContains(t, err, "listen port")
	})
}
