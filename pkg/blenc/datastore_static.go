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

package blenc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"github.com/cinode/go/pkg/internal/utilities/validatingreader"
)

var (
	ErrCanNotUpdateStaticBlob = errors.New("blob update is not supported for static blobs")
)

func (be *beDatastore) openStatic(ctx context.Context, name common.BlobName, key cipherfactory.Key) (io.ReadCloser, error) {

	rc, err := be.ds.Open(ctx, name)
	if err != nil {
		return nil, err
	}

	scr, err := cipherfactory.StreamCipherReader(key, key.DefaultIV(), rc)
	if err != nil {
		return nil, err
	}

	keyGenerator := cipherfactory.NewKeyGenerator(blobtypes.Static)

	return &struct {
		io.Reader
		io.Closer
	}{
		Reader: validatingreader.CheckOnEOF(
			io.TeeReader(scr, keyGenerator),
			func() error {
				if !bytes.Equal(key, keyGenerator.Generate()) {
					return blobtypes.ErrValidationFailed
				}
				return nil
			},
		),
		Closer: rc,
	}, nil
}

func (be *beDatastore) createStatic(
	ctx context.Context,
	r io.Reader,
) (
	common.BlobName,
	cipherfactory.Key,
	AuthInfo,
	error,
) {
	tempWriteBufferPlain, err := be.newSecureFifo()
	if err != nil {
		return nil, nil, nil, err
	}
	defer tempWriteBufferPlain.Close()

	tempWriteBufferEncrypted, err := be.newSecureFifo()
	if err != nil {
		return nil, nil, nil, err
	}
	defer tempWriteBufferEncrypted.Close()

	keyGenerator := cipherfactory.NewKeyGenerator(blobtypes.Static)
	_, err = io.Copy(tempWriteBufferPlain, io.TeeReader(r, keyGenerator))
	if err != nil {
		return nil, nil, nil, err
	}

	key := keyGenerator.Generate()
	iv := key.DefaultIV() // We can use this since each blob will have different key

	rClone, err := tempWriteBufferPlain.Done() // rClone will allow re-reading the source data
	if err != nil {
		return nil, nil, nil, err
	}
	defer rClone.Close()

	// Encrypt data with calculated key, hash encrypted data to generate blob name
	blobNameHasher := sha256.New()
	encWriter, err := cipherfactory.StreamCipherWriter(
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
	authInfo AuthInfo,
	key cipherfactory.Key,
	r io.Reader,
) error {
	return ErrCanNotUpdateStaticBlob
}
