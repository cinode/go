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
