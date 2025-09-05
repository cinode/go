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

package datastore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFromLocation(t *testing.T) {
	for _, d := range []struct {
		expectedMainType    any
		expectedStorageType any
		location            string
	}{
		{
			location:            "file://" + t.TempDir(),
			expectedMainType:    &datastore{},
			expectedStorageType: &fileSystem{},
		},
		{
			location:            "file-raw://" + t.TempDir(),
			expectedMainType:    &datastore{},
			expectedStorageType: &rawFileSystem{},
		},
		{
			location:            "memory://",
			expectedMainType:    &datastore{},
			expectedStorageType: &memory{},
		},
		{
			location:            "http://some.domain.com",
			expectedMainType:    &webConnector{},
			expectedStorageType: nil,
		},
		{
			location:            "https://some.domain.com",
			expectedMainType:    &webConnector{},
			expectedStorageType: nil,
		},
		{
			location:            t.TempDir(),
			expectedMainType:    &datastore{},
			expectedStorageType: &fileSystem{},
		},
	} {
		t.Run(d.location, func(t *testing.T) {
			ds, err := FromLocation(d.location)
			require.NoError(t, err)
			require.IsType(t, d.expectedMainType, ds)
			if d.expectedStorageType != nil {
				require.IsType(t, d.expectedStorageType, ds.(*datastore).s)
			}
		})
	}

	t.Run("invalid memory location", func(t *testing.T) {
		ds, err := FromLocation("memory://some-invalid-parameter")
		require.ErrorIs(t, err, ErrInvalidMemoryLocation)
		require.Nil(t, ds)
	})
}
