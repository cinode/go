package generation

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticBlobHandler(t *testing.T) {
	bt := newStaticBlobHandlerSha256()

	data := []byte("Hello world!")

	hash, wi, err := bt.PrepareNewBlob(bytes.NewReader(data))
	require.NoError(t, err)
	require.Len(t, hash, sha256.Size)
	require.Empty(t, wi)
}
