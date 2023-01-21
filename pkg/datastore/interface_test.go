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

package datastore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/stretchr/testify/suite"
)

type DatastoreTestSuite struct {
	suite.Suite
	createDS func() (DS, error)
	ds       DS
}

func TestDatastoreTestSuite(t *testing.T) {
	t.Run("InMemory", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			createDS: func() (DS, error) { return InMemory(), nil },
		})
	})

	t.Run("NewMultiSource", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			createDS: func() (DS, error) { return NewMultiSource(InMemory(), time.Hour), nil },
		})
	})

	t.Run("InFileSystem", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			createDS: func() (DS, error) { return InFileSystem(t.TempDir()) },
		})
	})

	t.Run("InRawFileSystem", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			createDS: func() (DS, error) { return InRawFileSystem(t.TempDir()) },
		})
	})

	t.Run("FromWeb", func(t *testing.T) {
		suite.Run(t, &DatastoreTestSuite{
			createDS: func() (DS, error) {
				server := httptest.NewServer(WebInterface(InMemory()))
				t.Cleanup(func() { server.Close() })

				return FromWeb(server.URL + "/")
			},
		})
	})
}

func (s *DatastoreTestSuite) SetupTest() {
	ds, err := s.createDS()
	s.Require().NoError(err)
	s.ds = ds
}

func (s *DatastoreTestSuite) TestOpenNonExisting() {
	for _, name := range emptyBlobNamesOfAllTypes {
		s.Run(fmt.Sprint(name.Type()), func() {
			r, err := s.ds.Open(context.Background(), name)
			s.Require().ErrorIs(err, ErrNotFound)
			s.Require().Nil(r)
		})
	}
}

func (s *DatastoreTestSuite) TestOpenInvalidBlobType() {
	bn, err := common.BlobNameFromHashAndType(sha256.New().Sum(nil), common.NewBlobType(0xFF))
	s.Require().NoError(err)

	r, err := s.ds.Open(context.Background(), bn)
	s.Require().ErrorIs(err, blobtypes.ErrUnknownBlobType)
	s.Require().Nil(r)

	err = s.ds.Update(context.Background(), bn, bytes.NewBuffer(nil))
	s.Require().ErrorIs(err, blobtypes.ErrUnknownBlobType)
}

func (s *DatastoreTestSuite) TestBlobValidationFailed() {
	for _, name := range emptyBlobNamesOfAllTypes {
		s.Run(fmt.Sprint(name.Type()), func() {
			err := s.ds.Update(context.Background(), name, bytes.NewReader([]byte("test")))
			s.Require().ErrorIs(err, blobtypes.ErrValidationFailed)
		})
	}
}

func (s *DatastoreTestSuite) TestSaveSuccessfulStatic() {
	for _, b := range testBlobs {

		exists, err := s.ds.Exists(context.Background(), b.name)
		s.Require().NoError(err)
		s.Require().False(exists)

		err = s.ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
		s.Require().NoError(err)

		exists, err = s.ds.Exists(context.Background(), b.name)
		s.Require().NoError(err)
		s.Require().True(exists)

		// Overwrite with the same data must be fine
		err = s.ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
		s.Require().NoError(err)

		exists, err = s.ds.Exists(context.Background(), b.name)
		s.Require().NoError(err)
		s.Require().True(exists)

		// Overwrite with wrong data must fail
		err = s.ds.Update(context.Background(), b.name, bytes.NewReader(append([]byte{0x00}, b.data...)))
		s.Require().ErrorIs(err, blobtypes.ErrValidationFailed)

		exists, err = s.ds.Exists(context.Background(), b.name)
		s.Require().NoError(err)
		s.Require().True(exists)

		r, err := s.ds.Open(context.Background(), b.name)
		s.Require().NoError(err)

		data, err := io.ReadAll(r)
		s.Require().NoError(err)
		s.Require().Equal(b.data, data)

		err = r.Close()
		s.Require().NoError(err)
	}
}

func (s *DatastoreTestSuite) TestErrorWhileUpdating() {
	for i, b := range testBlobs {
		s.Run(fmt.Sprint(i), func() {
			errRet := errors.New("Test error")
			err := s.ds.Update(context.Background(), b.name, bReader(b.data, func() error {
				return errRet
			}, nil))
			s.Require().ErrorIs(err, errRet)

			exists, err := s.ds.Exists(context.Background(), b.name)
			s.Require().NoError(err)
			s.Require().False(exists)
		})
	}
}

func (s *DatastoreTestSuite) TestErrorWhileOverwriting() {
	for i, b := range testBlobs {
		s.Run(fmt.Sprint(i), func() {

			err := s.ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
			s.Require().NoError(err)

			errRet := errors.New("cancel")

			err = s.ds.Update(context.Background(), b.name, bReader(b.data, func() error {
				exists, err := s.ds.Exists(context.Background(), b.name)
				s.Require().NoError(err)
				s.Require().True(exists)

				return errRet
			}, nil))

			s.Require().ErrorIs(err, errRet)

			exists, err := s.ds.Exists(context.Background(), b.name)
			s.Require().NoError(err)
			s.Require().True(exists)

			r, err := s.ds.Open(context.Background(), b.name)
			s.Require().NoError(err)

			data, err := io.ReadAll(r)
			s.Require().NoError(err)
			s.Require().Equal(b.data, data)

			err = r.Close()
			s.Require().NoError(err)
		})
	}
}

