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

package cinode_web_proxy_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/cinodefs/uploader"
	"github.com/cinode/go/pkg/cmd/cinode_web_proxy"
	"github.com/cinode/go/pkg/datastore"
	"github.com/stretchr/testify/require"
)

func TestIntegration(t *testing.T) {
	os.Clearenv()

	// Prepare test filesystem
	testFS := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("Hello world!"),
		},
		"test.txt": &fstest.MapFile{
			Data: []byte("test.txt"),
		},
		"internal/folder/file.txt": &fstest.MapFile{
			Data: []byte("internal folder file"),
		},
	}

	for i := 0; i < 20; i++ {
		testFS[fmt.Sprintf("batch/file_%d", i)] = &fstest.MapFile{
			Data: []byte(fmt.Sprintf("data_%d", i)),
		}
	}

	// Compile encrypted datastore

	dir := t.TempDir()
	ds, err := datastore.InRawFileSystem(dir)
	require.NoError(t, err)

	cfs, err := cinodefs.New(
		t.Context(),
		blenc.FromDatastore(ds),
		cinodefs.NewRootStaticDirectory(),
	)
	require.NoError(t, err)

	err = uploader.UploadStaticDirectory(
		t.Context(),
		testFS,
		cfs,
	)
	require.NoError(t, err)

	err = cfs.Flush(t.Context())
	require.NoError(t, err)

	ep, err := cfs.RootEntrypoint()
	require.NoError(t, err)

	t.Setenv("CINODE_ENTRYPOINT", ep.String())

	runAndValidateCinodeProxy := func() {
		ctx, cancel := context.WithCancel(t.Context())

		// Run the server in the background
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			cinode_web_proxy.Execute(ctx)
		}()
		time.Sleep(time.Millisecond) // Wait for the server, TODO: This is ugly way to do this

		// Ensure we clean up cleanly
		defer func() {
			cancel()
			wg.Wait()
		}()

		// Validate content of all files
		for name, file := range testFS {
			t.Run(name, func(t *testing.T) {
				resp, err := http.Get("http://localhost:8080/" + name)
				require.NoError(t, err)
				defer resp.Body.Close()

				data, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, file.Data, data)
			})
		}
	}

	t.Run("main datastore from compiled files", func(t *testing.T) {
		t.Setenv("CINODE_MAIN_DATASTORE", "file-raw://"+dir)
		runAndValidateCinodeProxy()
	})

	t.Run("additional datastore from compiled files", func(t *testing.T) {
		t.Setenv("CINODE_ADDITIONAL_DATASTORE", "file-raw://"+dir)
		runAndValidateCinodeProxy()
	})

	t.Run("use multiple datastores", func(t *testing.T) {
		// the blobstore is split to smaller folders, each one of them containing
		// some subset of blobs
		partialDirs := []string{
			t.TempDir(),
			t.TempDir(),
			t.TempDir(),
		}

		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		for i, fl := range files {
			partialDir := partialDirs[i%len(partialDirs)]

			require.False(t, fl.IsDir())
			require.True(t, fl.Type().IsRegular())

			data, err := os.ReadFile(filepath.Join(dir, fl.Name()))
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(partialDir, fl.Name()), data, 0666)
			require.NoError(t, err)
		}

		for i, partialDir := range partialDirs {
			t.Setenv(fmt.Sprintf("CINODE_ADDITIONAL_DATASTORE_%d", i), "file-raw://"+partialDir)
		}
		runAndValidateCinodeProxy()
	})
}
