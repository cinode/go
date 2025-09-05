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

package cipherfactory

import (
	"bytes"
	"io"
	"testing"

	"github.com/cinode/go/pkg/common"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20"
)

func TestCipherForKeyIV(t *testing.T) {
	for _, d := range []struct {
		err  error
		desc string
		key  []byte
		iv   []byte
	}{
		{
			desc: "Empty key",
			key:  nil,
			iv:   nil,
			err:  ErrInvalidEncryptionConfigKeyType,
		},
		{
			desc: "Invalid Chacha20 key size",
			key:  make([]byte, chacha20.KeySize),
			iv:   make([]byte, chacha20.NonceSizeX),
			err:  ErrInvalidEncryptionConfigKeySize,
		},
		{
			desc: "Invalid Chacha20 nonce size",
			key:  make([]byte, chacha20.KeySize+1),
			iv:   make([]byte, chacha20.NonceSizeX-1),
			err:  ErrInvalidEncryptionConfigIVSize,
		},
		{
			desc: "Valid chacha20 key",
			key:  make([]byte, chacha20.KeySize+1),
			iv:   make([]byte, chacha20.NonceSizeX),
			err:  nil,
		},
	} {
		t.Run(d.desc, func(t *testing.T) {
			sr, err := StreamCipherReader(
				common.BlobKeyFromBytes(d.key),
				common.BlobIVFromBytes(d.iv),
				bytes.NewReader([]byte{}),
			)
			require.ErrorIs(t, err, d.err)
			if err == nil {
				require.NotNil(t, sr)
			}

			sw, err := StreamCipherWriter(
				common.BlobKeyFromBytes(d.key),
				common.BlobIVFromBytes(d.iv),
				bytes.NewBuffer(nil),
			)
			require.ErrorIs(t, err, d.err)
			if err == nil {
				require.NotNil(t, sw)
			}
		})
	}
}

func TestStreamCipherRoundtrip(t *testing.T) {
	key := common.BlobKeyFromBytes(make([]byte, chacha20.KeySize+1))
	iv := common.BlobIVFromBytes(make([]byte, chacha20.NonceSizeX))

	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	buf := bytes.NewBuffer(nil)

	writer, err := StreamCipherWriter(key, iv, buf)
	require.NoError(t, err)

	_, err = writer.Write(data)
	require.NoError(t, err)
	require.NotEqual(t, buf.Bytes(), data)

	reader, err := StreamCipherReader(key, iv, bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)

	readBack, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, data, readBack)
}
