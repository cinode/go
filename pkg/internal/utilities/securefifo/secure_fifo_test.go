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

package securefifo

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecureFifoReadBack(t *testing.T) {
	for i, d := range []struct {
		data []byte
	}{
		{data: []byte{}},
		{data: []byte("a")},
		{data: []byte(strings.Repeat("a", 15))},
		{data: []byte(strings.Repeat("a", 16))},
		{data: []byte(strings.Repeat("a", 17))},
		{data: []byte(strings.Repeat("a", 16*1024))},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			w, err := New()
			require.NoError(t, err)
			defer w.Close()

			n, err := w.Write(d.data)
			require.NoError(t, err)
			require.EqualValues(t, len(d.data), n)

			r, err := w.Done()
			require.NoError(t, err)
			defer r.Close()

			err = w.Close()
			require.NoError(t, err)

			// Close must be idempotent
			err = w.Close()
			require.NoError(t, err)

			readBack, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, readBack, d.data)

			r2, err := r.Reset()
			require.NoError(t, err)

			err = r.Close()
			require.NoError(t, err)

			// Close must be idempotent
			err = r.Close()
			require.NoError(t, err)

			readBack, err = io.ReadAll(r2)
			require.NoError(t, err)
			require.Equal(t, readBack, d.data)

			err = r2.Close()
			require.NoError(t, err)
		})
	}
}

func TestSecureFifoFileAccess(t *testing.T) {
	w, err := New()
	require.NoError(t, err)
	defer w.Close()

	data := []byte("secret data")

	n, err := w.Write(data)
	require.NoError(t, err)
	require.EqualValues(t, len(data), n)

	// The file must not physically exist - it is unlinked so only the open handles
	// keep it on disk
	fl := w.(*writer).sf.fl
	require.NoFileExists(t, fl.Name())

	_, err = fl.Seek(0, io.SeekStart)
	require.NoError(t, err)

	dataRead, err := io.ReadAll(fl)
	require.NoError(t, err)
	require.NotContains(t, dataRead, []byte("secret"))

	err = w.Close()
	require.NoError(t, err)

	// File must be closed
	err = fl.Close()
	require.ErrorIs(t, err, os.ErrClosed)
}
