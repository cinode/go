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
	"context"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/utilities/securefifo"
	"golang.org/x/crypto/chacha20"
)

func (be *beDatastore) readStatic(ctx context.Context, name common.BlobName, key EncryptionKey, w io.Writer) error {

	// TODO: Validate the key - to avoid forcing weak keys

	iv := make([]byte, chacha20.NonceSizeX)

	// TODO: This should be reversed - we should use streamCipherReader here since we're reading encrypted data,
	// currently it works because stream ciphers we're using are xor-based thus reader and writer is performing the same logic,
	// it will break if we start using stream cipher where there's asymmetry between encryption and decryption algorithm

	cw, err := streamCipherWriter(key, iv, w)
	if err != nil {
		return err
	}

	err = be.ds.Read(ctx, name, cw)
	if err != nil {
		return err
	}
	return nil
}

func (be *beDatastore) createStatic(
	ctx context.Context,
	r io.Reader,
) (
	common.BlobName,
	EncryptionKey,
	WriterInfo,
	error,
) {
	tempWriteBufferPlain, err := securefifo.New()
	if err != nil {
		return nil, nil, nil, err
	}
	defer tempWriteBufferPlain.Close()

	tempWriteBufferEncrypted, err := securefifo.New()
	if err != nil {
		return nil, nil, nil, err
	}
	defer tempWriteBufferEncrypted.Close()

	// Generate encryption key
	// 		Key - sha256(content)
	//		IV - constant (zeroed iv)

	keyHasher := sha256.New()
	_, err = io.Copy(tempWriteBufferPlain, io.TeeReader(r, keyHasher))
	if err != nil {
		return nil, nil, nil, err
	}

	key := keyHasher.Sum(nil)[:chacha20.KeySize]
	iv := make([]byte, chacha20.NonceSizeX)

	rClone, err := tempWriteBufferPlain.Done() // rClone will allow re-reading the source data
	if err != nil {
		return nil, nil, nil, err
	}
	defer rClone.Close()

	// Encrypt data with calculated key, hash encrypted data to generate blob name
	blobNameHasher := sha256.New()
	encWriter, err := streamCipherWriter(
		key, iv,
		io.MultiWriter(
			tempWriteBufferEncrypted, // Stream out encrypted data to temporary fifo
			blobNameHasher,           // Also hash the output to avoid re-reading the fifo again to build blob name
		),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	_, err = io.Copy(encWriter, rClone)
	if err != nil {
		return nil, nil, nil, err
	}

	encReader, err := tempWriteBufferEncrypted.Done()
	if err != nil {
		return nil, nil, nil, err
	}
	defer encReader.Close()

	// Generate blob name from the encrypted data
	name, err := common.BlobNameFromHashAndType(blobNameHasher.Sum(nil), blobtypes.Static)
	if err != nil {
		return nil, nil, nil, err
	}

	// Send encrypted blob into the datastore
	err = be.ds.Update(ctx, name, encReader)
	if err != nil {
		return nil, nil, nil, err
	}

	return name, key, nil, nil
}

func (be *beDatastore) updateStatic(
	ctx context.Context,
	name common.BlobName,
	wi WriterInfo,
	key EncryptionKey,
	r io.Reader,
) error {
	return errors.New("Blob update is not supported for static blobs")
}
