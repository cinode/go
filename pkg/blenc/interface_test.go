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

package blenc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"github.com/stretchr/testify/suite"
)

type BlencTestSuite struct {
	suite.Suite
	be BE
}

func TestBlencTestSuite(t *testing.T) {
	suite.Run(t, &BlencTestSuite{
		be: FromDatastore(datastore.InMemory()),
	})
}

func (s *BlencTestSuite) TestStaticBlobs() {
	data := []byte("Hello world!!!")

	bn, key, ai, err := s.be.Create(context.Background(), blobtypes.Static, bytes.NewReader(data))
	s.Require().NoError(err)
	s.Require().Equal(blobtypes.Static, bn.Type())
	s.Require().Len(bn.Hash(), sha256.Size)
	s.Require().Nil(ai) // Static blobs don't generate auth info

	s.Run("check successful operations on a static blob", func() {
		s.Run("blob must be reported as existing", func() {
			exists, err := s.be.Exists(context.Background(), bn)
			s.Require().NoError(err)
			s.Require().True(exists)
		})

		s.Run("must correctly read blob's content", func() {
			rc, err := s.be.Open(context.Background(), bn, key)
			s.Require().NoError(err)

			readData, err := io.ReadAll(rc)
			s.Require().NoError(err)
			s.Require().Equal(data, readData)

			err = rc.Close()
			s.Require().NoError(err)
		})

		s.Run("must correctly delete blob", func() {
			err := s.be.Delete(context.Background(), bn)
			s.Require().NoError(err)

			exists, err := s.be.Exists(context.Background(), bn)
			s.Require().NoError(err)
			s.Require().False(exists)
		})
	})

	s.Run("work with second static blob", func() {
		data2 := []byte("Hello Cinode!")

		bn2, key2, ai2, err := s.be.Create(context.Background(), blobtypes.Static, bytes.NewReader(data2))
		s.Require().NoError(err)
		s.Require().NotEqual(bn, bn2)
		s.Require().Nil(ai2)

		s.Run("new static blob must be different from the first one", func() {
			s.Require().NoError(err)
			s.Require().NotEqual(key, key2)
			s.Require().Len(key2.Bytes(), len(key.Bytes()))
		})

		s.Run("must fail to update static blob", func() {
			data3 := []byte("Hello Universe!")

			err := s.be.Update(context.Background(), bn2, ai2, key2, bytes.NewReader(data3))
			s.Require().ErrorIs(err, ErrCanNotUpdateStaticBlob)
		})

		s.Run("must fail to open static blob with wrong key", func() {
			err := func() error {
				rc, err := s.be.Open(context.Background(), bn2, key)
				if err != nil {
					return err
				}

				_, err = io.ReadAll(rc)
				if err != nil {
					return err
				}

				return rc.Close()
			}()
			s.Require().ErrorIs(err, blobtypes.ErrValidationFailed)
		})

		s.Run("must fail to open static blob with invalid key", func() {
			brokenKey := common.BlobKeyFromBytes(key2.Bytes()[1:])
			rc, err := s.be.Open(context.Background(), bn2, brokenKey)
			s.Require().ErrorIs(err, cipherfactory.ErrInvalidEncryptionConfig)
			s.Require().Nil(rc)
		})
	})

}

