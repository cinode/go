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
	"bytes"
	"crypto/aes"
	"errors"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20"
)

type errorKeyInfo struct {
	err error
}

func (k *errorKeyInfo) GetSymmetricKey() (byte, []byte, []byte, error) {
	return 0, nil, nil, k.err
}

func TestCipherForKi(t *testing.T) {

	errCheck := errors.New("test error")

	for _, d := range []struct {
		desc string
		key  KeyInfo
		err  error
	}{
		{
			"Empty key",
			&keyInfoStatic{},
			ErrInvalidKey,
		},
		{
			"Error when retrieving key info",
			&errorKeyInfo{err: errCheck},
			errCheck,
		},
		{
			"Invalid AES key size",
			&keyInfoStatic{
				t:   keyTypeAES,
				key: bytes.Repeat([]byte{0x0}, 23),
				iv:  bytes.Repeat([]byte{0x0}, 16),
			},
			ErrInvalidKey,
		},
		{
			"Invalid AES iv size",
			&keyInfoStatic{
				t:   keyTypeAES,
				key: bytes.Repeat([]byte{0x0}, 24),
				iv:  bytes.Repeat([]byte{0x0}, 15),
			},
			ErrInvalidKey,
		},
		{
			"Valid AES key",
			&keyInfoStatic{
				t:   keyTypeAES,
				key: bytes.Repeat([]byte{0x0}, 24),
				iv:  bytes.Repeat([]byte{0x0}, 16),
			},
			nil,
		},
		{
			"Invalid chacha20 key size",
			&keyInfoStatic{
				t:   keyTypeChaCha20,
				key: bytes.Repeat([]byte{0x0}, 31),
				iv:  bytes.Repeat([]byte{0x0}, 12),
			},
			ErrInvalidKey,
		},
		{
			"Invalid chacha20 iv size",
			&keyInfoStatic{
				t:   keyTypeChaCha20,
				key: bytes.Repeat([]byte{0x0}, 32),
				iv:  bytes.Repeat([]byte{0x0}, 11),
			},
			ErrInvalidKey,
		},
		{
			"Valid chacha20 key",
			&keyInfoStatic{
				t:   keyTypeChaCha20,
				key: bytes.Repeat([]byte{0x0}, 32),
				iv:  bytes.Repeat([]byte{0x0}, 12),
			},
			nil,
		},
	} {
		t.Run(d.desc, func(t *testing.T) {
			sc, err := streamCipherReaderForKeyInfo(d.key, bytes.NewReader([]byte{}))
			require.ErrorIs(t, err, d.err)
			if err == nil {
				require.NotNil(t, sc)
			}
		})
	}
}

func TestStreamCipherReaderWriterForKeyInfo(t *testing.T) {
	for _, ki := range []KeyInfo{
		&keyInfoStatic{
			t:   keyTypeAES,
			key: make([]byte, keySizeAES),
			iv:  make([]byte, aes.BlockSize),
		},
		&keyInfoStatic{
			t:   keyTypeChaCha20,
			key: make([]byte, chacha20.KeySize),
			iv:  make([]byte, chacha20.NonceSize),
		},
	} {
		t.Run(fmt.Sprintf("valid roundtrip: %+v", ki), func(t *testing.T) {
			data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
			buf := bytes.NewBuffer(nil)

			writer, err := streamCipherWriterForKeyInfo(ki, buf)
			require.NoError(t, err)

			_, err = writer.Write(data)
			require.NoError(t, err)
			require.NotEqual(t, buf.Bytes(), data)

			reader, err := streamCipherReaderForKeyInfo(ki, bytes.NewReader(buf.Bytes()))
			require.NoError(t, err)

			readBack, err := ioutil.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, data, readBack)
		})
	}

	for _, ki := range []KeyInfo{
		&keyInfoStatic{
			t:   keyTypeAES,
			key: make([]byte, keySizeAES-1),
			iv:  make([]byte, aes.BlockSize),
		},
		&keyInfoStatic{
			t:   keyTypeChaCha20,
			key: make([]byte, chacha20.KeySize-1),
			iv:  make([]byte, chacha20.NonceSize),
		},
	} {
		t.Run(fmt.Sprintf("invalid key: %+v", ki), func(t *testing.T) {
			_, err := streamCipherWriterForKeyInfo(ki, bytes.NewBuffer(nil))
			require.ErrorIs(t, err, ErrInvalidKey)

			_, err = streamCipherReaderForKeyInfo(ki, bytes.NewReader(nil))
			require.ErrorIs(t, err, ErrInvalidKey)
		})
	}
}
