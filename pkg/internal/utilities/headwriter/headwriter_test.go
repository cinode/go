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

package headwriter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeadWriter(t *testing.T) {
	lw := New(10)

	n, err := lw.Write([]byte{0, 1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, []byte{0, 1, 2, 3}, lw.Head())

	n, err = lw.Write([]byte{})
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.Equal(t, []byte{0, 1, 2, 3}, lw.Head())

	n, err = lw.Write([]byte{4, 5, 6, 7})
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, []byte{0, 1, 2, 3, 4, 5, 6, 7}, lw.Head())

	n, err = lw.Write([]byte{8, 9, 10})
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, lw.Head())

	n, err = lw.Write([]byte{11, 12, 13, 14})
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, lw.Head())
}
