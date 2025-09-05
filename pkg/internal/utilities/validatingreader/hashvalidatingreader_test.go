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

package validatingreader_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"testing"

	"github.com/cinode/go/pkg/internal/utilities/validatingreader"
	"github.com/stretchr/testify/require"
)

func TestHashValidatingReader(t *testing.T) {
	for _, data := range [][]byte{
		{},
		{0x00},
		[]byte("Hello world"),
		bytes.Repeat([]byte("Hello,"), 100),
	} {
		t.Run("valid hash", func(t *testing.T) {
			hash := sha256.Sum256(data)

			r := validatingreader.NewHashValidation(
				bytes.NewReader(data),
				sha256.New(),
				hash[:],
				errors.New("Test error"),
			)

			readBack, err := io.ReadAll(r)
			require.NoError(t, err)
			require.EqualValues(t, data, readBack)
		})

		t.Run("invalid hash", func(t *testing.T) {
			hash := sha256.Sum256(data)
			hash[0] ^= 0xFF

			retErr := errors.New("Test error")

			r := validatingreader.NewHashValidation(
				bytes.NewReader(data),
				sha256.New(),
				hash[:],
				retErr,
			)

			readBack, err := io.ReadAll(r)
			require.ErrorIs(t, err, retErr)
			require.EqualValues(t, data, readBack)
		})
	}
}
