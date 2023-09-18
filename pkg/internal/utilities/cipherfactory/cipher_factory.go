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

package cipherfactory

import (
	"crypto/cipher"
	"errors"
	"fmt"
	"io"

	"github.com/cinode/go/pkg/common"
	"golang.org/x/crypto/chacha20"
)

var (
	// ErrEncryptionConfig is used as an error when given string representation if the
	// key can not be interpreted
	ErrInvalidEncryptionConfig = errors.New("invalid encryption config")

	ErrInvalidEncryptionConfigKeyType = fmt.Errorf("%w: wrong key type", ErrInvalidEncryptionConfig)
	ErrInvalidEncryptionConfigKeySize = fmt.Errorf("%w: wrong XChaCha20 key size, expected %d bytes", ErrInvalidEncryptionConfig, chacha20.KeySize+1)
	ErrInvalidEncryptionConfigIVSize  = fmt.Errorf("%w: wrong XChaCha20 iv size, expected %d bytes", ErrInvalidEncryptionConfig, chacha20.NonceSizeX)
)

const (
	reservedByteForKeyType byte = 0
)

func StreamCipherReader(key common.BlobKey, iv common.BlobIV, r io.Reader) (io.Reader, error) {
	stream, err := _cipherForKeyIV(key, iv)
	if err != nil {
		return nil, err
	}
	return &cipher.StreamReader{S: stream, R: r}, nil
}

func StreamCipherWriter(key common.BlobKey, iv common.BlobIV, w io.Writer) (io.Writer, error) {
	stream, err := _cipherForKeyIV(key, iv)
	if err != nil {
		return nil, err
	}
	return cipher.StreamWriter{S: stream, W: w}, nil
}

func _cipherForKeyIV(key common.BlobKey, iv common.BlobIV) (cipher.Stream, error) {
	if len(key) == 0 || key[0] != reservedByteForKeyType {
		return nil, ErrInvalidEncryptionConfigKeyType
	}

	if len(key) != chacha20.KeySize+1 {
		return nil, fmt.Errorf("%w, got %d bytes", ErrInvalidEncryptionConfigKeySize, len(key)+1)
	}

	if len(iv) != chacha20.NonceSizeX {
		return nil, fmt.Errorf("%w, got %d bytes", ErrInvalidEncryptionConfigIVSize, len(iv))
	}

	return chacha20.NewUnauthenticatedCipher(key[1:], iv)
}
