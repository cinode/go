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
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes"
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
	return be.read(ctx, name, key, w, 0)
}

func (be *beDatastore) read(ctx context.Context, name common.BlobName, key EncryptionKey, w io.Writer, recursionDepth int) error {
	switch name.Type() {
	case blobtypes.Static:
		return be.readStatic(ctx, name, key, w)
	case blobtypes.DynamicLink:
		return be.readDynamicLink(ctx, name, key, w, recursionDepth)
	}
	return blobtypes.ErrUnknownBlobType
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
	switch blobType {
	case blobtypes.Static:
		return be.createStatic(ctx, r)
	}
	return nil, nil, nil, blobtypes.ErrUnknownBlobType

}

func (be *beDatastore) Exists(ctx context.Context, name common.BlobName) (bool, error) {
	return be.ds.Exists(ctx, name)
}

func (be *beDatastore) Delete(ctx context.Context, name common.BlobName) error {
	return be.ds.Delete(ctx, name)
}
