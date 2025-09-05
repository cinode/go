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

package blenc

import (
	"bytes"
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
	"github.com/stretchr/testify/require"
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
	t := s.T()
	data := []byte("Hello world!!!")

	bn, key, ai, err := s.be.Create(
		t.Context(),
		blobtypes.Static,
		bytes.NewReader(data),
	)
	require.NoError(t, err)
	require.Equal(t, blobtypes.Static, bn.Type())
	require.Len(t, bn.Hash(), sha256.Size)
	require.Nil(t, ai) // Static blobs don't generate auth info

	t.Run("check successful operations on a static blob", func(t *testing.T) {
		t.Run("blob must be reported as existing", func(t *testing.T) {
			exists, err := s.be.Exists(t.Context(), bn)
			require.NoError(t, err)
			require.True(t, exists)
		})

		t.Run("must correctly read blob's content", func(t *testing.T) {
			rc, err := s.be.Open(t.Context(), bn, key)
			require.NoError(t, err)

			readData, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.Equal(t, data, readData)

			err = rc.Close()
			require.NoError(t, err)
		})

		t.Run("must correctly delete blob", func(t *testing.T) {
			err := s.be.Delete(t.Context(), bn)
			require.NoError(t, err)

			exists, err := s.be.Exists(t.Context(), bn)
			require.NoError(t, err)
			require.False(t, exists)
		})
	})

	t.Run("work with second static blob", func(t *testing.T) {
		data2 := []byte("Hello Cinode!")

		bn2, key2, ai2, err := s.be.Create(
			t.Context(),
			blobtypes.Static,
			bytes.NewReader(data2),
		)
		require.NoError(t, err)
		require.NotEqual(t, bn, bn2)
		require.Nil(t, ai2)

		t.Run("new static blob must be different from the first one", func(t *testing.T) {
			require.NoError(t, err)
			require.NotEqual(t, key, key2)
			require.Len(t, key2.Bytes(), len(key.Bytes()))
		})

		t.Run("must fail to update static blob", func(t *testing.T) {
			data3 := []byte("Hello Universe!")

			err := s.be.Update(
				t.Context(),
				bn2,
				ai2,
				key2,
				bytes.NewReader(data3),
			)
			require.ErrorIs(t, err, ErrCanNotUpdateStaticBlob)
		})

		t.Run("must fail to open static blob with wrong key", func(t *testing.T) {
			err := func() error {
				rc, err := s.be.Open(t.Context(), bn2, key)
				if err != nil {
					return err
				}

				_, err = io.ReadAll(rc)
				if err != nil {
					return err
				}

				return rc.Close()
			}()
			require.ErrorIs(t, err, blobtypes.ErrValidationFailed)
		})

		t.Run("must fail to open static blob with invalid key", func(t *testing.T) {
			brokenKey := common.BlobKeyFromBytes(key2.Bytes()[1:])
			rc, err := s.be.Open(t.Context(), bn2, brokenKey)
			require.ErrorIs(t, err, cipherfactory.ErrInvalidEncryptionConfig)
			require.Nil(t, rc)
		})
	})
}