func (s *BlencTestSuite) TestDynamicLinkSuccessPath() {
	data := []byte("Hello world!!!")

	bn, key, ai, err := s.be.Create(context.Background(), blobtypes.DynamicLink, bytes.NewReader(data))
	s.Require().NoError(err)
	s.Require().Equal(blobtypes.DynamicLink, bn.Type())
	s.Require().Len(bn.Hash(), sha256.Size)
	s.Require().NotNil(ai)

	s.Run("check successful operations on a dynamic link", func() {
		s.Run("blob must be reported as existing", func() {
			exists, err := s.be.Exists(context.Background(), bn)
			s.Require().NoError(err)
			s.Require().True(exists)
		})

		s.Run("must correctly read blob's content", func() {
			rc, err := s.be.Open(context.Background(), bn, key)
			s.Require().NoError(err)

			readData, err := io.ReadAll(rc)
			s.Require().NoError(err)
			s.Require().Equal(data, readData)

			err = rc.Close()
			s.Require().NoError(err)
		})

		s.Run("must correctly delete blob", func() {
			err := s.be.Delete(context.Background(), bn)
			s.Require().NoError(err)

			exists, err := s.be.Exists(context.Background(), bn)
			s.Require().NoError(err)
			s.Require().False(exists)
		})
	})

	s.Run("work with second dynamic link", func() {

		data2 := []byte("Hello Cinode!")

		bn2, key2, ai2, err := s.be.Create(context.Background(), blobtypes.DynamicLink, bytes.NewReader(data2))
		s.Require().NoError(err)
		s.Require().NotEqual(bn, bn2)
		s.Require().NotNil(ai2)

		s.Run("new dynamic link must be different from the first one", func() {
			s.Require().NoError(err)
			s.Require().NotEqual(key, key2)
			s.Require().Len(key2.Bytes(), len(key.Bytes()))
		})

		s.Run("must correctly read blob's content", func() {
			rc, err := s.be.Open(context.Background(), bn2, key2)
			s.Require().NoError(err)

			readData, err := io.ReadAll(rc)
			s.Require().NoError(err)
			s.Require().Equal(data2, readData)

			err = rc.Close()
			s.Require().NoError(err)
		})

		s.Run("must correctly update dynamic link", func() {
			data3 := []byte("Hello Universe!")

			err = s.be.Update(context.Background(), bn2, ai2, key2, bytes.NewReader(data3))
			s.Require().NoError(err)

			rc, err := s.be.Open(context.Background(), bn2, key2)
			s.Require().NoError(err)

			readData, err := io.ReadAll(rc)
			s.Require().NoError(err)
			s.Require().Equal(data3, readData)

			err = rc.Close()
			s.Require().NoError(err)
		})

		s.Run("must fail to update if encryption key is invalid", func() {
			err := s.be.Update(context.Background(), bn2, ai2, key, bytes.NewReader(nil))
			s.Require().ErrorIs(err, ErrDynamicLinkUpdateFailed)
			s.Require().ErrorIs(err, ErrDynamicLinkUpdateFailedWrongKey)
		})

		s.Run("must fail to update if blob name is invalid", func() {
			err := s.be.Update(context.Background(), bn, ai2, key2, bytes.NewReader(nil))
			s.Require().ErrorIs(err, ErrDynamicLinkUpdateFailed)
			s.Require().ErrorIs(err, ErrDynamicLinkUpdateFailedWrongName)
		})

		s.Run("must fail to update if auth info is invalid", func() {
			err := s.be.Update(context.Background(), bn, ai2[1:], key2, bytes.NewReader(nil))
			s.Require().ErrorIs(err, dynamiclink.ErrInvalidDynamicLinkAuthInfo)
		})

		s.Run("must fail to update link on read errors", func() {
			injectedErr := errors.New("test")

			err := s.be.Update(context.Background(), bn, ai2, key2, iotest.ErrReader(injectedErr))
			s.Require().ErrorIs(err, injectedErr)
		})

	})

	s.Run("must fail to create link on read errors", func() {
		injectedErr := errors.New("test")

		bn, key, ai, err := s.be.Create(context.Background(), blobtypes.DynamicLink, iotest.ErrReader(injectedErr))
		s.Require().ErrorIs(err, injectedErr)
		s.Require().Empty(bn)
		s.Require().Empty(key)
		s.Require().Empty(ai)
	})
}

func (s *BlencTestSuite) TestInvalidBlobTypes() {
	invalidBlobName, err := common.BlobNameFromHashAndType(sha256.New().Sum(nil), blobtypes.Invalid)
	s.Require().NoError(err)

	s.Run("must fail to create blob of invalid type", func() {
		bn, key, ai, err := s.be.Create(context.Background(), blobtypes.Invalid, bytes.NewReader(nil))
		s.Require().ErrorIs(err, blobtypes.ErrUnknownBlobType)
		s.Require().Empty(bn)
		s.Require().Empty(key)
		s.Require().Empty(ai)
	})

	s.Run("must fail to open blob of invalid type", func() {
		rc, err := s.be.Open(
			context.Background(),
			invalidBlobName,
			nil,
		)
		s.Require().ErrorIs(err, blobtypes.ErrUnknownBlobType)
		s.Require().Nil(rc)
	})

	s.Run("must fail to update blob of invalid type", func() {
		err = s.be.Update(
			context.Background(),
			invalidBlobName,
			AuthInfo{},
			nil,
			bytes.NewReader(nil),
		)
		s.Require().ErrorIs(err, blobtypes.ErrUnknownBlobType)
	})
}
