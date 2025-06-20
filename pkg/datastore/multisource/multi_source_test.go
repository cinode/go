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

package multisource

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/stretchr/testify/suite"
)

func TestInterface(t *testing.T) {
	suite.Run(t, &datastore.DatastoreTestSuite{
		CreateDS: func() (datastore.DS, error) {
			return New(datastore.InMemory()), nil
		},
	})
}

type MultiSourceDatastoreTestSuite struct {
	suite.Suite
}

func TestMultiSourceDatastore(t *testing.T) {
	suite.Run(t, &MultiSourceDatastoreTestSuite{})
}

func (s *MultiSourceDatastoreTestSuite) addStaticBlob(ds datastore.DS, c string) *common.BlobName {
	hash := sha256.Sum256([]byte(c))
	name, err := common.BlobNameFromHashAndType(hash[:], blobtypes.Static)
	s.Require().NoError(err)
	err = ds.Update(context.Background(), name, bytes.NewReader([]byte(c)))
	s.Require().NoError(err)
	return name
}

func (s *MultiSourceDatastoreTestSuite) fetchBlob(ds datastore.DS, n *common.BlobName) string {
	rc, err := ds.Open(context.Background(), n)
	s.Require().NoError(err)

	data, err := io.ReadAll(rc)
	s.Require().NoError(err)

	err = rc.Close()
	s.Require().NoError(err)

	return string(data)
}

func (s *MultiSourceDatastoreTestSuite) ensureNotFound(ds datastore.DS, n *common.BlobName) {
	_, err := ds.Open(context.Background(), n)
	s.Require().ErrorIs(err, datastore.ErrNotFound)
}

func (s *MultiSourceDatastoreTestSuite) TestStaticBlobPropagation() {
	main := datastore.InMemory()
	add1 := datastore.InMemory()
	add2 := datastore.InMemory()

	ds := New(
		main,
		WithDynamicDataRefreshTime(time.Hour),
		WithAdditionalDatastores(add1, add2),
	).(*multiSourceDatastore)

	bn1 := s.addStaticBlob(add1, "Hello world 1")
	bn2 := s.addStaticBlob(add2, "Hello world 2")

	s.Require().EqualValues("Hello world 1", s.fetchBlob(ds, bn1))
	s.Require().EqualValues("Hello world 2", s.fetchBlob(ds, bn2))

	// Blobs should already be in the cache
	ds.additional = []datastore.DS{}

	s.Require().EqualValues("Hello world 1", s.fetchBlob(ds, bn1))
	s.Require().EqualValues("Hello world 2", s.fetchBlob(ds, bn2))
}

func (s *MultiSourceDatastoreTestSuite) TestNotFoundRecheck() {
	main := datastore.InMemory()
	add := datastore.InMemory()

	bn := s.addStaticBlob(datastore.InMemory(), "Hello world")

	ds := New(
		main,
		WithDynamicDataRefreshTime(time.Hour),
		WithNotFoundRecheckTime(time.Millisecond*10),
		WithAdditionalDatastores(add),
	).(*multiSourceDatastore)

	// Try to fetch the blob while it is not found in the additional datastore,
	s.ensureNotFound(ds, bn)

	// Add the blob to the additional datastore
	s.addStaticBlob(add, "Hello world")

	// The not found state should be cached for a while
	for i := 0; i < 10; i++ {
		// Result still cached
		s.ensureNotFound(ds, bn)
	}

	time.Sleep(time.Millisecond * 20)

	// Should refresh by now
	s.Require().EqualValues("Hello world", s.fetchBlob(ds, bn))
}

func (s *MultiSourceDatastoreTestSuite) TestDynamicLinkRefresh() {
	main := datastore.InMemory()
	add := datastore.InMemory()

	ds := New(
		main,
		WithDynamicDataRefreshTime(time.Millisecond*10),
		WithNotFoundRecheckTime(time.Hour),
		WithAdditionalDatastores(add),
	).(*multiSourceDatastore)

	// Create a new dynamic link
	dlp, err := dynamiclink.Create(rand.Reader)
	s.Require().NoError(err)

	bn := dlp.BlobName()

	// Upload the data to the additional datastore
	key := s.updateLink(dlp, 1, add, "Hello world")

	// The data should be available in the main datastore
	s.Require().EqualValues("Hello world", s.fetchLink(ds, bn, key))

	// Update the data in the additional datastore
	s.updateLink(dlp, 2, add, "Hello world 2")

	// Old link data should be cached
	for range 10 {
		s.Require().EqualValues("Hello world", s.fetchLink(ds, bn, key))
	}

	time.Sleep(time.Millisecond * 20)

	// The updated data should be available in the main datastore by now
	s.Require().EqualValues("Hello world 2", s.fetchLink(ds, bn, key))

}

func (s *MultiSourceDatastoreTestSuite) updateLink(
	dlp *dynamiclink.Publisher,
	version uint64,
	ds datastore.DS,
	content string,
) *common.BlobKey {
	pr, key, err := dlp.UpdateLinkData(bytes.NewReader([]byte(content)), version)
	s.Require().NoError(err)

	err = ds.Update(context.Background(), dlp.BlobName(), pr.GetPublicDataReader())
	s.Require().NoError(err)

	return key
}

func (s *MultiSourceDatastoreTestSuite) fetchLink(
	ds datastore.DS,
	bn *common.BlobName,
	key *common.BlobKey,
) string {
	rdr, err := ds.Open(context.Background(), bn)
	s.Require().NoError(err)

	defer rdr.Close()

	pr, err := dynamiclink.FromPublicData(bn, rdr)
	s.Require().NoError(err)

	lrdr, err := pr.GetLinkDataReader(key)
	s.Require().NoError(err)

	data, err := io.ReadAll(lrdr)
	s.Require().NoError(err)

	return string(data)
}

type testSyncReaderDS struct {
	datastore.DS
	waitChan  chan struct{}
	openCount atomic.Int32
}

func (s *testSyncReaderDS) Open(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
	s.openCount.Add(1)
	<-s.waitChan
	return s.DS.Open(ctx, name)
}

func (s *MultiSourceDatastoreTestSuite) TestParallelDownloads() {
	main := datastore.InMemory()
	add := datastore.InMemory()
	addSync := &testSyncReaderDS{DS: add, waitChan: make(chan struct{})}

	ds := New(
		main,
		WithAdditionalDatastores(addSync),
		WithDynamicDataRefreshTime(time.Hour),
	).(*multiSourceDatastore)
	bn := s.addStaticBlob(add, "Hello world")

	startWg := sync.WaitGroup{}
	finishWg := sync.WaitGroup{}
	goroutinesDone := atomic.Int32{}

	for range 5 {
		startWg.Add(1)
		finishWg.Add(1)

		go func() {
			defer finishWg.Done()
			defer goroutinesDone.Add(1)

			startWg.Done()
			rc, err := ds.Open(context.Background(), bn)
			s.Require().NoError(err)
			err = rc.Close()
			s.Require().NoError(err)
		}()
	}

	startWg.Wait()

	s.Require().Eventually(
		func() bool { return addSync.openCount.Load() == 1 },
		time.Second, time.Millisecond,
		"must start downloading the blob from additional datastore",
	)

	s.Require().Zero(goroutinesDone.Load())

	close(addSync.waitChan)

	finishWg.Wait()

	s.Require().EqualValues(
		1, addSync.openCount.Load(),
		"download should be started once for all goroutinesS",
	)
}
