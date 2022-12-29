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
	"github.com/stretchr/testify/suite"
)

type datasetFile struct {
	fName    string
	contents string
}
type CompileAndReadTestSuite struct {
	suite.Suite

	initialTestDataset []datasetFile
	updatedTestDataset []datasetFile
}

func TestCompileAndReadTestSuite(t *testing.T) {

	s := &CompileAndReadTestSuite{
		initialTestDataset: []datasetFile{
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
		},

		updatedTestDataset: []datasetFile{
			{
				"/homefile.txt",
				"Hello home dir - updated",
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
		},
	}

	suite.Run(t, s)
}

func (s *CompileAndReadTestSuite) uploadDatasetToDatastore(
	dataset []datasetFile,
	datastoreDir string,
	wi []byte,
) []byte {

	s.T().Run("prepare dataset", func(t *testing.T) {

		dir := t.TempDir()

		for _, td := range dataset {
			err := os.MkdirAll(filepath.Join(dir, filepath.Dir(td.fName)), 0777)
			s.Require().NoError(err)

			err = os.WriteFile(filepath.Join(dir, td.fName), []byte(td.contents), 0600)
			s.Require().NoError(err)
		}

		retWi, err := compileFS(dir, datastoreDir, false, wi)
		require.NoError(t, err)
		wi = retWi
	})

	return wi
}

func (s *CompileAndReadTestSuite) validateDataset(
	dataset []datasetFile,
	datastoreDir string,
) {
	hnd, err := serverHandler(datastoreDir)
	s.Require().NoError(err)
	testServer := httptest.NewServer(hnd)
	defer testServer.Close()

	for _, td := range dataset {
		s.Run(td.fName, func() {
			res, err := http.Get(testServer.URL + td.fName)
			s.Require().NoError(err)
			defer res.Body.Close()

			data, err := io.ReadAll(res.Body)
			s.Require().NoError(err)

			s.Require().Equal([]byte(td.contents), data)

			res, err = http.Post(testServer.URL+td.fName, "plain/text", bytes.NewReader([]byte("test")))
			s.Require().NoError(err)
			defer res.Body.Close()

			s.Require().Equal(http.StatusMethodNotAllowed, res.StatusCode)

			res, err = http.Get(testServer.URL + td.fName + ".notfound")
			s.Require().NoError(err)
			defer res.Body.Close()

			s.Require().Equal(http.StatusNotFound, res.StatusCode)
		})
	}

	s.Run("Default to index.html", func() {
		res, err := http.Get(testServer.URL + "/")
		s.Require().NoError(err)
		defer res.Body.Close()

		data, err := io.ReadAll(res.Body)
		s.Require().NoError(err)

		s.Require().Equal([]byte("Index"), data)
	})
}

func (s *CompileAndReadTestSuite) TestCompileAndRead() {
	datastore := s.T().TempDir()

	// Create and test initial dataset
	wi := s.uploadDatasetToDatastore(s.initialTestDataset, datastore, nil)
	s.validateDataset(s.initialTestDataset, datastore)

	// Re-upload same dataset
	s.uploadDatasetToDatastore(s.initialTestDataset, datastore, wi)
	s.validateDataset(s.initialTestDataset, datastore)

	// Upload modified dataset but for different root link
	origEntrypoint, err := os.ReadFile(filepath.Join(datastore, "entrypoint.txt"))
	s.Require().NoError(err)

	s.uploadDatasetToDatastore(s.updatedTestDataset, datastore, nil)
	s.validateDataset(s.updatedTestDataset, datastore)

	updatedEntrypoint, err := os.ReadFile(filepath.Join(datastore, "entrypoint.txt"))
	s.Require().NoError(err)
	s.Require().NotEqualValues(origEntrypoint, updatedEntrypoint)

	// After restoring the original entrypoint dataset should be back to the initial one
	err = os.WriteFile(filepath.Join(datastore, "entrypoint.txt"), origEntrypoint, 0644)
	s.Require().NoError(err)
	s.validateDataset(s.initialTestDataset, datastore)

	// Update the original entrypoint with the new dataset
	s.uploadDatasetToDatastore(s.updatedTestDataset, datastore, wi)
	s.validateDataset(s.updatedTestDataset, datastore)

	// Entrypoint must stay the same
	checkOrigEntrypoint, err := os.ReadFile(filepath.Join(datastore, "entrypoint.txt"))
	s.Require().NoError(err)
	s.Require().EqualValues(origEntrypoint, checkOrigEntrypoint)
}
