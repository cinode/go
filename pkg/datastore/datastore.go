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

package datastore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
)

type datastore struct {
	s storage
}

var _ DS = (*datastore)(nil)

func (ds *datastore) Kind() string {
	return ds.s.kind()
}

func (ds *datastore) Read(ctx context.Context, name common.BlobName, output io.Writer) error {
	if name.Type() != blobtypes.Static {
		return blobtypes.ErrUnknownBlobType
	}

	rc, err := ds.s.openReadStream(ctx, name)
	if err != nil {
		return err
	}
	defer rc.Close()

	hasher := sha256.New()
	_, err = io.Copy(output, io.TeeReader(rc, hasher))
	if err != nil {
		return err
	}

	if !bytes.Equal(name.Hash(), hasher.Sum(nil)) {
		return blobtypes.ErrValidationFailed
	}

	return nil
}

func (ds *datastore) Update(ctx context.Context, name common.BlobName, updateStream io.Reader) error {
	if name.Type() != blobtypes.Static {
		return blobtypes.ErrUnknownBlobType
	}

	outputStream, err := ds.s.openWriteStream(ctx, name)
	if err != nil {
		return err
	}
	defer outputStream.Cancel()

	hasher := sha256.New()
	_, err = io.Copy(outputStream, io.TeeReader(updateStream, hasher))
	if err != nil {
		return err
	}

	if !bytes.Equal(name.Hash(), hasher.Sum(nil)) {
		return blobtypes.ErrValidationFailed
	}

	err = outputStream.Close()
	if err != nil {
		return err
	}

	outputStream = nil
	return nil
}

func (ds *datastore) Exists(ctx context.Context, name common.BlobName) (bool, error) {
	return ds.s.exists(ctx, name)
}

func (ds *datastore) Delete(ctx context.Context, name common.BlobName) error {
	return ds.s.delete(ctx, name)
}

func InMemory() DS {
	return &datastore{s: newStorageMemory()}
}

func InFileSystem(path string) DS {
	return &datastore{s: newStorageFilesystem(path)}
}
