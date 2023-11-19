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

package dynamiclink

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	math_rand "math/rand"
	"sort"
	"testing"
	"testing/iotest"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"github.com/stretchr/testify/require"
)

func TestFromPublicData(t *testing.T) {
	t.Run("Ensure we don't crash on truncated data", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			data := make([]byte, i)
			dl, err := FromPublicData(&common.BlobName{}, bytes.NewReader(data))
			require.ErrorIs(t, err, ErrInvalidDynamicLinkData)
			require.Nil(t, dl)
		}
	})

	t.Run("Do not accept the link if reserved byte is not zero", func(t *testing.T) {
		data := []byte{0xFF, 0, 0, 0}
		dl, err := FromPublicData(&common.BlobName{}, bytes.NewReader(data))
		require.ErrorIs(t, err, ErrInvalidDynamicLinkData)
		require.ErrorIs(t, err, ErrInvalidDynamicLinkDataReservedByte)
		require.Nil(t, dl)
	})

	t.Run("fail when reading link data", func(t *testing.T) {

		dl, err := Create(rand.Reader)
		require.NoError(t, err)

		pr, key, err := dl.UpdateLinkData(bytes.NewReader(nil), 0)
		require.NoError(t, err)
		require.NotEmpty(t, key)

		r := pr.GetPublicDataReader()
		require.NotNil(t, r)

		data, err := io.ReadAll(r)
		require.NoError(t, err)

		injectedErr := errors.New("test")

		for validBytes := range data {
			t.Run(fmt.Sprint(validBytes), func(t *testing.T) {
				rdr := io.MultiReader(
					bytes.NewReader(data[:validBytes]),
					iotest.ErrReader(injectedErr),
				)

				dl, err := FromPublicData(dl.BlobName(), rdr)
				if err == nil {
					// Error is not returned directly from the method but
					// will appear later while trying to read the link data
					_, err = io.ReadAll(dl.GetEncryptedLinkReader())
					require.ErrorIs(t, err, injectedErr)
				} else {
					require.ErrorIs(t, err, injectedErr)
					require.Nil(t, dl)
				}
			})
		}
	})

	t.Run("valid link serialization and deserialization", func(t *testing.T) {
		dl, err := Create(rand.Reader)
		require.NoError(t, err)

		pr, key, err := dl.UpdateLinkData(bytes.NewReader(nil), 0)
		require.NoError(t, err)
		require.NotEmpty(t, key)

		r := pr.GetPublicDataReader()
		require.NotNil(t, r)

		dl2, err := FromPublicData(dl.BlobName(), r)
		require.NoError(t, err)
		require.Equal(t, dl.publicKey, dl2.publicKey)
	})

	t.Run("link data corruption", func(t *testing.T) {
		t.Run("invalid signature", func(t *testing.T) {
			// Valid signature can only be created by the owner of the private key
			// If the signature does not match, there's an indication that someone tries
			// to fake signature expecting network nodes to accept it

			dl, err := Create(rand.Reader)
			require.NoError(t, err)

			pr, key, err := dl.UpdateLinkData(bytes.NewReader(nil), 0)
			require.NoError(t, err)
			require.NotEmpty(t, key)

			pr.signature[len(pr.signature)/2] ^= 0x01 // Flipping just a single bit in the signature

			r := pr.GetPublicDataReader()
			require.NotNil(t, r)

			dl2, err := FromPublicData(dl.BlobName(), r)
			require.NoError(t, err)

			_, err = io.ReadAll(dl2.GetEncryptedLinkReader())
			require.ErrorIs(t, err, ErrInvalidDynamicLinkData)
			require.ErrorIs(t, err, ErrInvalidDynamicLinkDataSignature)
		})

		t.Run("invalid blob name", func(t *testing.T) {
			// The link data itself can be correct, but the authenticity is also
			// validated through the blob name.
			dlKeyPair1, err := Create(rand.Reader)
			require.NoError(t, err)

			dlKeyPair2, err := Create(rand.Reader)
			require.NoError(t, err)
			dlKeyPair2.nonce = dlKeyPair1.nonce // Ensure the nonce is the same

			dlNonce1, err := Create(rand.Reader)
			require.NoError(t, err)

			dlNonce2, err := ReNonce(dlNonce1, rand.Reader)
			require.NoError(t, err)

			for _, d := range []struct {
				name string
				dl1  *Publisher
				dl2  *Publisher
			}{
				{
					// changing key pair must generate different blog name.
					// Otherwise one could potentially reuse existing blob name
					// and swap out to own key pair
					name: "key pair mismatch",
					dl1:  dlKeyPair1,
					dl2:  dlKeyPair2,
				},
				{
					// changing key pair must generate different blog name.
					// Otherwise one could potentially reuse existing blob name
					// and swap out to own key pair
					name: "nonce mismatch",
					dl1:  dlNonce1,
					dl2:  dlNonce2,
				},
			} {
				t.Run(d.name, func(t *testing.T) {

					pr1, key, err := d.dl1.UpdateLinkData(bytes.NewReader(nil), 0)
					require.NoError(t, err)
					require.NotEmpty(t, key)

					_, key, err = d.dl2.UpdateLinkData(bytes.NewReader(nil), 0)
					require.NoError(t, err)
					require.NotEmpty(t, key)

					// Name from the dl2 link, but content from the dl1 one
					r := pr1.GetPublicDataReader()
					require.NotNil(t, r)

					dl1data, err := io.ReadAll(r)
					require.NoError(t, err)

					dl3, err := FromPublicData(d.dl2.BlobName(), bytes.NewReader(dl1data))
					require.ErrorIs(t, err, ErrInvalidDynamicLinkData)
					require.ErrorIs(t, err, ErrInvalidDynamicLinkDataBlobName)
					require.Nil(t, dl3)

					// Just as a sanity check - with correct blob name it must work
					dl4, err := FromPublicData(d.dl1.BlobName(), bytes.NewReader(dl1data))
					require.NoError(t, err)
					require.NotNil(t, dl4)
				})
			}

		})
	})
}

