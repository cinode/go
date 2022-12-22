package blenc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyInfoStatic(t *testing.T) {
	ski := NewStaticKeyInfo(
		0x01,
		[]byte{0x02, 0x03},
		[]byte{0x04, 0x05, 0x06},
	)

	tp, key, iv, err := ski.GetSymmetricKey()
	require.NoError(t, err)
	require.EqualValues(t, 0x01, tp)
	require.Equal(t, []byte{0x02, 0x03}, key)
	require.Equal(t, []byte{0x04, 0x05, 0x06}, iv)
}
