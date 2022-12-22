package crosstest

import (
	"bytes"
	"testing"

	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/generation"
	"github.com/cinode/go/pkg/internal/blobtypes/propagation"
	"github.com/stretchr/testify/require"
)

func TestGenerationAndPropagation(t *testing.T) {
	for n, bt := range blobtypes.All {
		t.Run(n, func(t *testing.T) {
			var data = []byte("Hello world!")
			var updatePack []byte
			var blobHash []byte

			t.Run("generation of the update stream", func(t *testing.T) {

				gHandler, err := generation.HandlerForType(bt)
				require.NoError(t, err)

				hash, wi, err := gHandler.PrepareNewBlob(bytes.NewReader(data))
				require.NoError(t, err)

				updatePackBuff := bytes.NewBuffer([]byte{})

				err = gHandler.SendUpdate(hash, wi, bytes.NewReader(data), updatePackBuff)
				require.NoError(t, err)

				updatePack = updatePackBuff.Bytes()
				blobHash = hash
			})

			t.Run("propagation of the update pack", func(t *testing.T) {

				pHandler, err := propagation.HandlerForType(bt)
				require.NoError(t, err)

				err = pHandler.Validate(blobHash, bytes.NewReader(updatePack))
				require.NoError(t, err)

				finalPack := bytes.NewBuffer([]byte{})

				err = pHandler.Ingest(
					blobHash,
					bytes.NewReader(updatePack),
					bytes.NewReader(updatePack),
					finalPack,
				)
				require.NoError(t, err)
			})

		})
	}
}
