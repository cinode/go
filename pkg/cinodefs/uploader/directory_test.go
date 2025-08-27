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

package uploader_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/cinodefs/uploader"
	"github.com/cinode/go/pkg/datastore"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DirectoryTestSuite struct {
	suite.Suite

	cfs cinodefs.FS
}

func TestDirectoryTestSuite(t *testing.T) {
	suite.Run(t, &DirectoryTestSuite{})
}

func (s *DirectoryTestSuite) SetupTest() {
	t := s.T()

	cfs, err := cinodefs.New(
		t.Context(),
		blenc.FromDatastore(datastore.InMemory()),
		cinodefs.NewRootStaticDirectory(),
	)
	require.NoError(t, err)
	s.cfs = cfs
}

func (s *DirectoryTestSuite) singleFileFs() fstest.MapFS {
	return fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("hello")},
	}
}

type wrapFS struct {
	fs.FS

	openFunc    func(path string) (fs.File, error)
	statFunc    func(name string) (fs.FileInfo, error)
	readDirFunc func(name string) ([]fs.DirEntry, error)
}

func (w *wrapFS) Open(path string) (fs.File, error) {
	if w.openFunc != nil {
		return w.openFunc(path)
	}
	return w.FS.Open(path)
}

func (w *wrapFS) Stat(name string) (fs.FileInfo, error) {
	if w.statFunc != nil {
		return w.statFunc(name)
	}
	return fs.Stat(w.FS, name)
}

func (w *wrapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if w.readDirFunc != nil {
		return w.readDirFunc(name)
	}
	return fs.ReadDir(w.FS, name)
}

func (s *DirectoryTestSuite) uploadFS(t *testing.T, fs fs.FS, opts ...uploader.Option) {
	err := uploader.UploadStaticDirectory(
		t.Context(),
		fs,
		s.cfs,
		opts...,
	)
	require.NoError(t, err)
}

func (s *DirectoryTestSuite) readContent(t *testing.T, path ...string) (string, error) {
	rc, err := s.cfs.OpenEntryData(t.Context(), path)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	return string(data), err
}

func (s *DirectoryTestSuite) TestSingleFileUploadDefaultOptions() {
	t := s.T()

	s.uploadFS(t, s.singleFileFs())

	readBack, err := s.readContent(t, "file.txt")
	require.NoError(t, err)
	require.Equal(t, "hello", readBack)
}

func (s *DirectoryTestSuite) TestSingleFileUploadBasePath() {
	t := s.T()

	s.uploadFS(t, s.singleFileFs(), uploader.BasePath("sub", "dir"))

	readBack, err := s.readContent(t, "sub", "dir", "file.txt")
	require.NoError(t, err)
	require.Equal(t, "hello", readBack)

	_, err = s.readContent(t, "file.txt")
	require.ErrorIs(t, err, cinodefs.ErrEntryNotFound)
}

func (s *DirectoryTestSuite) TestSingleFileUploadWithIndexFile() {
	t := s.T()

	s.uploadFS(t, s.singleFileFs(), uploader.CreateIndexFile("index.html"))

	readBack, err := s.readContent(t, "index.html")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(readBack, "<!DOCTYPE"))
	require.Contains(t, readBack, "file.txt")
}

func (s *DirectoryTestSuite) TestSingleFileUploadWithIndexFileDontOverwrite() {
	t := s.T()

	fs := s.singleFileFs()
	fs["index.html"] = &fstest.MapFile{Data: []byte("not-html")}
	s.uploadFS(t, fs, uploader.CreateIndexFile("index.html"))

	readBack, err := s.readContent(t, "index.html")
	require.NoError(t, err)
	require.Equal(t, "not-html", readBack)
}

func (s *DirectoryTestSuite) TestFailLinkUpload() {
	t := s.T()

	testFS := &fstest.MapFS{
		"file.txt": &fstest.MapFile{
			Data: []byte("hello"),
			Mode: fs.ModeCharDevice,
		},
	}

	err := uploader.UploadStaticDirectory(
		t.Context(),
		testFS,
		s.cfs,
	)
	require.ErrorIs(t, err, uploader.ErrNotADirectoryOrAFile)
}

