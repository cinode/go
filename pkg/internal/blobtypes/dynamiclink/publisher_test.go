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

package dynamiclink

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/cinode/go/pkg/common"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		dl, err := Create(rand.Reader)
		require.NoError(t, err)
		require.NotNil(t, dl)

		require.NotEmpty(t, dl.privKey)
		require.NotEmpty(t, dl.publicKey)
		require.NotZero(t, dl.nonce)
	})

	t.Run("error from rand source", func(t *testing.T) {
		for goodBytes := 0; goodBytes < ed25519.SeedSize+8; goodBytes++ {
			injectedErr := errors.New("test")
			r := io.MultiReader(
				io.LimitReader(rand.Reader, int64(goodBytes)),
				iotest.ErrReader(injectedErr),
			)

			dl, err := Create(r)
			require.ErrorIs(t, err, injectedErr)
			require.Nil(t, dl)
		}
	})
}

func TestFromAuthInfo(t *testing.T) {
	dl1, err := Create(rand.Reader)
	require.NoError(t, err)

	authInfo := dl1.AuthInfo()

	t.Run("valid auth info", func(t *testing.T) {
		dl2, err := FromAuthInfo(authInfo)
		require.NoError(t, err)
		require.Equal(t, dl1.privKey, dl2.privKey)
		require.Equal(t, dl1.publicKey, dl2.publicKey)
		require.Equal(t, dl1.nonce, dl2.nonce)
		require.Equal(t, dl1.BlobName(), dl2.BlobName())
	})

	t.Run("Invalid auth info", func(t *testing.T) {
		authInfoBytes := authInfo.Bytes()
		for i := 0; i < len(authInfoBytes)-1; i++ {
			brokenAuthInfo := common.AuthInfoFromBytes(authInfoBytes[:i])
			dl2, err := FromAuthInfo(brokenAuthInfo)
			require.ErrorIs(t, err, ErrInvalidDynamicLinkAuthInfo)
			require.Nil(t, dl2)
		}
	})
}

func TestReNonce(t *testing.T) {
	dl1, err := Create(rand.Reader)
	require.NoError(t, err)

	t.Run("successful renonce", func(t *testing.T) {
		dl2, err := ReNonce(dl1, rand.Reader)
		require.NoError(t, err)
		require.Equal(t, dl1.privKey, dl2.privKey)
		require.Equal(t, dl1.publicKey, dl2.publicKey)
		require.NotEqual(t, dl1.nonce, dl2.nonce)
		require.NotEqual(t, dl1.BlobName(), dl2.BlobName())
	})

	t.Run("failed random source", func(t *testing.T) {
		for goodBytes := 0; goodBytes < 8; goodBytes++ {
			injectedErr := errors.New("test")
			r := io.MultiReader(
				io.LimitReader(rand.Reader, int64(goodBytes)),
				iotest.ErrReader(injectedErr),
			)

			dl2, err := ReNonce(dl1, r)
			require.ErrorIs(t, err, injectedErr)
			require.Nil(t, dl2)
		}
	})
}

func TestPublisherUpdateLinkData(t *testing.T) {
	dl, err := Create(rand.Reader)
	require.NoError(t, err)

	pr, key, err := dl.UpdateLinkData(io.LimitReader(rand.Reader, 32), 0)
	require.NoError(t, err)
	require.NotNil(t, key)
	require.NotNil(t, pr.r)
	require.NotNil(t, pr.iv)
	require.NotNil(t, pr.signature)
	require.EqualValues(t, 0, pr.contentVersion)

	t.Run("successful update", func(t *testing.T) {
		pr2, key2, err := dl.UpdateLinkData(io.LimitReader(rand.Reader, 32), 1)
		require.NoError(t, err)
		require.Equal(t, key, key2)
		require.EqualValues(t, 1, pr2.contentVersion)
	})

	t.Run("failed data reader", func(t *testing.T) {
		injectedErr := errors.New("test")
		pr2, key2, err := dl.UpdateLinkData(iotest.ErrReader(injectedErr), 3)
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, pr2)
		require.Nil(t, key2)
	})
}
