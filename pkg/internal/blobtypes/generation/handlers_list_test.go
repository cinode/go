package generation

import (
	"testing"

	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/stretchr/testify/require"
)

func TestBlobHandlerForValidType(t *testing.T) {
	h, err := HandlerForType(blobtypes.Static)
	require.NoError(t, err)
	require.NotNil(t, h)
}

func TestBlobHandlerForInvalidType(t *testing.T) {
	h, err := HandlerForType(blobtypes.Invalid)
	require.ErrorIs(t, err, blobtypes.ErrUnknownBlobType)
	require.Nil(t, h)
}
