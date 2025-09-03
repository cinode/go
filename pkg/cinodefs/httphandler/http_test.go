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

package httphandler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type mockDatastore struct {
	datastore.DS
	openFunc func(ctx context.Context, name *common.BlobName) (io.ReadCloser, error)
}

func (m *mockDatastore) Open(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
	if m.openFunc != nil {
		return m.openFunc(ctx, name)
	}
	return m.DS.Open(ctx, name)
}

type HandlerTestSuite struct {
	suite.Suite

	ds      mockDatastore
	fs      cinodefs.FS
	handler *Handler
	server  *httptest.Server
	logData *bytes.Buffer
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, &HandlerTestSuite{})
}

func (s *HandlerTestSuite) SetupTest() {
	t := s.T()

	s.ds = mockDatastore{DS: datastore.InMemory()}
	fs, err := cinodefs.New(
		t.Context(),
		blenc.FromDatastore(&s.ds),
		cinodefs.NewRootStaticDirectory(),
	)
	require.NoError(t, err)
	s.fs = fs

	s.logData = bytes.NewBuffer(nil)
	log := slog.New(slog.NewJSONHandler(
		s.logData,
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	s.handler = &Handler{
		FS:        fs,
		IndexFile: "index.html",
		Log:       log,
	}
	s.server = httptest.NewServer(s.handler)
	s.T().Cleanup(s.server.Close)
}

func (s *HandlerTestSuite) setEntry(t *testing.T, data string, path ...string) {
	_, err := s.fs.SetEntryFile(
		t.Context(),
		path,
		strings.NewReader(data),
	)
	require.NoError(t, err)
}

func (s *HandlerTestSuite) getEntryETag(
	t *testing.T,
	path string,
	etag string,
) (
	data string,
	contentType string,
	respEtag string,
	code int,
) {
	req, err := http.NewRequest(http.MethodGet, s.server.URL+path, http.NoBody)
	require.NoError(t, err)

	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyData, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(bodyData), resp.Header.Get("content-type"), resp.Header.Get("ETag"), resp.StatusCode
}

func (s *HandlerTestSuite) getEntry(path string) (
	data string,
	contentType string,
	code int,
) {
	data, contentType, _, code = s.getEntryETag(s.T(), path, "")
	return data, contentType, code
}

func (s *HandlerTestSuite) getData(t *testing.T, path string) string {
	data, _, code := s.getEntry(path)
	require.Equal(t, http.StatusOK, code)
	return data
}

func (s *HandlerTestSuite) TestSuccessfulFileDownload() {
	t := s.T()

	s.setEntry(t, "hello", "file.txt")
	readBack := s.getData(t, "/file.txt")
	require.Equal(t, "hello", readBack)
}

func (s *HandlerTestSuite) TestEtag() {
	t := s.T()

	s.setEntry(t, "hello", "file.txt")

	readBack, _, etag, code := s.getEntryETag(t, "/file.txt", "")
	require.NotEmpty(t, etag)
	require.Greater(t, len(etag), 10)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "hello", readBack)

	readBack, _, _, code = s.getEntryETag(t, "/file.txt", etag)
	require.Equal(t, http.StatusNotModified, code)
	require.Empty(t, readBack)

	s.setEntry(t, "updated", "file.txt")

	readBack, _, etag2, code := s.getEntryETag(t, "/file.txt", etag)
	require.Equal(t, http.StatusOK, code)
	require.Greater(t, len(etag2), 10)
	require.NotEqual(t, etag, etag2)
	require.Equal(t, "updated", readBack)
}

func (s *HandlerTestSuite) TestNonGetRequest() {
	t := s.T()

	resp, err := http.Post(s.server.URL, "text/plain", strings.NewReader("Hello world!"))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func (s *HandlerTestSuite) TestNotFound() {
	t := s.T()

	_, err := s.fs.SetEntryFile(t.Context(), []string{"hello.txt"}, strings.NewReader("hello"))
	require.NoError(t, err)

	_, _, code := s.getEntry("/no-hello.txt")
	require.Equal(t, http.StatusNotFound, code)

	_, _, code = s.getEntry("/hello.txt/world")
	require.Equal(t, http.StatusNotFound, code)
}

func (s *HandlerTestSuite) TestReadIndexFile() {
	t := s.T()

	s.setEntry(t, "hello", "dir", "index.html")

	// Repeat twice, once before and once after flush
	for i := 0; i < 2; i++ {
		readBack := s.getData(t, "/dir")
		require.Equal(t, "hello", readBack)

		err := s.fs.Flush(t.Context())
		require.NoError(t, err)
	}
}

func (s *HandlerTestSuite) TestReadErrors() {
	t := s.T()

	// Strictly controlled list of blob ids accessed, if at any time blob names
	// would change, that would mean change in blob hashing algorithm
	const bNameDir = "KAJgH9GYbmHxp4MUZvLswDh4t2TjTfVECAMmmv7MAzSZF"
	const bNameFile = "pKFmwKyCeLeHjFRiwhGaajuhupPg5tS61tcL6F7sjBHRW"

	s.setEntry(t, "hello", "file.txt")

	err := s.fs.Flush(t.Context())
	require.NoError(t, err)

	t.Run("dir read error", func(t *testing.T) {
		mockErr := errors.New("mock error dir")
		s.ds.openFunc = func(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
			switch n := name.String(); n {
			case bNameDir:
				return nil, mockErr
			case bNameFile:
				return s.ds.DS.Open(ctx, name)
			default:
				panic("Unrecognized blob: " + n)
			}
		}
		defer func() { s.ds.openFunc = nil }()

		_, _, code := s.getEntry("/file.txt")
		require.Equal(t, http.StatusInternalServerError, code)
		require.Contains(t, s.logData.String(), mockErr.Error())
	})

	t.Run("file open error", func(t *testing.T) {
		mockErr := errors.New("mock error file open")
		s.ds.openFunc = func(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
			switch n := name.String(); n {
			case bNameDir:
				return s.ds.DS.Open(ctx, name)
			case bNameFile:
				return nil, mockErr
			default:
				panic("Unrecognized blob: " + n)
			}
		}
		defer func() { s.ds.openFunc = nil }()

		_, _, code := s.getEntry("/file.txt")
		require.Equal(t, http.StatusInternalServerError, code)
		require.Contains(t, s.logData.String(), mockErr.Error())
	})

	t.Run("file read error with error header", func(t *testing.T) {
		mockErr := errors.New("mock error file read with headers")
		s.ds.openFunc = func(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
			switch n := name.String(); n {
			case bNameDir:
				return s.ds.DS.Open(ctx, name)
			case bNameFile:
				return io.NopCloser(iotest.ErrReader(mockErr)), nil
			default:
				panic("Unrecognized blob: " + n)
			}
		}
		defer func() { s.ds.openFunc = nil }()

		_, _, code := s.getEntry("/file.txt")
		require.Equal(t, http.StatusInternalServerError, code)
		require.Contains(t, s.logData.String(), mockErr.Error())
	})

	t.Run("file read error with partially sent data", func(t *testing.T) {
		mockErr := errors.New("mock error file read without headers")
		s.ds.openFunc = func(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
			switch n := name.String(); n {
			case bNameDir:
				return s.ds.DS.Open(ctx, name)
			case bNameFile:
				return io.NopCloser(io.MultiReader(
					strings.NewReader("hello world!"),
					iotest.ErrReader(mockErr),
				)), nil
			default:
				panic("Unrecognized blob: " + n)
			}
		}
		defer func() { s.ds.openFunc = nil }()

		content, _, _ := s.getEntry("/file.txt")
		// Since headers were already sent, there's no way to report back an error,
		// we can only check if logs contain some error information
		require.Contains(t, s.logData.String(), mockErr.Error())
		require.Contains(t, content, http.StatusText(http.StatusInternalServerError))
	})
}
