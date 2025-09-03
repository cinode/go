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

package staticdatastore

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/cinodefs/httphandler"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/spf13/cobra"
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

type testOutputParser struct {
	Result string `json:"result"`
	Msg    string `json:"msg"`
	WI     string `json:"writer-info"`
	EP     string `json:"entrypoint"`
}

func (s *CompileAndReadTestSuite) uploadDatasetToDatastore(
	t *testing.T,
	dataset []datasetFile,
	datastoreDir string,
	extraArgs ...string,
) (wi *cinodefs.WriterInfo, ep *cinodefs.Entrypoint) {
	dir := t.TempDir()

	for _, td := range dataset {
		err := os.MkdirAll(filepath.Join(dir, filepath.Dir(td.fName)), 0o777)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dir, td.fName), []byte(td.contents), 0o600)
		require.NoError(t, err)
	}

	buf := bytes.NewBuffer(nil)

	args := []string{
		"compile",
		"-s", dir,
		"-d", datastoreDir,
	}
	args = append(args, extraArgs...)

	cmd := RootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := testOutputParser{}

	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoErrorf(t, err, "output: %s", buf.String())
	require.Equal(t, "OK", output.Result)

	if output.WI != "" {
		wi = golang.Must(cinodefs.WriterInfoFromString(output.WI))
	}
	ep = golang.Must(cinodefs.EntrypointFromString(output.EP))
	return wi, ep
}

func (s *CompileAndReadTestSuite) validateDataset(
	dataset []datasetFile,
	ep *cinodefs.Entrypoint,
	datastoreDir string,
) {
	t := s.T()

	ds, err := datastore.InFileSystem(datastoreDir)
	require.NoError(t, err)

	s.validateDatasetInDatastore(t, dataset, ep, ds)
}