func (s *DatastoreTestSuite) TestDeleteNonExisting() {
	b := testBlobs[0]

	err := s.ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
	s.Require().NoError(err)

	err = s.ds.Delete(context.Background(), testBlobs[1].name)
	s.Require().ErrorIs(err, ErrNotFound)

	exists, err := s.ds.Exists(context.Background(), b.name)
	s.Require().NoError(err)
	s.Require().True(exists)
}

func (s *DatastoreTestSuite) TestDeleteExisting() {

	b := testBlobs[0]
	err := s.ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
	s.Require().NoError(err)

	exists, err := s.ds.Exists(context.Background(), b.name)
	s.Require().NoError(err)
	s.Require().True(exists)

	err = s.ds.Delete(context.Background(), b.name)
	s.Require().NoError(err)

	exists, err = s.ds.Exists(context.Background(), b.name)
	s.Require().NoError(err)
	s.Require().False(exists)

	r, err := s.ds.Open(context.Background(), b.name)
	s.Require().ErrorIs(err, ErrNotFound)
	s.Require().Nil(r)
}

func (s *DatastoreTestSuite) TestGetKind() {
	k := s.ds.Kind()
	s.Require().NotEmpty(k)
}

func (s *DatastoreTestSuite) TestSimultaneousReads() {
	const threadCnt = 10
	const readCnt = 200

	// Prepare data
	for _, b := range testBlobs {
		err := s.ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
		s.Require().NoError(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(threadCnt)

	for i := 0; i < threadCnt; i++ {
		go func(i int) {
			defer wg.Done()
			for n := 0; n < readCnt; n++ {
				b := testBlobs[(i+n)%len(testBlobs)]

				r, err := s.ds.Open(context.Background(), b.name)
				s.Require().NoError(err)

				data, err := io.ReadAll(r)
				s.Require().NoError(err)
				s.Require().Equal(b.data, data)

				err = r.Close()
				s.Require().NoError(err)
			}
		}(i)
	}

	wg.Wait()
}

func (s *DatastoreTestSuite) TestSimultaneousUpdates() {
	const threadCnt = 3

	b := testBlobs[0]

	wg := sync.WaitGroup{}
	wg.Add(threadCnt)

	for i := 0; i < threadCnt; i++ {
		go func(i int) {
			defer wg.Done()

			err := s.ds.Update(context.Background(), b.name, bytes.NewReader(b.data))
			if errors.Is(err, ErrUploadInProgress) {
				// TODO: We should be able to handle this case
				return
			}

			s.Require().NoError(err)

			exists, err := s.ds.Exists(context.Background(), b.name)
			s.Require().NoError(err)
			s.Require().True(exists)
		}(i)
	}

	wg.Wait()

	exists, err := s.ds.Exists(context.Background(), b.name)
	s.Require().NoError(err)
	s.Require().True(exists)

	r, err := s.ds.Open(context.Background(), b.name)
	s.Require().NoError(err)

	data, err := io.ReadAll(r)
	s.Require().NoError(err)
	s.Require().Equal(b.data, data)

	err = r.Close()
	s.Require().NoError(err)
}

func (s *DatastoreTestSuite) updateDynamicLink(num int) {
	err := s.ds.Update(
		context.Background(),
		dynamicLinkPropagationData[num].name,
		bytes.NewReader(dynamicLinkPropagationData[num].data),
	)
	s.Require().NoError(err)
}

func (s *DatastoreTestSuite) readDynamicLinkData() string {
	r, err := s.ds.Open(context.Background(), dynamicLinkPropagationData[0].name)
	s.Require().NoError(err)

	dl, err := dynamiclink.FromReader(dynamicLinkPropagationData[0].name, r)
	s.Require().NoError(err)

	err = r.Close()
	s.Require().NoError(err)

	return string(dl.EncryptedLink)
}

func (s *DatastoreTestSuite) expectDynamicLinkData(num int) {
	s.Require().Equal(
		dynamicLinkPropagationData[num].expected,
		s.readDynamicLinkData(),
	)
}

func (s *DatastoreTestSuite) TestDynamicLinkPropagation() {
	s.updateDynamicLink(0)
	s.expectDynamicLinkData(0)

	s.updateDynamicLink(1)
	s.expectDynamicLinkData(1)

	s.updateDynamicLink(0)
	s.expectDynamicLinkData(1)

	s.updateDynamicLink(2)
	s.expectDynamicLinkData(1)

	s.updateDynamicLink(1)
	s.expectDynamicLinkData(1)

	s.updateDynamicLink(0)
	s.expectDynamicLinkData(1)
}
