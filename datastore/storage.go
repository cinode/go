package datastore

import (
	"context"
	"io"

	"github.com/cinode/go/common"
)

type WriteCloseCanceller interface {
	io.WriteCloser
	Cancel()
}

type storage interface {
	kind() string
	openReadStream(ctx context.Context, name common.BlobName) (io.ReadCloser, error)
	openWriteStream(ctx context.Context, name common.BlobName) (WriteCloseCanceller, error)
	exists(ctx context.Context, name common.BlobName) (bool, error)
	delete(ctx context.Context, name common.BlobName) error
}
