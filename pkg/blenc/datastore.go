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
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/utilities/securefifo"
	"golang.org/x/crypto/chacha20"
)

// FromDatastore creates Blob Encoder using given datastore implementation as
// the storage layer
func FromDatastore(ds datastore.DS) BE {
	return &beDatastore{ds: ds}
}

type beDatastore struct {
	ds datastore.DS
}

func (be *beDatastore) Read(ctx context.Context, name common.BlobName, key EncryptionKey, w io.Writer) error {
	if name.Type() != blobtypes.Static {
		return blobtypes.ErrUnknownBlobType
	}

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

func (be *beDatastore) Create(
	ctx context.Context,
	blobType common.BlobType,
	r io.Reader,
) (
	common.BlobName,
	EncryptionKey,
	WriterInfo,
	error,
) {
	if blobType != blobtypes.Static {
		return nil, nil, nil, blobtypes.ErrUnknownBlobType
	}

	// Static blobtype generation

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
	name, err := common.BlobNameFromHashAndType(blobNameHasher.Sum(nil), blobType)
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

func (be *beDatastore) Exists(ctx context.Context, name common.BlobName) (bool, error) {
	return be.ds.Exists(ctx, name)
}

func (be *beDatastore) Delete(ctx context.Context, name common.BlobName) error {
	return be.ds.Delete(ctx, name)
}
