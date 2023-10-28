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

package cinodefs_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/datastore"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestCinodeFSSingleFileScenario(t *testing.T) {
	ctx := context.Background()
	fs, err := cinodefs.New(ctx,
		blenc.FromDatastore(datastore.InMemory()),
		cinodefs.NewRootDynamicLink(),
	)
	require.NoError(t, err)
	require.NotNil(t, fs)

	{ // Check single file write operation
		path1 := []string{"dir", "subdir", "file.txt"}

		ep1, err := fs.SetEntryFile(ctx,
			path1,
			strings.NewReader("Hello world!"),
		)
		require.NoError(t, err)
		require.NotNil(t, ep1)

		ep2, err := fs.FindEntry(
			ctx,
			path1,
		)
		require.NoError(t, err)
		require.NotNil(t, ep2)

		require.Equal(t, ep1.String(), ep2.String())

		// Directories are modified, not yet flushed
		for i := range path1 {
			ep3, err := fs.FindEntry(ctx, path1[:i])
			require.ErrorIs(t, err, cinodefs.ErrModifiedDirectory)
			require.Nil(t, ep3)
		}

		err = fs.Flush(ctx)
		require.NoError(t, err)
	}
}

type testFileEntry struct {
	path     []string
	content  string
	mimeType string
}

type CinodeFSMultiFileTestSuite struct {
	suite.Suite

	ds         datastore.DS
	fs         cinodefs.FS
	contentMap []testFileEntry
}

func TestCinodeFSMultiFileTestSuite(t *testing.T) {
	suite.Run(t, &CinodeFSMultiFileTestSuite{})
}

func (c *CinodeFSMultiFileTestSuite) SetupTest() {
	ctx := context.Background()

	c.ds = datastore.InMemory()
	fs, err := cinodefs.New(ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.NewRootDynamicLink(),
	)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), fs)
	c.fs = fs

	c.contentMap = make([]testFileEntry, 1000)
	for i := 0; i < 1000; i++ {
		c.contentMap[i].path = []string{
			fmt.Sprintf("dir%d", i%7),
			fmt.Sprintf("subdir%d", i%19),
			fmt.Sprintf("file%d.txt", i),
		}
		c.contentMap[i].content = fmt.Sprintf("Hello world! from file %d!", i)
		c.contentMap[i].mimeType = "text/plain"
	}

	for _, file := range c.contentMap {
		_, err := c.fs.SetEntryFile(ctx,
			file.path,
			strings.NewReader(file.content),
		)
		require.NoError(c.T(), err)
	}

	c.checkContentMap(fs)

	err = c.fs.Flush(context.Background())
	require.NoError(c.T(), err)

	c.checkContentMap(c.fs)
}

func (c *CinodeFSMultiFileTestSuite) checkContentMap(fs cinodefs.FS) {
	ctx := context.Background()
	for _, file := range c.contentMap {
		ep, err := fs.FindEntry(ctx, file.path)
		require.NoError(c.T(), err)
		require.Contains(c.T(), ep.MimeType(), file.mimeType)

		rc, err := fs.OpenEntrypointData(ctx, ep)
		require.NoError(c.T(), err)
		defer rc.Close()

		data, err := io.ReadAll(rc)
		require.NoError(c.T(), err)

		require.Equal(c.T(), file.content, string(data))
	}
}

func (c *CinodeFSMultiFileTestSuite) TestReopeningInReadOnlyMode() {
	ctx := context.Background()
	rootEP, err := c.fs.RootEntrypoint()
	require.NoError(c.T(), err)

	fs2, err := cinodefs.New(
		ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.RootEntrypoint(rootEP),
	)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), fs2)

	c.checkContentMap(fs2)

	_, err = c.fs.SetEntryFile(ctx,
		c.contentMap[0].path,
		strings.NewReader("modified content"),
	)
	require.NoError(c.T(), err)

	// Data in fs was not yet flushed to the datastore, fs2 should still refer to the old content
	c.checkContentMap(fs2)

	err = c.fs.Flush(ctx)
	require.NoError(c.T(), err)

	// reopen fs2 to avoid any caching issues
	fs2, err = cinodefs.New(
		ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.RootEntrypoint(rootEP),
	)
	require.NoError(c.T(), err)

	// Check with modified content map
	c.contentMap[0].content = "modified content"
	c.checkContentMap(fs2)

	// We should not be allowed to modify fs2 without writer info
	ep, err := fs2.SetEntryFile(ctx, c.contentMap[0].path, strings.NewReader("should fail"))
	require.ErrorIs(c.T(), err, cinodefs.ErrMissingWriterInfo)
	require.Nil(c.T(), ep)
	c.checkContentMap(c.fs)
	c.checkContentMap(fs2)
}

