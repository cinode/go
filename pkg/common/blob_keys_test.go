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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlobKey(t *testing.T) {
	keyBytes := []byte{1, 2, 3}
	key := BlobKeyFromBytes(keyBytes)
	require.Equal(t, keyBytes, key.Bytes())
	require.True(t, key.Equal(BlobKeyFromBytes(keyBytes)))
	require.Nil(t, new(BlobKey).Bytes())
}

func TestBlobIV(t *testing.T) {
	ivBytes := []byte{1, 2, 3}
	iv := BlobIVFromBytes(ivBytes)
	require.Equal(t, ivBytes, iv.Bytes())
	require.True(t, iv.Equal(BlobIVFromBytes(ivBytes)))
	require.Nil(t, new(BlobKey).Bytes())
}
