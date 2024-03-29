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
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/cinodefs/protobuf"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
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

type testBEWrapper struct {
	blenc.BE

	createFunc func(
		ctx context.Context, blobType common.BlobType, r io.Reader,
	) (*common.BlobName, *common.BlobKey, *common.AuthInfo, error)

	updateFunc func(
		ctx context.Context, name *common.BlobName, ai *common.AuthInfo,
		key *common.BlobKey, r io.Reader,
	) error
}

func (w *testBEWrapper) Create(
	ctx context.Context, blobType common.BlobType, r io.Reader,
) (*common.BlobName, *common.BlobKey, *common.AuthInfo, error) {
	if w.createFunc != nil {
		return w.createFunc(ctx, blobType, r)
	}
	return w.BE.Create(ctx, blobType, r)
}

func (w *testBEWrapper) Update(
	ctx context.Context, name *common.BlobName, ai *common.AuthInfo,
	key *common.BlobKey, r io.Reader,
) error {
	if w.updateFunc != nil {
		return w.updateFunc(ctx, name, ai, key, r)
	}
	return w.BE.Update(ctx, name, ai, key, r)
}

type testFileEntry struct {
	path     []string
	content  string
	mimeType string
}

type CinodeFSMultiFileTestSuite struct {
	suite.Suite

	ds               datastore.DS
	be               testBEWrapper
	fs               cinodefs.FS
	contentMap       []testFileEntry
	maxLinkRedirects int
	randSource       io.Reader
	timeFunc         func() time.Time
}

type randReaderForCinodeFSMultiFileTestSuite CinodeFSMultiFileTestSuite

func (r *randReaderForCinodeFSMultiFileTestSuite) Read(b []byte) (int, error) {
	return r.randSource.Read(b)
}

func TestCinodeFSMultiFileTestSuite(t *testing.T) {
	suite.Run(t, &CinodeFSMultiFileTestSuite{
		maxLinkRedirects: 5,
	})
}

func (c *CinodeFSMultiFileTestSuite) SetupTest() {
	ctx := context.Background()

	c.timeFunc = time.Now
	c.randSource = rand.Reader
	c.ds = datastore.InMemory()
	c.be = testBEWrapper{
		BE: blenc.FromDatastore(c.ds),
	}
	fs, err := cinodefs.New(ctx,
		&c.be,
		cinodefs.NewRootDynamicLink(),
		cinodefs.MaxLinkRedirects(c.maxLinkRedirects),
		cinodefs.TimeFunc(func() time.Time { return c.timeFunc() }),
		cinodefs.RandSource((*randReaderForCinodeFSMultiFileTestSuite)(c)),
	)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), fs)
	c.fs = fs

	const testFilesCount = 10
	const dirsCount = 3
	const subDirsCount = 2

	c.contentMap = make([]testFileEntry, testFilesCount)
	for i := 0; i < testFilesCount; i++ {
		c.contentMap[i].path = []string{
			fmt.Sprintf("dir%d", i%dirsCount),
			fmt.Sprintf("subdir%d", i%subDirsCount),
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

	err = c.fs.Flush(context.Background())
	require.NoError(c.T(), err)
}

func (c *CinodeFSMultiFileTestSuite) checkContentMap(t *testing.T, fs cinodefs.FS) {
	ctx := context.Background()
	for _, file := range c.contentMap {
		ep, err := fs.FindEntry(ctx, file.path)
		require.NoError(t, err)
		require.Contains(t, ep.MimeType(), file.mimeType)

		rc, err := fs.OpenEntrypointData(ctx, ep)
		require.NoError(t, err)
		defer rc.Close()

		data, err := io.ReadAll(rc)
		require.NoError(t, err)

		require.Equal(t, file.content, string(data))
	}
}

func (c *CinodeFSMultiFileTestSuite) TestReopeningInReadOnlyMode() {
	ctx := context.Background()
	rootEP, err := c.fs.RootEntrypoint()
	require.NoError(c.T(), err)

	fs2, err := cinodefs.New(
		ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.RootEntrypointString(rootEP.String()),
	)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), fs2)

	c.checkContentMap(c.T(), fs2)

	_, err = c.fs.SetEntryFile(ctx,
		c.contentMap[0].path,
		strings.NewReader("modified content"),
	)
	require.NoError(c.T(), err)

	// Data in fs was not yet flushed to the datastore, fs2 should still refer to the old content
	c.checkContentMap(c.T(), fs2)

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
	c.checkContentMap(c.T(), fs2)

	// We should not be allowed to modify fs2 without writer info
	ep, err := fs2.SetEntryFile(ctx, c.contentMap[0].path, strings.NewReader("should fail"))
	require.ErrorIs(c.T(), err, cinodefs.ErrMissingWriterInfo)
	require.Nil(c.T(), ep)
	c.checkContentMap(c.T(), c.fs)
	c.checkContentMap(c.T(), fs2)
}

