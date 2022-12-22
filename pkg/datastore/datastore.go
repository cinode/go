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
	"context"
	"errors"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes/propagation"
)

type datastore struct {
	s storage
}

var _ DS = (*datastore)(nil)

func (ds *datastore) Kind() string {
	return ds.s.kind()
}

func (ds *datastore) Read(ctx context.Context, name common.BlobName, output io.Writer) error {
	handler, err := propagation.HandlerForType(name.Type())
	if err != nil {
		return err
	}

	rc, err := ds.s.openReadStream(ctx, name)
	if err != nil {
		return err
	}
	defer rc.Close()

	return handler.Validate(
		name.Hash(),
		io.TeeReader(rc, output),
	)
}

func (ds *datastore) Update(ctx context.Context, name common.BlobName, updateStream io.Reader) error {
	handler, err := propagation.HandlerForType(name.Type())
	if err != nil {
		return err
	}

	outputStream, err := ds.s.openWriteStream(ctx, name)
	if err != nil {
		return err
	}
	defer func() {
		// Cancel the write operation if for whatever reason
		// we don't finish the write operation
		if outputStream != nil {
			outputStream.Cancel()
		}
	}()

	// Check if we can get the currentStream blob data
	currentStream, err := ds.s.openReadStream(ctx, name)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	defer func() {
		if currentStream != nil {
			currentStream.Close()
		}
	}()

	if currentStream == nil {
		// No content yet, only need to validate the updateStream
		// and store in the result file
		err := handler.Validate(name.Hash(), io.TeeReader(updateStream, outputStream))
		if err != nil {
			return err
		}
	} else {
		err = handler.Ingest(name.Hash(), currentStream, updateStream, outputStream)
		if err != nil {
			return err
		}

		// Current dataset must be closed before closing the output stream
		currentStream.Close()
		currentStream = nil
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
