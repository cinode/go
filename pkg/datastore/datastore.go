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

package datastore

import (
	"context"
	"io"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
)

type datastore struct {
	s storage
}

var _ DS = (*datastore)(nil)

func (ds *datastore) Kind() string {
	return ds.s.kind()
}

func (ds *datastore) Address() string {
	return ds.s.address()
}

func (ds *datastore) Open(ctx context.Context, name *common.BlobName) (io.ReadCloser, error) {
	switch name.Type() {
	case blobtypes.Static:
		return ds.openStatic(ctx, name)
	case blobtypes.DynamicLink:
		return ds.openDynamicLink(ctx, name)
	default:
		return nil, blobtypes.ErrUnknownBlobType
	}
}

func (ds *datastore) Update(ctx context.Context, name *common.BlobName, updateStream io.Reader) error {
	switch name.Type() {
	case blobtypes.Static:
		return ds.updateStatic(ctx, name, updateStream)
	case blobtypes.DynamicLink:
		return ds.updateDynamicLink(ctx, name, updateStream)
	default:
		return blobtypes.ErrUnknownBlobType
	}
}

func (ds *datastore) Exists(ctx context.Context, name *common.BlobName) (bool, error) {
	return ds.s.exists(ctx, name)
}

func (ds *datastore) Delete(ctx context.Context, name *common.BlobName) error {
	return ds.s.delete(ctx, name)
}

// InMemory constructs an in-memory datastore
//
// The content is lost if the datastore is destroyed (either by garbage collection
// or by program termination)
func InMemory() DS {
	return &datastore{s: newStorageMemory()}
}

// InFileSystem constructs a datastore using filesystem as a storage layer.
//
// Contrary to InRawFileSystem, this datastore is optimized for large datastores
// and concurrent use.
func InFileSystem(path string) (DS, error) {
	s, err := newStorageFilesystem(path)
	if err != nil {
		return nil, err
	}
	return &datastore{s: s}, nil
}

// InRawFilesystem is a simplified storage that uses filesystem as a storage layer.
//
// Datastore files are stored directly under base58-encoded blob names.
// This datastore should not be used for highly concurrent or highly modified
// cases. The main purpose is to dump files to a disk in a form that can
// be lated used in a classic web server and used as a static web source.
func InRawFileSystem(path string) (DS, error) {
	s, err := newStorageRawFilesystem(path)
	if err != nil {
		return nil, err
	}
	return &datastore{s: s}, nil
}