func (c *CinodeFSMultiFileTestSuite) TestReopeningInReadWriteMode() {
	ctx := context.Background()

	rootWriterInfo, err := c.fs.RootWriterInfo(ctx)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), rootWriterInfo)

	fs3, err := cinodefs.New(
		ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.RootWriterInfoString(rootWriterInfo.String()),
	)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), fs3)

	c.checkContentMap(c.T(), fs3)

	// With a proper auth info we can modify files in the root path
	ep, err := fs3.SetEntryFile(ctx, c.contentMap[0].path, strings.NewReader("modified through fs3"))
	require.NoError(c.T(), err)
	require.NotNil(c.T(), ep)

	c.contentMap[0].content = "modified through fs3"
	c.checkContentMap(c.T(), fs3)
}

func (c *CinodeFSMultiFileTestSuite) TestRemovalOfAFile() {
	ctx := context.Background()

	err := c.fs.DeleteEntry(ctx, c.contentMap[0].path)
	require.NoError(c.T(), err)

	c.contentMap = c.contentMap[1:]
	c.checkContentMap(c.T(), c.fs)
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

	c.checkContentMap(c.T(), c.fs)

	err = c.fs.DeleteEntry(ctx, removedPath)
	require.ErrorIs(c.T(), err, cinodefs.ErrEntryNotFound)

	c.checkContentMap(c.T(), c.fs)

	ep, err := c.fs.FindEntry(ctx, removedPath)
	require.ErrorIs(c.T(), err, cinodefs.ErrEntryNotFound)
	require.Nil(c.T(), ep)

	err = c.fs.DeleteEntry(ctx, []string{})
	require.ErrorIs(c.T(), err, cinodefs.ErrCantDeleteRoot)
}

func (c *CinodeFSMultiFileTestSuite) TestDeleteTreatFileAsDirectory() {
	ctx := context.Background()

	path := append(c.contentMap[0].path, "sub-file")
	err := c.fs.DeleteEntry(ctx, path)
	require.ErrorIs(c.T(), err, cinodefs.ErrNotADirectory)
}

func (c *CinodeFSMultiFileTestSuite) TestResetDir() {
	ctx := context.Background()

	removedPath := c.contentMap[0].path[:2]

	err := c.fs.ResetDir(ctx, removedPath)
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

	c.checkContentMap(c.T(), c.fs)

	err = c.fs.ResetDir(ctx, removedPath)
	require.NoError(c.T(), err)

	c.checkContentMap(c.T(), c.fs)

	ep, err := c.fs.FindEntry(ctx, removedPath)
	require.ErrorIs(c.T(), err, cinodefs.ErrModifiedDirectory)
	require.Nil(c.T(), ep)
}

