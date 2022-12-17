package blenc

import (
	"context"
	"io"

	"github.com/cinode/go/common"
	"github.com/cinode/go/internal/blobtypes/generation"
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

	// Create completely new blob with given dataset, as a result, the blobname and optional
	// WriterInfo that allows blob's update is returned
	Create(ctx context.Context, blobType common.BlobType, r io.Reader) (common.BlobName, KeyInfo, WriterInfo, error)

	// Exists does check whether blob of given name exists. It forwards the call
	// to underlying datastore.
	Exists(ctx context.Context, name common.BlobName) (bool, error)

	// Delete tries to remove blob with given name. It forwards the call to
	// underlying datastore.
	Delete(ctx context.Context, name common.BlobName) error
}
