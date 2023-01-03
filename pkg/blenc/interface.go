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
)

type AuthInfo = []byte
type EncryptionKey = []byte

// BE interface describes functionality exposed by Blob Encryption layer
// implementation
type BE interface {

	// Open opens given blob data for reading.
	//
	// If returned error is not nil, the reader must be nil. Otherwise it is required to
	// close the reader once done working with it.
	Open(ctx context.Context, name common.BlobName, key EncryptionKey) (io.ReadCloser, error)

	// Create completely new blob with given dataset, as a result, the blob name and optional
	// AuthInfo that allows blob's update is returned
	Create(ctx context.Context, blobType common.BlobType, r io.Reader) (common.BlobName, EncryptionKey, AuthInfo, error)

	// Update updates given blob type with new data,
	// The update must happen within a single blob name (i.e. it can not end up with blob with different name)
	// and may not be available for certain blob types such as static blobs.
	// A valid writer info is necessary to ensure a correct new content can be created
	Update(ctx context.Context, name common.BlobName, ai AuthInfo, key EncryptionKey, r io.Reader) error

	// Exists does check whether blob of given name exists. It forwards the call
	// to underlying datastore.
	Exists(ctx context.Context, name common.BlobName) (bool, error)

	// Delete tries to remove blob with given name. It forwards the call to
	// underlying datastore.
	Delete(ctx context.Context, name common.BlobName) error
}