func (c *CinodeFSMultiFileTestSuite) TestSettingEntry() {
	ctx := context.Background()

	c.T().Run("prevent treating file as directory", func(t *testing.T) {
		path := append(c.contentMap[0].path, "sub-file")
		_, err := c.fs.SetEntryFile(ctx, path, strings.NewReader("should not happen"))
		require.ErrorIs(t, err, cinodefs.ErrNotADirectory)
	})

	c.T().Run("prevent setting empty path segment", func(t *testing.T) {
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
	})

	c.T().Run("tet root entrypoint on dirty filesystem", func(t *testing.T) {
		ep1, err := c.fs.RootEntrypoint()
		require.NoError(t, err)

		_, err = c.fs.SetEntryFile(ctx, c.contentMap[0].path, strings.NewReader("hello"))
		require.NoError(t, err)
		c.contentMap[0].content = "hello"

		ep2, err := c.fs.RootEntrypoint()
		require.NoError(t, err)

		// Even though dirty, entrypoint won't change it's content
		require.Equal(t, ep1.String(), ep2.String())

		err = c.fs.Flush(ctx)
		require.NoError(t, err)

		ep3, err := c.fs.RootEntrypoint()
		require.NoError(t, err)

		require.Equal(t, ep1.String(), ep3.String())
	})

	c.T().Run("test crete file entrypoint", func(t *testing.T) {
		ep, err := c.fs.CreateFileEntrypoint(ctx, strings.NewReader("new file"))
		require.NoError(t, err)
		require.NotNil(t, ep)

		err = c.fs.SetEntry(context.Background(), []string{"new-file.txt"}, ep)
		require.NoError(t, err)

		c.contentMap = append(c.contentMap, testFileEntry{
			path:     []string{"new-file.txt"},
			content:  "new file",
			mimeType: ep.MimeType(),
		})

		c.checkContentMap(c.T(), c.fs)
	})
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

func (c *CinodeFSMultiFileTestSuite) TestOpeningData() {
	_, err := c.fs.OpenEntrypointData(context.Background(), nil)
	require.ErrorIs(c.T(), err, cinodefs.ErrNilEntrypoint)

	_, err = c.fs.OpenEntryData(context.Background(), []string{"a", "b", "c"})
	require.ErrorIs(c.T(), err, cinodefs.ErrEntryNotFound)

	_, err = c.fs.OpenEntryData(context.Background(), []string{})
	require.ErrorIs(c.T(), err, cinodefs.ErrIsADirectory)

	contentReader, err := c.fs.OpenEntryData(context.Background(), c.contentMap[0].path)
	require.NoError(c.T(), err)
	content, err := io.ReadAll(contentReader)
	require.NoError(c.T(), err)
	require.Equal(c.T(), c.contentMap[0].content, string(content))
}

func (c *CinodeFSMultiFileTestSuite) TestSubLinksAndWriteOnlyPath() {
	ctx := context.Background()
	t := c.T()
	path := append([]string{}, c.contentMap[0].path...)
	path = append(path[:len(path)-1], "linked", "sub", "directory", "linked-file.txt")
	linkPath := path[:len(path)-2]

	// Create normal file
	ep, err := c.fs.SetEntryFile(ctx, path, strings.NewReader("linked-file"))
	require.NoError(t, err)
	c.contentMap = append(c.contentMap, testFileEntry{
		path:     path,
		content:  "linked-file",
		mimeType: ep.MimeType(),
	})
	c.checkContentMap(t, c.fs)

	// Convert path to the file to a dynamic link
	wi, err := c.fs.InjectDynamicLink(ctx, linkPath)
	require.NoError(t, err)
	require.NotNil(t, wi)
	c.checkContentMap(t, c.fs)

	// Ensure flushing through the dynamic link works
	err = c.fs.Flush(ctx)
	require.NoError(t, err)
	c.checkContentMap(t, c.fs)

	// Ensure the content can still be changed - corresponding auth info
	// is still kept in the concept
	_, err = c.fs.SetEntryFile(ctx, path, strings.NewReader("updated-linked-file"))
	require.NoError(t, err)
	c.contentMap[len(c.contentMap)-1].content = "updated-linked-file"
	c.checkContentMap(t, c.fs)

	// Ensure flushing works after the change behind the link
	err = c.fs.Flush(ctx)
	require.NoError(t, err)
	c.checkContentMap(t, c.fs)

	rootWriterInfo, err := c.fs.RootWriterInfo(ctx)
	require.NoError(t, err)

	// Reopen the filesystem, but only with the root writer info
	fs2, err := cinodefs.New(ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.RootWriterInfoString(rootWriterInfo.String()),
	)
	require.NoError(c.T(), err)
	c.checkContentMap(c.T(), fs2)

	// Can not do any operation below the split point
	ep, err = fs2.SetEntryFile(ctx, path, strings.NewReader("won't work"))
	require.ErrorIs(t, err, cinodefs.ErrMissingWriterInfo)
	require.Nil(t, ep)

	altPath := append(append([]string{}, path[:len(path)-1]...), "other", "directory", "path")
	ep, err = fs2.SetEntryFile(ctx, altPath, strings.NewReader("won't work"))
	require.ErrorIs(t, err, cinodefs.ErrMissingWriterInfo)
	require.Nil(t, ep)

	err = fs2.ResetDir(ctx, path[:len(path)-1])
	require.ErrorIs(t, err, cinodefs.ErrMissingWriterInfo)

	err = fs2.DeleteEntry(ctx, path)
	require.ErrorIs(t, err, cinodefs.ErrMissingWriterInfo)

	_, err = fs2.InjectDynamicLink(ctx, path)
	require.ErrorIs(t, err, cinodefs.ErrMissingWriterInfo)
}

func (c *CinodeFSMultiFileTestSuite) TestMaxLinksRedirects() {
	t := c.T()
	ctx := context.Background()

	entryPath := c.contentMap[0].path
	linkPath := entryPath[:len(entryPath)-1]

	// Up to max links redirects, lookup must be allowed
	for i := 0; i < c.maxLinkRedirects; i++ {
		_, err := c.fs.InjectDynamicLink(ctx, linkPath)
		require.NoError(t, err)

		_, err = c.fs.FindEntry(ctx, entryPath)
		require.NoError(t, err)
	}

	// Cross the max redirects count, next lookup should fail
	_, err := c.fs.InjectDynamicLink(ctx, linkPath)
	require.NoError(t, err)

	_, err = c.fs.FindEntry(ctx, entryPath)
	require.ErrorIs(t, err, cinodefs.ErrTooManyRedirects)
}

func (c *CinodeFSMultiFileTestSuite) TestExplicitMimeType() {
	t := c.T()
	ctx := context.Background()
	entryPath := c.contentMap[0].path
	const newMimeType = "forced-mime-type"

	_, err := c.fs.SetEntryFile(ctx,
		entryPath,
		strings.NewReader("modified content"),
		cinodefs.SetMimeType(newMimeType),
	)
	require.NoError(t, err)

	entry, err := c.fs.FindEntry(ctx, entryPath)
	require.NoError(t, err)
	require.Equal(t, newMimeType, entry.MimeType())
}

func (c *CinodeFSMultiFileTestSuite) TestMalformedDirectory() {
	var ep protobuf.Entrypoint
	err := proto.Unmarshal(
		golang.Must(c.fs.FindEntry(context.Background(), c.contentMap[0].path)).Bytes(),
		&ep,
	)
	require.NoError(c.T(), err)

	var brokenEP protobuf.Entrypoint
	proto.Merge(&brokenEP, &ep)
	brokenEP.BlobName = []byte{}

	for _, d := range []struct {
		n   string
		d   []byte
		err error
	}{
		{
			"malformed data",
			[]byte{23, 45, 67, 89, 12, 34, 56, 78, 90}, // Some malformed message
			cinodefs.ErrCantOpenDir,
		},
		{
			"entry with empty name",
			golang.Must(proto.Marshal(&protobuf.Directory{
				Entries: []*protobuf.Directory_Entry{{
					Name: "",
				}},
			})),
			cinodefs.ErrEmptyName,
		},
		{
			"two entries with the same name",
			golang.Must(proto.Marshal(&protobuf.Directory{
				Entries: []*protobuf.Directory_Entry{
					{Name: "entry", Ep: &ep},
					{Name: "entry", Ep: &ep},
				},
			})),
			cinodefs.ErrCantOpenDirDuplicateEntry,
		},
		{
			"missing entrypoint",
			golang.Must(proto.Marshal(&protobuf.Directory{
				Entries: []*protobuf.Directory_Entry{
					{Name: "entry"},
				},
			})),
			cinodefs.ErrInvalidEntrypointDataNil,
		},
		{
			"missing blob name",
			golang.Must(proto.Marshal(&protobuf.Directory{
				Entries: []*protobuf.Directory_Entry{
					{Name: "entry", Ep: &brokenEP},
				},
			})),
			common.ErrInvalidBlobName,
		},
	} {
		c.T().Run(d.n, func(t *testing.T) {
			_, err := c.fs.SetEntryFile(context.Background(),
				[]string{"dir"},
				bytes.NewReader(d.d),
				cinodefs.SetMimeType(cinodefs.CinodeDirMimeType),
			)
			require.NoError(t, err)

			_, err = c.fs.FindEntry(context.Background(), []string{"dir", "entry"})
			require.ErrorIs(t, err, cinodefs.ErrCantOpenDir)
			require.ErrorIs(t, err, d.err)

			// TODO: We should be able to set new entry even if the underlying object is broken
			err = c.fs.DeleteEntry(context.Background(), []string{"dir"})
			require.NoError(t, err)
		})
	}
}

func (c *CinodeFSMultiFileTestSuite) TestMalformedLink() {
	var ep protobuf.Entrypoint
	err := proto.Unmarshal(
		golang.Must(c.fs.FindEntry(context.Background(), c.contentMap[0].path)).Bytes(),
		&ep,
	)
	require.NoError(c.T(), err)

	var brokenEP protobuf.Entrypoint
	proto.Merge(&brokenEP, &ep)
	brokenEP.BlobName = []byte{}

	_, err = c.fs.SetEntryFile(context.Background(), []string{"link", "file"}, strings.NewReader("test"))
	require.NoError(c.T(), err)

	linkWI_, err := c.fs.InjectDynamicLink(context.Background(), []string{"link"})
	require.NoError(c.T(), err)

	// Flush is needed so that we can update entrypoint data and the fs cache won't get into our way
	err = c.fs.Flush(context.Background())
	require.NoError(c.T(), err)

	for _, d := range []struct {
		n   string
		d   []byte
		err error
	}{
		{
			"malformed data",
			[]byte{23, 45, 67, 89, 12, 34, 56, 78, 90}, // Some malformed message
			cinodefs.ErrCantOpenLink,
		},
		{
			"missing target blob name",
			golang.Must(proto.Marshal(&brokenEP)),
			common.ErrInvalidBlobName,
		},
	} {
		c.T().Run(d.n, func(t *testing.T) {
			var linkWI protobuf.WriterInfo
			err = proto.Unmarshal(linkWI_.Bytes(), &linkWI)
			require.NoError(c.T(), err)
			linkBlobName := golang.Must(common.BlobNameFromBytes(linkWI.BlobName))
			linkAuthInfo := common.AuthInfoFromBytes(linkWI.AuthInfo)
			linkKey := common.BlobKeyFromBytes(linkWI.Key)

			err = c.be.Update(context.Background(),
				linkBlobName, linkAuthInfo, linkKey, bytes.NewReader(d.d),
			)
			require.NoError(t, err)

			_, err = c.fs.FindEntry(context.Background(), []string{"link", "file"})
			require.ErrorIs(t, err, cinodefs.ErrCantOpenLink)
			require.ErrorIs(t, err, d.err)
		})
	}
}

func (c *CinodeFSMultiFileTestSuite) TestPathWithMultipleLinks() {
	path := []string{
		"multi",
		"level",
		"path",
		"with",
		"more",
		"than",
		"one",
		"link",
	}
	ctx := context.Background()
	t := c.T()

	// Create test entry
	const initialContent = "initial content"
	ep, err := c.fs.SetEntryFile(ctx, path, strings.NewReader(initialContent))
	require.NoError(t, err)

	// Inject few links among the path to the entry
	for _, splitPoint := range []int{2, 6, 4} {
		_, err = c.fs.InjectDynamicLink(ctx, path[:splitPoint])
		require.NoError(t, err)

		err = c.fs.Flush(ctx)
		require.NoError(t, err)
	}

	// Create parallel filesystem
	rootEP, err := c.fs.RootEntrypoint()
	require.NoError(t, err)

	fs2, err := cinodefs.New(ctx,
		blenc.FromDatastore(c.ds),
		cinodefs.RootEntrypointString(rootEP.String()),
	)
	require.NoError(t, err)

	c.contentMap = append(c.contentMap, testFileEntry{
		path:     path,
		content:  initialContent,
		mimeType: ep.MimeType(),
	})
	c.checkContentMap(t, c.fs)

	// Modify the content of the file in the original filesystem, not yet flushed
	const modifiedContent1 = "modified content 1"
	_, err = c.fs.SetEntryFile(ctx, path, strings.NewReader(modifiedContent1))
	require.NoError(t, err)

	// Change not yet observed through the second filesystem due to no flush
	c.checkContentMap(t, fs2)

	err = c.fs.Flush(ctx)
	require.NoError(t, err)

	// Change must now be observed through the second filesystem
	c.contentMap[len(c.contentMap)-1].content = modifiedContent1
	c.checkContentMap(t, c.fs)
	c.checkContentMap(t, fs2)
}

func (c *CinodeFSMultiFileTestSuite) TestBlobWriteErrorWhenCreatingFile() {
	injectedErr := errors.New("entry file create error")
	c.be.createFunc = func(ctx context.Context, blobType common.BlobType, r io.Reader,
	) (*common.BlobName, *common.BlobKey, *common.AuthInfo, error) {
		return nil, nil, nil, injectedErr
	}

	_, err := c.fs.SetEntryFile(context.Background(), []string{"file"}, strings.NewReader("test"))
	require.ErrorIs(c.T(), err, injectedErr)
}

func (c *CinodeFSMultiFileTestSuite) TestBlobWriteErrorWhenFlushing() {
	_, err := c.fs.SetEntryFile(context.Background(), []string{"file"}, strings.NewReader("test"))
	require.NoError(c.T(), err)

	injectedErr := errors.New("flush error")
	c.be.createFunc = func(ctx context.Context, blobType common.BlobType, r io.Reader,
	) (*common.BlobName, *common.BlobKey, *common.AuthInfo, error) {
		return nil, nil, nil, injectedErr
	}

	err = c.fs.Flush(context.Background())
	require.ErrorIs(c.T(), err, injectedErr)
}

func (c *CinodeFSMultiFileTestSuite) TestLinkGenerationError() {
	injectedErr := errors.New("rand data read error")

	c.randSource = iotest.ErrReader(injectedErr)

	_, err := c.fs.InjectDynamicLink(
		context.Background(),
		c.contentMap[0].path[:2],
	)
	require.ErrorIs(c.T(), err, injectedErr)
}

func (c *CinodeFSMultiFileTestSuite) TestBlobWriteWhenCreatingLink() {
	injectedErr := errors.New("link creation error")
	c.be.updateFunc = func(ctx context.Context, name *common.BlobName, ai *common.AuthInfo, key *common.BlobKey, r io.Reader) error {
		return injectedErr
	}

	_, err := c.fs.InjectDynamicLink(context.Background(), c.contentMap[0].path[:2])
	require.NoError(c.T(), err)

	err = c.fs.Flush(context.Background())
	require.ErrorIs(c.T(), err, injectedErr)
}

func (c *CinodeFSMultiFileTestSuite) TestReadFailureMissingKey() {
	var epProto protobuf.Entrypoint
	err := proto.Unmarshal(
		golang.Must(c.fs.FindEntry(context.Background(), c.contentMap[0].path)).Bytes(),
		&epProto,
	)
	require.NoError(c.T(), err)

	// Generate derived EP without key
	epProto.KeyInfo.Key = nil
	ep := golang.Must(cinodefs.EntrypointFromBytes(
		golang.Must(proto.Marshal(&epProto)),
	))

	// Replace current entrypoint with one without the key
	err = c.fs.SetEntry(context.Background(), c.contentMap[0].path, ep)
	require.NoError(c.T(), err)

	r, err := c.fs.OpenEntryData(context.Background(), c.contentMap[0].path)
	require.ErrorIs(c.T(), err, cinodefs.ErrMissingKeyInfo)
	require.Nil(c.T(), r)
}

func TestFetchingWriterInfo(t *testing.T) {
	t.Run("not a dynamic link", func(t *testing.T) {
		fs, err := cinodefs.New(
			context.Background(),
			blenc.FromDatastore(datastore.InMemory()),
			cinodefs.NewRootStaticDirectory(),
		)
		require.NoError(t, err)

		wi, err := fs.RootWriterInfo(context.Background())
		require.ErrorIs(t, err, cinodefs.ErrModifiedDirectory)
		require.Nil(t, wi)

		err = fs.Flush(context.Background())
		require.NoError(t, err)

		wi, err = fs.RootWriterInfo(context.Background())
		require.ErrorIs(t, err, cinodefs.ErrNotALink)
		require.Nil(t, wi)
	})

	t.Run("dynamic link without writer info", func(t *testing.T) {
		link, err := dynamiclink.Create(rand.Reader)
		require.NoError(t, err)
		ep := cinodefs.EntrypointFromBlobNameAndKey(link.BlobName(), link.EncryptionKey())

		fs, err := cinodefs.New(
			context.Background(),
			blenc.FromDatastore(datastore.InMemory()),
			// Set entrypoint without auth info
			cinodefs.RootEntrypoint(ep),
		)
		require.NoError(t, err)

		wi, err := fs.RootWriterInfo(context.Background())
		require.ErrorIs(t, err, cinodefs.ErrMissingWriterInfo)
		require.Nil(t, wi)
	})
}
