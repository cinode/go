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
	"context"
	"crypto/rand"
	"io"
	"time"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/utilities/securefifo"
)

// FromDatastore creates Blob Encoder using given datastore implementation as
// the storage layer
func FromDatastore(ds datastore.DS) BE {
	return &beDatastore{
		ds:              ds,
		rand:            rand.Reader,
		generateVersion: func() uint64 { return uint64(time.Now().UnixMicro()) },
		newSecureFifo:   securefifo.New,
	}
}

type versionSource func() uint64

type secureFifoGenerator func() (securefifo.Writer, error)

type beDatastore struct {
	ds              datastore.DS
	rand            io.Reader
	generateVersion versionSource
	newSecureFifo   secureFifoGenerator
}

func (be *beDatastore) Open(ctx context.Context, name common.BlobName, key common.BlobKey) (io.ReadCloser, error) {
	switch name.Type() {
	case blobtypes.Static:
		return be.openStatic(ctx, name, key)
	case blobtypes.DynamicLink:
		return be.openDynamicLink(ctx, name, key)
	}
	return nil, blobtypes.ErrUnknownBlobType
}

func (be *beDatastore) Create(
	ctx context.Context,
	blobType common.BlobType,
	r io.Reader,
) (
	common.BlobName,
	common.BlobKey,
	AuthInfo,
	error,
) {
	switch blobType {
	case blobtypes.Static:
		return be.createStatic(ctx, r)
	case blobtypes.DynamicLink:
		return be.createDynamicLink(ctx, r)
	}
	return common.BlobName{}, common.BlobKey{}, nil, blobtypes.ErrUnknownBlobType
}

func (be *beDatastore) Update(ctx context.Context, name common.BlobName, authInfo AuthInfo, key common.BlobKey, r io.Reader) error {
	switch name.Type() {
	case blobtypes.Static:
		return be.updateStatic(ctx, name, authInfo, key, r)
	case blobtypes.DynamicLink:
		return be.updateDynamicLink(ctx, name, authInfo, key, r)
	}
	return blobtypes.ErrUnknownBlobType
}

func (be *beDatastore) Exists(ctx context.Context, name common.BlobName) (bool, error) {
	return be.ds.Exists(ctx, name)
}

func (be *beDatastore) Delete(ctx context.Context, name common.BlobName) error {
	return be.ds.Delete(ctx, name)
}