func (s *BlencTestSuite) TestDynamicLinkSuccessPath() {
	t := s.T()

	data := []byte("Hello world!!!")

	bn, key, ai, err := s.be.Create(
		t.Context(),
		blobtypes.DynamicLink,
		bytes.NewReader(data),
	)
	require.NoError(t, err)
	require.Equal(t, blobtypes.DynamicLink, bn.Type())
	require.Len(t, bn.Hash(), sha256.Size)
	require.NotNil(t, ai)

	t.Run("check successful operations on a dynamic link", func(t *testing.T) {
		t.Run("blob must be reported as existing", func(t *testing.T) {
			exists, err := s.be.Exists(t.Context(), bn)
			require.NoError(t, err)
			require.True(t, exists)
		})

		t.Run("must correctly read blob's content", func(t *testing.T) {
			rc, err := s.be.Open(t.Context(), bn, key)
			require.NoError(t, err)

			readData, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.Equal(t, data, readData)

			err = rc.Close()
			require.NoError(t, err)
		})

		t.Run("must correctly delete blob", func(t *testing.T) {
			err := s.be.Delete(t.Context(), bn)
			require.NoError(t, err)

			exists, err := s.be.Exists(t.Context(), bn)
			require.NoError(t, err)
			require.False(t, exists)
		})
	})

	t.Run("work with second dynamic link", func(t *testing.T) {
		data2 := []byte("Hello Cinode!")

		bn2, key2, ai2, err := s.be.Create(
			t.Context(),
			blobtypes.DynamicLink,
			bytes.NewReader(data2),
		)
		require.NoError(t, err)
		require.NotEqual(t, bn, bn2)
		require.NotNil(t, ai2)

		t.Run("new dynamic link must be different from the first one", func(t *testing.T) {
			require.NoError(t, err)
			require.NotEqual(t, key, key2)
			require.Len(t, key2.Bytes(), len(key.Bytes()))
		})

		t.Run("must correctly read blob's content", func(t *testing.T) {
			rc, err := s.be.Open(t.Context(), bn2, key2)
			require.NoError(t, err)

			readData, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.Equal(t, data2, readData)

			err = rc.Close()
			require.NoError(t, err)
		})

		t.Run("must correctly update dynamic link", func(t *testing.T) {
			data3 := []byte("Hello Universe!")

			err = s.be.Update(t.Context(), bn2, ai2, key2, bytes.NewReader(data3))
			require.NoError(t, err)

			rc, err := s.be.Open(t.Context(), bn2, key2)
			require.NoError(t, err)

			readData, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.Equal(t, data3, readData)

			err = rc.Close()
			require.NoError(t, err)
		})

		t.Run("must fail to update if encryption key is invalid", func(t *testing.T) {
			err := s.be.Update(
				t.Context(),
				bn2,
				ai2,
				key,
				bytes.NewReader(nil),
			)
			require.ErrorIs(t, err, ErrDynamicLinkUpdateFailed)
			require.ErrorIs(t, err, ErrDynamicLinkUpdateFailedWrongKey)
		})

		t.Run("must fail to update if blob name is invalid", func(t *testing.T) {
			err := s.be.Update(
				t.Context(),
				bn,
				ai2,
				key2,
				bytes.NewReader(nil),
			)
			require.ErrorIs(t, err, ErrDynamicLinkUpdateFailed)
			require.ErrorIs(t, err, ErrDynamicLinkUpdateFailedWrongName)
		})

		t.Run("must fail to update if auth info is invalid", func(t *testing.T) {
			brokenAI2 := common.AuthInfoFromBytes(ai2.Bytes()[1:])
			err := s.be.Update(
				t.Context(),
				bn,
				brokenAI2,
				key2,
				bytes.NewReader(nil),
			)
			require.ErrorIs(t, err, dynamiclink.ErrInvalidDynamicLinkAuthInfo)
		})

		t.Run("must fail to update link on read errors", func(t *testing.T) {
			injectedErr := errors.New("test")

			err := s.be.Update(
				t.Context(),
				bn,
				ai2,
				key2,
				iotest.ErrReader(injectedErr),
			)
			require.ErrorIs(t, err, injectedErr)
		})
	})

	t.Run("must fail to create link on read errors", func(t *testing.T) {
		injectedErr := errors.New("test")

		bn, key, ai, err := s.be.Create(
			t.Context(),
			blobtypes.DynamicLink,
			iotest.ErrReader(injectedErr),
		)
		require.ErrorIs(t, err, injectedErr)
		require.Empty(t, bn)
		require.Empty(t, key)
		require.Empty(t, ai)
	})
}

func (s *BlencTestSuite) TestInvalidBlobTypes() {
	t := s.T()

	invalidBlobName, err := common.BlobNameFromHashAndType(sha256.New().Sum(nil), blobtypes.Invalid)
	require.NoError(t, err)

	t.Run("must fail to create blob of invalid type", func(t *testing.T) {
		bn, key, ai, err := s.be.Create(
			t.Context(),
			blobtypes.Invalid,
			bytes.NewReader(nil),
		)
		require.ErrorIs(t, err, blobtypes.ErrUnknownBlobType)
		require.Empty(t, bn)
		require.Empty(t, key)
		require.Empty(t, ai)
	})

	t.Run("must fail to open blob of invalid type", func(t *testing.T) {
		rc, err := s.be.Open(
			t.Context(),
			invalidBlobName,
			nil,
		)
		require.ErrorIs(t, err, blobtypes.ErrUnknownBlobType)
		require.Nil(t, rc)
	})

	t.Run("must fail to update blob of invalid type", func(t *testing.T) {
		err = s.be.Update(
			t.Context(),
			invalidBlobName,
			nil,
			nil,
			bytes.NewReader(nil),
		)
		require.ErrorIs(t, err, blobtypes.ErrUnknownBlobType)
	})
}
