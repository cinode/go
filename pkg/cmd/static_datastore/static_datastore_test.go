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

package static_datastore

import (
	"bytes"
	"context"
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
	dataset []datasetFile,
	datastoreDir string,
	extraArgs ...string,
) (wi *cinodefs.WriterInfo, ep *cinodefs.Entrypoint) {
	dir := s.T().TempDir()

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

	cmd := RootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(buf)

	err := cmd.Execute()
	s.Require().NoError(err)

	output := testOutputParser{}

	err = json.Unmarshal(buf.Bytes(), &output)
	s.Require().NoErrorf(err, "output: %s", buf.String())
	s.Require().Equal("OK", output.Result)

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
	ds, err := datastore.InFileSystem(datastoreDir)
	s.Require().NoError(err)

	s.validateDatasetInDatastore(dataset, ep, ds)
}

func (s *CompileAndReadTestSuite) validateDatasetInDatastore(
	dataset []datasetFile,
	ep *cinodefs.Entrypoint,
	ds datastore.DS,
) {
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
	datastoreAddress := s.T().TempDir()

	// Create and test initial dataset
	wi, ep := s.uploadDatasetToDatastore(s.initialTestDataset, datastoreAddress)
	s.validateDataset(s.initialTestDataset, ep, datastoreAddress)

	s.Run("Re-upload same dataset", func() {
		s.uploadDatasetToDatastore(s.initialTestDataset, datastoreAddress,
			"--writer-info", wi.String(),
		)
		s.validateDataset(s.initialTestDataset, ep, datastoreAddress)
	})

	s.Run("Upload modified dataset but for different root link", func() {
		_, updatedEP := s.uploadDatasetToDatastore(s.updatedTestDataset, datastoreAddress)
		s.validateDataset(s.updatedTestDataset, updatedEP, datastoreAddress)
		s.Require().NotEqual(ep, updatedEP)

		// After restoring the original entrypoint dataset should be back to the initial one
		s.validateDataset(s.initialTestDataset, ep, datastoreAddress)
	})

	s.Run("Update the original entrypoint with the new dataset", func() {
		_, epOrigWriterInfo := s.uploadDatasetToDatastore(s.updatedTestDataset, datastoreAddress,
			"--writer-info", wi.String(),
		)
		s.validateDataset(s.updatedTestDataset, epOrigWriterInfo, datastoreAddress)

		// Entrypoint must stay the same
		s.Require().EqualValues(ep, epOrigWriterInfo)
	})

	s.Run("Upload data with static entrypoint", func() {
		wiStatic, epStatic := s.uploadDatasetToDatastore(s.initialTestDataset, datastoreAddress,
			"--static",
		)
		s.validateDataset(s.initialTestDataset, epStatic, datastoreAddress)
		s.Require().Nil(wiStatic)
	})

	s.Run("Read writer info from file", func() {
		wiFile := filepath.Join(s.T().TempDir(), "epfile")
		s.Require().NoError(os.WriteFile(wiFile, []byte(wi.String()), 0777))

		_, ep := s.uploadDatasetToDatastore(s.initialTestDataset, datastoreAddress,
			"--writer-info-file", wiFile,
		)
		s.validateDataset(s.initialTestDataset, ep, datastoreAddress)
	})
}

func (s *CompileAndReadTestSuite) TestBackwardsCompatibilityForRawFileSystem() {
	dir := s.T().TempDir()

	_, ep := s.uploadDatasetToDatastore(s.initialTestDataset, dir, "--raw-filesystem")

	dsRaw, err := datastore.InRawFileSystem(dir)
	s.Require().NoError(err)
	s.validateDatasetInDatastore(s.initialTestDataset, ep, dsRaw)
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
