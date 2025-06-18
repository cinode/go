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

package datastore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/stretchr/testify/suite"
)

type MultiSourceDatastoreTestSuite struct {
	suite.Suite
}

func TestMultiSourceDatastore(t *testing.T) {
	suite.Run(t, &MultiSourceDatastoreTestSuite{})
}

func (s *MultiSourceDatastoreTestSuite) addBlob(ds DS, c string) *common.BlobName {
	hash := sha256.Sum256([]byte(c))
	name, err := common.BlobNameFromHashAndType(hash[:], blobtypes.Static)
	s.Require().NoError(err)
	err = ds.Update(context.Background(), name, bytes.NewReader([]byte(c)))
	s.Require().NoError(err)
	return name
}

func (s *MultiSourceDatastoreTestSuite) fetchBlob(ds DS, n *common.BlobName) string {
	rc, err := ds.Open(context.Background(), n)
	s.Require().NoError(err)

	data, err := io.ReadAll(rc)
	s.Require().NoError(err)

	err = rc.Close()
	s.Require().NoError(err)

	return string(data)
}

func (s *MultiSourceDatastoreTestSuite) ensureNotFound(ds DS, n *common.BlobName) {
	_, err := ds.Open(context.Background(), n)
	s.Require().ErrorIs(err, ErrNotFound)
}

func (s *MultiSourceDatastoreTestSuite) TestStaticLinkPropagation() {
	main := InMemory()
	add1 := InMemory()
	add2 := InMemory()

	ds := NewMultiSource(main, time.Hour, add1, add2).(*multiSourceDatastore)

	bn1 := s.addBlob(add1, "Hello world 1")
	bn2 := s.addBlob(add2, "Hello world 2")

	s.Require().EqualValues("Hello world 1", s.fetchBlob(ds, bn1))
	s.Require().EqualValues("Hello world 2", s.fetchBlob(ds, bn2))

	// Blobs should already be in the cache
	ds.additional = []DS{}

	s.Require().EqualValues("Hello world 1", s.fetchBlob(ds, bn1))
	s.Require().EqualValues("Hello world 2", s.fetchBlob(ds, bn2))
}

func (s *MultiSourceDatastoreTestSuite) TestLinkRefresh() {
	main := InMemory()
	add := InMemory()

	bn := s.addBlob(InMemory(), "Hello world")

	ds := NewMultiSource(main, time.Millisecond*10, add).(*multiSourceDatastore)
	s.ensureNotFound(ds, bn)

	s.addBlob(add, "Hello world")
	for i := 0; i < 10; i++ {
		// Result still cached
		s.ensureNotFound(ds, bn)
	}

	time.Sleep(time.Millisecond * 20)

	// Should refresh by now
	s.Require().EqualValues("Hello world", s.fetchBlob(ds, bn))
}

type testSyncReaderDS struct {
	DS
	waitChan  chan struct{}
	openCount atomic.Int32
}

func (s *testSyncReaderDS) Open(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
	s.openCount.Add(1)
	<-s.waitChan
	return s.DS.Open(ctx, name)
}

func (s *MultiSourceDatastoreTestSuite) TestParallelDownloads() {
	main := InMemory()
	add := InMemory()
	addSync := &testSyncReaderDS{DS: add, waitChan: make(chan struct{})}

	ds := NewMultiSource(main, time.Hour, addSync).(*multiSourceDatastore)
	bn := s.addBlob(add, "Hello world")

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
