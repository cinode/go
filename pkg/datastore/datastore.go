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

func (ds *datastore) Open(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	switch name.Type() {
	case blobtypes.Static:
		return ds.openStatic(ctx, name)
	case blobtypes.DynamicLink:
		return ds.openDynamicLink(ctx, name)
	default:
		return nil, blobtypes.ErrUnknownBlobType
	}
}

func (ds *datastore) Update(ctx context.Context, name common.BlobName, updateStream io.Reader) error {
	switch name.Type() {
	case blobtypes.Static:
		return ds.updateStatic(ctx, name, updateStream)
	case blobtypes.DynamicLink:
		return ds.updateDynamicLink(ctx, name, updateStream)
	default:
		return blobtypes.ErrUnknownBlobType
	}
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
