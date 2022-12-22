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
	"github.com/cinode/go/pkg/internal/blobtypes/generation"
)

type KeyInfo interface {
	GetSymmetricKey() (byte, []byte, []byte, error)
}

type WriterInfo = generation.WriterInfo

// BE interface describes functionality exposed by Blob Encryption layer
// implementation
type BE interface {

	// Open reads a data stream from a blob with given name and writes the stream
	// to given writer
	Read(ctx context.Context, name common.BlobName, ki KeyInfo, w io.Writer) error

	// Create completely new blob with given dataset, as a result, the blob name and optional
	// WriterInfo that allows blob's update is returned
	Create(ctx context.Context, blobType common.BlobType, r io.Reader) (common.BlobName, KeyInfo, WriterInfo, error)

	// Exists does check whether blob of given name exists. It forwards the call
	// to underlying datastore.
	Exists(ctx context.Context, name common.BlobName) (bool, error)

	// Delete tries to remove blob with given name. It forwards the call to
	// underlying datastore.
	Delete(ctx context.Context, name common.BlobName) error
}