func (c *CinodeFSMultiFileTestSuite) TestReopeningInReadWriteMode() {
	ctx := context.Background()

	rootWriterInfo, err := c.fs.RootWriterInfo(ctx)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), rootWriterInfo)

	fs3, err := cinodefs.New(
		ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.RootWriterInfo(rootWriterInfo),
	)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), fs3)

	c.checkContentMap(fs3)

	// With a proper auth info we can modify files in the root path
	ep, err := fs3.SetEntryFile(ctx, c.contentMap[0].path, strings.NewReader("modified through fs3"))
	require.NoError(c.T(), err)
	require.NotNil(c.T(), ep)

	c.contentMap[0].content = "modified through fs3"
	c.checkContentMap(fs3)
}

func (c *CinodeFSMultiFileTestSuite) TestRemovalOfAFile() {
	ctx := context.Background()

	err := c.fs.DeleteEntry(ctx, c.contentMap[0].path)
	require.NoError(c.T(), err)

	c.contentMap = c.contentMap[1:]
	c.checkContentMap(c.fs)
}

func (c *CinodeFSMultiFileTestSuite) TestRemovalOfADirectory() {
	ctx := context.Background()

	removedPath := c.contentMap[0].path[:2]

	err := c.fs.DeleteEntry(ctx, removedPath)
	require.NoError(c.T(), err)

	filteredEntries := []testFileEntry{}
	removed := 0
	for _, e := range c.contentMap {
		if e.path[0] == removedPath[0] && e.path[1] == removedPath[1] {
			continue
		}

		filteredEntries = append(filteredEntries, e)
		removed++
	}
	c.contentMap = filteredEntries
	require.NotZero(c.T(), removed)

	c.checkContentMap(c.fs)

	err = c.fs.DeleteEntry(ctx, removedPath)
	require.ErrorIs(c.T(), err, cinodefs.ErrEntryNotFound)

	c.checkContentMap(c.fs)
}

func (c *CinodeFSMultiFileTestSuite) TestDeleteTreatFileAsDirectory() {
	ctx := context.Background()

	path := append(c.contentMap[0].path, "sub-file")
	err := c.fs.DeleteEntry(ctx, path)
	require.ErrorIs(c.T(), err, cinodefs.ErrNotADirectory)
}

func (c *CinodeFSMultiFileTestSuite) TestPreventSettingFileAsDirectory() {
	ctx := context.Background()

	path := append(c.contentMap[0].path, "sub-file")
	_, err := c.fs.SetEntryFile(ctx, path, strings.NewReader("should not happen"))
	require.ErrorIs(c.T(), err, cinodefs.ErrNotADirectory)
}

func (c *CinodeFSMultiFileTestSuite) TestPreventSettingEmptyEntryName() {
	ctx := context.Background()

	for _, path := range [][]string{
		{"", "subdir", "file.txt"},
		{"dir", "", "file.txt"},
		{"dir", "subdir", ""},
	} {
		c.T().Run(strings.Join(path, "::"), func(t *testing.T) {
			_, err := c.fs.SetEntryFile(ctx, path, strings.NewReader("should not succeed"))
			require.ErrorIs(t, err, cinodefs.ErrEmptyName)

		})
	}

}

func (c *CinodeFSMultiFileTestSuite) TestRootEPLinkOnDirtyFS() {
	ctx := context.Background()

	ep1, err := c.fs.RootEntrypoint()
	require.NoError(c.T(), err)

	_, err = c.fs.SetEntryFile(ctx, c.contentMap[0].path, strings.NewReader("hello"))
	require.NoError(c.T(), err)

	ep2, err := c.fs.RootEntrypoint()
	require.NoError(c.T(), err)

	// Even though dirty, entrypoint won't change it's content
	require.Equal(c.T(), ep1.String(), ep2.String())

	err = c.fs.Flush(ctx)
	require.NoError(c.T(), err)

	ep3, err := c.fs.RootEntrypoint()
	require.NoError(c.T(), err)

	require.Equal(c.T(), ep1.String(), ep3.String())
}

func (c *CinodeFSMultiFileTestSuite) TestRootEPDirectoryOnDirtyFS() {
	ctx := context.Background()

	rootDir, err := c.fs.FindEntry(ctx, []string{})
	require.NoError(c.T(), err)

	fs2, err := cinodefs.New(ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.RootEntrypoint(rootDir),
	)
	require.NoError(c.T(), err)

	ep1, err := fs2.RootEntrypoint()
	require.NoError(c.T(), err)
	require.Equal(c.T(), rootDir.String(), ep1.String())

	_, err = fs2.SetEntryFile(ctx, c.contentMap[0].path, strings.NewReader("hello"))
	require.NoError(c.T(), err)

	ep2, err := fs2.RootEntrypoint()
	require.ErrorIs(c.T(), err, cinodefs.ErrModifiedDirectory)
	require.Nil(c.T(), ep2)

	err = fs2.Flush(ctx)
	require.NoError(c.T(), err)

	ep3, err := c.fs.RootEntrypoint()
	require.NoError(c.T(), err)

	require.NotEqual(c.T(), ep1.String(), ep3.String())
}

func (c *CinodeFSMultiFileTestSuite) TestWriteOnlyLink() {
	// ctx := context.Background()
	// fs, err := graph.NewCinodeFS(ctx, blenc.FromDatastore(datastore.InMemory()), graph.NewRootDynamicLink())
	// require.NoError(c.T(), err)

}
