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