func (s *CompileAndReadTestSuite) validateDatasetInDatastore(
	t *testing.T,
	dataset []datasetFile,
	ep *cinodefs.Entrypoint,
	ds datastore.DS,
) {
	fs, err := cinodefs.New(
		t.Context(),
		blenc.FromDatastore(ds),
		cinodefs.RootEntrypoint(ep),
		cinodefs.MaxLinkRedirects(10),
	)
	require.NoError(t, err)

	testServer := httptest.NewServer(&httphandler.Handler{
		FS:        fs,
		IndexFile: "index.html",
		Log:       slog.Default(),
	})
	defer testServer.Close()

	for _, td := range dataset {
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
}

func (s *CompileAndReadTestSuite) TestCompileAndRead() {
	t := s.T()

	datastoreAddress := t.TempDir()

	// Create and test initial dataset
	wi, ep := s.uploadDatasetToDatastore(t, s.initialTestDataset, datastoreAddress)
	s.validateDataset(s.initialTestDataset, ep, datastoreAddress)

	t.Run("Re-upload same dataset", func(t *testing.T) {
		s.uploadDatasetToDatastore(t, s.initialTestDataset, datastoreAddress,
			"--writer-info", wi.String(),
		)
		s.validateDataset(s.initialTestDataset, ep, datastoreAddress)
	})

	t.Run("Upload modified dataset but for different root link", func(t *testing.T) {
		_, updatedEP := s.uploadDatasetToDatastore(t, s.updatedTestDataset, datastoreAddress)
		s.validateDataset(s.updatedTestDataset, updatedEP, datastoreAddress)
		require.NotEqual(t, ep, updatedEP)

		// After restoring the original entrypoint dataset should be back to the initial one
		s.validateDataset(s.initialTestDataset, ep, datastoreAddress)
	})

	t.Run("Update the original entrypoint with the new dataset", func(t *testing.T) {
		_, epOrigWriterInfo := s.uploadDatasetToDatastore(t, s.updatedTestDataset, datastoreAddress,
			"--writer-info", wi.String(),
		)
		s.validateDataset(s.updatedTestDataset, epOrigWriterInfo, datastoreAddress)

		// Entrypoint must stay the same
		require.EqualValues(t, ep, epOrigWriterInfo)
	})

	t.Run("Upload data with static entrypoint", func(t *testing.T) {
		wiStatic, epStatic := s.uploadDatasetToDatastore(t, s.initialTestDataset, datastoreAddress,
			"--static",
		)
		s.validateDataset(s.initialTestDataset, epStatic, datastoreAddress)
		require.Nil(t, wiStatic)
	})

	t.Run("Read writer info from file", func(t *testing.T) {
		wiFile := filepath.Join(t.TempDir(), "epfile")
		require.NoError(t, os.WriteFile(wiFile, []byte(wi.String()), 0o777))

		_, ep := s.uploadDatasetToDatastore(t, s.initialTestDataset, datastoreAddress,
			"--writer-info-file", wiFile,
		)
		s.validateDataset(s.initialTestDataset, ep, datastoreAddress)
	})

	t.Run("Generate index file", func(t *testing.T) {
		dir := t.TempDir()
		_, ep := s.uploadDatasetToDatastore(t, s.initialTestDataset, dir,
			"--generate-index-files",
			"--index-file", "homefile.txt",
		)
		s.validateDataset(s.initialTestDataset, ep, dir)

		ds, err := datastore.InFileSystem(dir)
		require.NoError(t, err)

		fs, err := cinodefs.New(
			t.Context(),
			blenc.FromDatastore(ds),
			cinodefs.RootEntrypoint(ep),
		)
		require.NoError(t, err)

		rc, err := fs.OpenEntryData(t.Context(), []string{"subpath", "homefile.txt"})
		require.NoError(t, err)
		defer rc.Close()

		data, err := io.ReadAll(rc)
		require.NoError(t, err)

		dataStr := string(data)
		require.Contains(t, dataStr, "file.txt")
		require.Contains(t, dataStr, "file2.txt")
	})
}

func (s *CompileAndReadTestSuite) TestBackwardsCompatibilityForRawFileSystem() {
	t := s.T()

	dir := t.TempDir()

	_, ep := s.uploadDatasetToDatastore(t, s.initialTestDataset, dir, "--raw-filesystem")

	dsRaw, err := datastore.InRawFileSystem(dir)
	require.NoError(t, err)
	s.validateDatasetInDatastore(t, s.initialTestDataset, ep, dsRaw)
}

func testExecCommand(cmd *cobra.Command, args []string) (output, stderr []byte, err error) {
	outputBuff := bytes.NewBuffer(nil)
	stderrBuff := bytes.NewBuffer(nil)
	cmd.SetOut(outputBuff)
	cmd.SetErr(stderrBuff)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outputBuff.Bytes(), stderrBuff.Bytes(), err
}

func testExec(args []string) (output, stderr []byte, err error) {
	return testExecCommand(RootCmd(), args)
}

func TestHelpCalls(t *testing.T) {
	for _, d := range []struct {
		name string
		args []string
	}{
		{"no args", []string{}},
		{"not enough compile args", []string{"compile"}},
	} {
		t.Run(d.name, func(t *testing.T) {
			cmd := RootCmd()
			helpCalled := false
			cmd.SetHelpFunc(func(c *cobra.Command, s []string) { helpCalled = true })
			cmd.SetArgs(d.args)
			err := cmd.Execute()
			require.NoError(t, err)
			require.True(t, helpCalled)
		})
	}
}

func TestInvalidOptions(t *testing.T) {
	tempDir := t.TempDir()
	emptyFile := filepath.Join(tempDir, "empty")

	err := os.WriteFile(emptyFile, []byte{}, 0o777)
	require.NoError(t, err)

	for _, d := range []struct {
		name          string
		errorContains string
		args          []string
	}{
		{
			name: "invalid root writer info",
			args: []string{
				"compile",
				"--source", t.TempDir(),
				"--destination", t.TempDir(),
				"--writer-info", "not-a-valid-writer-info",
			},
			errorContains: "Couldn't parse writer info:",
		},
		{
			name: "invalid root writer info file",
			args: []string{
				"compile",
				"--source", t.TempDir(),
				"--destination", t.TempDir(),
				"--writer-info-file", "/invalid/file/name/with/writer/info",
			},
			errorContains: "no such file or directory",
		},
		{
			name: "empty root writer info file",
			args: []string{
				"compile",
				"--source", t.TempDir(),
				"--destination", t.TempDir(),
				"--writer-info-file", emptyFile,
			},
			errorContains: "is empty",
		},
	} {
		t.Run(d.name, func(t *testing.T) {
			output, _, err := testExec(d.args)
			require.ErrorContains(t, err, d.errorContains)

			parsedOutput := testOutputParser{}
			err = json.Unmarshal(output, &parsedOutput)
			require.NoError(t, err)
			require.Equal(t, "ERROR", parsedOutput.Result)
			require.Contains(t, parsedOutput.Msg, d.errorContains)
		})
	}
}
