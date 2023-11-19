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

package static_datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	"golang.org/x/exp/slog"
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
		err := os.MkdirAll(filepath.Join(dir, filepath.Dir(td.fName)), 0777)
		s.Require().NoError(err)

		err = os.WriteFile(filepath.Join(dir, td.fName), []byte(td.contents), 0600)
		s.Require().NoError(err)
	}

	buf := bytes.NewBuffer(nil)

	args := []string{
		"compile",
		"-s", dir,
		"-d", datastoreDir,
	}
	args = append(args, extraArgs...)

	cmd := rootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := testOutputParser{}

	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)
	require.Equal(t, "OK", output.Result)

	if output.WI != "" {
		wi = golang.Must(cinodefs.WriterInfoFromString(output.WI))
	}
	ep = golang.Must(cinodefs.EntrypointFromString(output.EP))
	return wi, ep
}

func (s *CompileAndReadTestSuite) validateDataset(
	t *testing.T,
	dataset []datasetFile,
	ep *cinodefs.Entrypoint,
	datastoreDir string,
) {
	ds, err := datastore.InFileSystem(datastoreDir)
	s.Require().NoError(err)

	fs, err := cinodefs.New(
		context.Background(),
		blenc.FromDatastore(ds),
		cinodefs.RootEntrypoint(ep),
		cinodefs.MaxLinkRedirects(10),
	)
	s.Require().NoError(err)

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
	datastore := t.TempDir()

	// Create and test initial dataset
	wi, ep := s.uploadDatasetToDatastore(t, s.initialTestDataset, datastore)
	s.validateDataset(t, s.initialTestDataset, ep, datastore)

	t.Run("Re-upload same dataset", func(t *testing.T) {
		s.uploadDatasetToDatastore(t, s.initialTestDataset, datastore,
			"--writer-info", wi.String(),
		)
		s.validateDataset(t, s.initialTestDataset, ep, datastore)
	})

	t.Run("Upload modified dataset but for different root link", func(t *testing.T) {
		_, updatedEP := s.uploadDatasetToDatastore(t, s.updatedTestDataset, datastore)
		s.validateDataset(t, s.updatedTestDataset, updatedEP, datastore)
		s.Require().NotEqual(ep, updatedEP)

		// After restoring the original entrypoint dataset should be back to the initial one
		s.validateDataset(t, s.initialTestDataset, ep, datastore)
	})

	t.Run("Update the original entrypoint with the new dataset", func(t *testing.T) {
		_, epOrigWriterInfo := s.uploadDatasetToDatastore(t, s.updatedTestDataset, datastore,
			"--writer-info", wi.String(),
		)
		s.validateDataset(t, s.updatedTestDataset, epOrigWriterInfo, datastore)

		// Entrypoint must stay the same
		require.EqualValues(t, ep, epOrigWriterInfo)
	})

	s.T().Run("Upload data with static entrypoint", func(t *testing.T) {
		wiStatic, epStatic := s.uploadDatasetToDatastore(t, s.initialTestDataset, datastore,
			"--static",
		)
		s.validateDataset(t, s.initialTestDataset, epStatic, datastore)
		require.Nil(t, wiStatic)
	})

	s.T().Run("Read writer info from file", func(t *testing.T) {
		wiFile := filepath.Join(t.TempDir(), "epfile")
		require.NoError(t, os.WriteFile(wiFile, []byte(wi.String()), 0777))

		_, ep := s.uploadDatasetToDatastore(t, s.initialTestDataset, datastore,
			"--writer-info-file", wiFile,
		)
		s.validateDataset(t, s.initialTestDataset, ep, datastore)
	})

}

func testExecCommand(cmd *cobra.Command, args []string) (output, stderr []byte, err error) {
	outputBuff := bytes.NewBuffer(nil)
	stderrBuff := bytes.NewBuffer(nil)
	cmd.SetOutput(outputBuff)
	cmd.SetErr(stderrBuff)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outputBuff.Bytes(), stderrBuff.Bytes(), err
}

func testExec(args []string) (output, stderr []byte, err error) {
	return testExecCommand(rootCmd(), args)
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
			cmd := rootCmd()
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

	err := os.WriteFile(emptyFile, []byte{}, 0777)
	require.NoError(t, err)

	for _, d := range []struct {
		name          string
		args          []string
		errorContains string
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