func (s *DirectoryTestSuite) TestFailUploadFileOpen() {
	t := s.T()

	injectErr := errors.New("injected open error")
	testFS := &wrapFS{FS: s.singleFileFs()}
	testFS.openFunc = func(path string) (fs.File, error) {
		if path == "file.txt" {
			return nil, injectErr
		}
		return testFS.FS.Open(path)
	}

	err := uploader.UploadStaticDirectory(
		t.Context(),
		testFS,
		s.cfs,
	)
	require.ErrorIs(t, err, injectErr)
}

type wrappedFile struct {
	fs.File
	readFunc func([]byte) (int, error)
}

func (w *wrappedFile) Read(b []byte) (int, error) {
	if w.readFunc != nil {
		return w.readFunc(b)
	}
	return w.File.Read(b)
}

func (s *DirectoryTestSuite) TestFailUploadFileRead() {
	t := s.T()

	injectErr := errors.New("injected read error")
	testFS := &wrapFS{FS: s.singleFileFs()}
	testFS.openFunc = func(path string) (fs.File, error) {
		if path == "file.txt" {
			fl, err := testFS.FS.Open(path)
			if err != nil {
				return nil, err
			}
			return &wrappedFile{
				File:     fl,
				readFunc: func(b []byte) (int, error) { return 0, injectErr },
			}, nil
		}
		return testFS.FS.Open(path)
	}

	err := uploader.UploadStaticDirectory(
		t.Context(),
		testFS,
		s.cfs,
	)
	require.ErrorIs(t, err, injectErr)
}

func (s *DirectoryTestSuite) TestFailUploadStat() {
	t := s.T()

	injectErr := errors.New("injected stat error")
	testFS := &wrapFS{FS: s.singleFileFs()}
	testFS.statFunc = func(name string) (fs.FileInfo, error) { return nil, injectErr }

	err := uploader.UploadStaticDirectory(
		t.Context(),
		testFS,
		s.cfs,
	)
	require.ErrorIs(t, err, injectErr)
}

func (s *DirectoryTestSuite) TestFailUploadReadDir() {
	t := s.T()

	injectErr := errors.New("injected readdir error")
	testFS := &wrapFS{FS: s.singleFileFs()}
	testFS.readDirFunc = func(name string) ([]fs.DirEntry, error) { return nil, injectErr }

	err := uploader.UploadStaticDirectory(
		t.Context(),
		testFS,
		s.cfs,
	)
	require.ErrorIs(t, err, injectErr)
}

type wrappedCinodeFS struct {
	cinodefs.FS
	setEntryFileFunc func(ctx context.Context, path []string, data io.Reader, opts ...cinodefs.EntrypointOption) (*cinodefs.Entrypoint, error)
}

func (w *wrappedCinodeFS) SetEntryFile(
	ctx context.Context,
	path []string,
	data io.Reader,
	opts ...cinodefs.EntrypointOption,
) (*cinodefs.Entrypoint, error) {
	if w.setEntryFileFunc != nil {
		return w.setEntryFileFunc(ctx, path, data, opts...)
	}
	return w.FS.SetEntryFile(ctx, path, data, opts...)
}

func (s *DirectoryTestSuite) TestFailStoreFile() {
	t := s.T()

	injectErr := errors.New("injected cfs store error")
	origFs := s.cfs

	for _, fName := range []string{"file.txt", "index.html"} {
		s.cfs = &wrappedCinodeFS{
			FS: origFs,
			setEntryFileFunc: func(ctx context.Context, path []string, data io.Reader, opts ...cinodefs.EntrypointOption) (*cinodefs.Entrypoint, error) {
				if path[0] == fName {
					return nil, injectErr
				}
				return origFs.SetEntryFile(ctx, path, data, opts...)
			},
		}

		err := uploader.UploadStaticDirectory(
			t.Context(),
			s.singleFileFs(),
			s.cfs,
			uploader.CreateIndexFile("index.html"),
		)
		require.ErrorIs(t, err, injectErr)
	}
}
