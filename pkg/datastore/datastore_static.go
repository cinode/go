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
	"github.com/cinode/go/pkg/internal/utilities/validatingreader"
)

func (ds *datastore) openStatic(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	rc, err := ds.s.openReadStream(ctx, name)
	if err != nil {
		return nil, err
	}

	return validatingreader.NewHashValidation(rc, sha256.New(), name.Hash(), blobtypes.ErrValidationFailed), nil
}

func (ds *datastore) updateStatic(ctx context.Context, name common.BlobName, updateStream io.Reader) error {
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
