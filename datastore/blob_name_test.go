package datastore

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlobName(t *testing.T) {
	for _, h := range [][]byte{
		{0}, {1}, {2}, {0xFE}, {0xFF},
		{0, 0, 0, 0},
		{1, 2, 3, 4, 5, 6},
		sha256.New().Sum(nil),
	} {
		for _, bt := range []BlobType{0, 1, 2, 0xFE, 0xFF} {
			t.Run(fmt.Sprintf("%v:%v", bt, h), func(t *testing.T) {
				bn, err := BlobNameFromHashAndType(h, bt)
				assert.NoError(t, err)
				assert.NotEmpty(t, bn)
				assert.Greater(t, len(bn), len(h))
				assert.Equal(t, h, bn.Hash())
				assert.Equal(t, bt, bn.Type())

				s := bn.String()
				bn2, err := BlobNameFromString(s)
				require.NoError(t, err)
				require.Equal(t, bn, bn2)
			})
		}
	}

	_, err := BlobNameFromString("!@#")
	require.ErrorIs(t, err, ErrInvalidBlobName)

	_, err = BlobNameFromString("")
	require.ErrorIs(t, err, ErrInvalidBlobName)

	_, err = BlobNameFromHashAndType(nil, 0x00)
	require.ErrorIs(t, err, ErrInvalidBlobName)
}
