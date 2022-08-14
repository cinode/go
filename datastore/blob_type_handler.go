package datastore

import "io"

type BlobType byte

type BlobTypeHandler interface {
	// Type returns the type of blobs supported by this handler
	Type() BlobType

	Ingest(hash []byte, current, update io.Reader, result io.Writer) error

	Validate(hash []byte, data io.Reader, validated io.Writer) error
}
