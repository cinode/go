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

package datastore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"testing"
	"time"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/stretchr/testify/require"
)

func TestMultiSourceDatastore(t *testing.T) {

	addBlob := func(ds DS, c string) *common.BlobName {
		hash := sha256.Sum256([]byte(c))
		name, err := common.BlobNameFromHashAndType(hash[:], blobtypes.Static)
		require.NoError(t, err)
		err = ds.Update(context.Background(), name, bytes.NewReader([]byte(c)))
		require.NoError(t, err)
		return name
	}

	fetchBlob := func(ds DS, n *common.BlobName) string {
		rc, err := ds.Open(context.Background(), n)
		require.NoError(t, err)

		data, err := io.ReadAll(rc)
		require.NoError(t, err)

		err = rc.Close()
		require.NoError(t, err)

		return string(data)
	}

	ensureNotFound := func(ds DS, n *common.BlobName) {
		_, err := ds.Open(context.Background(), n)
		require.ErrorIs(t, err, ErrNotFound)
	}

	t.Run("Test static link propagation", func(t *testing.T) {
		main := InMemory()
		add1 := InMemory()
		add2 := InMemory()

		ds := NewMultiSource(main, time.Hour, add1, add2).(*multiSourceDatastore)

		bn1 := addBlob(add1, "Hello world 1")
		bn2 := addBlob(add2, "Hello world 2")

		require.EqualValues(t, "Hello world 1", fetchBlob(ds, bn1))
		require.EqualValues(t, "Hello world 2", fetchBlob(ds, bn2))

		// Blobs should already be in the cache
		ds.additional = []DS{}

		require.EqualValues(t, "Hello world 1", fetchBlob(ds, bn1))
		require.EqualValues(t, "Hello world 2", fetchBlob(ds, bn2))
	})

	t.Run("Test link refresh", func(t *testing.T) {
		main := InMemory()
		add := InMemory()

		bn := addBlob(InMemory(), "Hello world")

		ds := NewMultiSource(main, time.Millisecond*10, add).(*multiSourceDatastore)
		ensureNotFound(ds, bn)

		addBlob(add, "Hello world")
		for i := 0; i < 10; i++ {
			// Result still cached
			ensureNotFound(ds, bn)
		}

		time.Sleep(time.Millisecond * 20)

		// Should refresh by now
		require.EqualValues(t, "Hello world", fetchBlob(ds, bn))
	})
}
