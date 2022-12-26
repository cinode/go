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

package static_datastore

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompileAndRead(t *testing.T) {

	testDataset := []struct {
		fName    string
		contents string
	}{
		{
			"/homefile.txt",
			"Hello home dir",
		},
		{
			"/subpath/file.txt",
			"File in subpath",
		},
		{
			"/subpath/file2.txt",
			"Another file in subpath",
		},
		{
			"/some/other/nested/path/file.txt",
			"Deeply nested file",
		},
		{
			"/index.html",
			"Index",
		},
	}

	datastore := t.TempDir()

	t.Run("Create new datastore", func(t *testing.T) {
		dir := t.TempDir()

		for _, td := range testDataset {
			err := os.MkdirAll(filepath.Join(dir, filepath.Dir(td.fName)), 0777)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(dir, td.fName), []byte(td.contents), 0600)
			require.NoError(t, err)
		}

		compile(dir, datastore)
	})

	t.Run("examine the datastore", func(t *testing.T) {
		hnd, err := serverHandler(datastore)
		require.NoError(t, err)
		testServer := httptest.NewServer(hnd)
		defer testServer.Close()

		for _, td := range testDataset {
			t.Run(td.fName, func(t *testing.T) {
				res, err := http.Get(testServer.URL + td.fName)
				require.NoError(t, err)
				defer res.Body.Close()

				data, err := io.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, []byte(td.contents), data)

				res, err = http.Post(testServer.URL+td.fName, "plain/text", bytes.NewReader([]byte("test")))
				require.NoError(t, err)
				defer res.Body.Close()

				require.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)

				res, err = http.Get(testServer.URL + td.fName + ".notfound")
				require.NoError(t, err)
				defer res.Body.Close()

				require.Equal(t, http.StatusNotFound, res.StatusCode)
			})
		}

		t.Run("Default to index.html", func(t *testing.T) {
			res, err := http.Get(testServer.URL + "/")
			require.NoError(t, err)
			defer res.Body.Close()

			data, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			require.Equal(t, []byte("Index"), data)
		})

	})

}