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

package cipherfactory

import (
	"testing"

	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/stretchr/testify/require"
)

func TestGenerator(t *testing.T) {
	t.Run("successful generation", func(t *testing.T) {
		buff := []byte{1, 2, 3, 4, 5}

		kg := NewKeyGenerator(blobtypes.Static)

		n, err := kg.Write(buff)
		require.NoError(t, err)
		require.Equal(t, 5, n)

		key := kg.Generate()

		ig := NewIVGenerator(blobtypes.Static)

		n, err = ig.Write(buff)
		require.NoError(t, err)
		require.Equal(t, 5, n)

		iv := ig.Generate()

		_, err = _cipherForKeyIV(key, iv)
		require.NoError(t, err)

		_, err = _cipherForKeyIV(key, key.DefaultIV())
		require.NoError(t, err)

		// Check initial bytes of keys only - since key and IV are of different
		// length, those won't be equal since the size won't match. Instead we
		// check the first 8 bytes (both key and iv are larger) - if those match
		// then the generation of key and iv for the same input dataset would
		// be using same hashed dataset which may be exploitable since IV
		// is made public
		require.NotEqual(t, key[1:1+8], iv[:8])
		require.NotEqual(t, key[1:1+8], key.DefaultIV()[:8])
		require.NotEqual(t, iv[:8], key.DefaultIV()[:8])

		// Note: once other key types are introduced, we should also check
		// that for different key types there are different hashes
	})
}