func TestGreaterThan(t *testing.T) {

	const testSize = 100
	const versions = 10
	const reSortingCount = 10

	links := make([]*PublicReader, testSize)

	{
		// Generate dataset, links will have the same blob name different contents,
		// versions do overlap

		baseLink, err := Create(rand.Reader)
		require.NoError(t, err)

		for i := 0; i < testSize; i++ {
			pr, _, err := baseLink.UpdateLinkData(io.LimitReader(rand.Reader, 16), uint64(i%versions))
			require.NoError(t, err)

			dl, err := FromPublicData(baseLink.BlobName(), pr.GetPublicDataReader())
			require.NoError(t, err)

			links[i] = dl
		}
	}

	// Sort links, if the order is not deterministic (strict ordering),
	// there may be a panic or infinite loop here,
	// both cases will be caught by test runtime
	sort.Slice(links, func(i, j int) bool {
		return links[j].GreaterThan(links[i])
	})

	// Check elements in the sorter links array
	for i := 1; i < len(links); i++ {
		require.True(t, links[i].GreaterThan(links[i-1]))
		require.False(t, links[i-1].GreaterThan(links[i]))

		// Versions must be deterministically ordered
		require.GreaterOrEqual(t, links[i].contentVersion, links[i-1].contentVersion)
	}

	// Try sorting the array multiple times, the result must always be the same
	rnd := math_rand.New(math_rand.NewSource(int64(links[0].nonce)))

	for i := 0; i < reSortingCount; i++ {
		linksCopy := make([]*PublicReader, len(links))
		copy(linksCopy, links)

		rnd.Shuffle(len(linksCopy), func(i, j int) { linksCopy[i], linksCopy[j] = linksCopy[j], linksCopy[i] })

		require.NotEqual(t, links, linksCopy)

		sort.Slice(linksCopy, func(i, j int) bool {
			return linksCopy[j].GreaterThan(linksCopy[i])
		})

		require.Equal(t, links, linksCopy)
	}
}

func TestPublicReaderGetPublicDataReader(t *testing.T) {
	link, err := Create(rand.Reader)
	require.NoError(t, err)

	pr, _, err := link.UpdateLinkData(bytes.NewReader([]byte("Hello world")), 0)
	require.NoError(t, err)

	data, err := io.ReadAll(pr.GetPublicDataReader())
	require.NoError(t, err)

	pr2, err := FromPublicData(pr.BlobName(), bytes.NewReader(data))
	require.NoError(t, err)

	data2, err := io.ReadAll(pr2.GetPublicDataReader())
	require.NoError(t, err)

	require.Equal(t, data2, data)
}

func TestPublicReaderGetLinkDataReader(t *testing.T) {
	t.Run("Successful encryption and decryption", func(t *testing.T) {
		link, err := Create(rand.Reader)
		require.NoError(t, err)

		pr, key, err := link.UpdateLinkData(bytes.NewReader([]byte("Hello world")), 0)
		require.NoError(t, err)

		rdr2, err := pr.GetLinkDataReader(key)
		require.NoError(t, err)

		readBack, err := io.ReadAll(rdr2)
		require.NoError(t, err)

		require.Equal(t, []byte("Hello world"), readBack)
	})

	t.Run("IV failure", func(t *testing.T) {
		link, err := Create(rand.Reader)
		require.NoError(t, err)

		pr, key, err := link.UpdateLinkData(bytes.NewReader([]byte("Hello world")), 0)
		require.NoError(t, err)

		// Flip a single bit in IV
		ivBytes := pr.iv.Bytes()
		ivBytes[len(ivBytes)/2] ^= 0x80
		pr.iv = common.BlobIVFromBytes(ivBytes)

		// Because the IV is incorrect, key validation block that is encrypted will be invalid
		// thus the method will complain about key, not the IV that will fail first
		_, err = pr.GetLinkDataReader(key)
		require.ErrorIs(t, err, blobtypes.ErrValidationFailed)
	})

	t.Run("nil key", func(t *testing.T) {
		link, err := Create(rand.Reader)
		require.NoError(t, err)

		pr, _, err := link.UpdateLinkData(bytes.NewReader([]byte("Hello world")), 0)
		require.NoError(t, err)

		_, err = pr.GetLinkDataReader(&common.BlobKey{})
		require.ErrorIs(t, err, cipherfactory.ErrInvalidEncryptionConfigKeyType)
	})
}
