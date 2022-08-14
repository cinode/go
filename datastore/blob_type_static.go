package datastore

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
)

var (
	ErrInvalidStaticBlobHash = fmt.Errorf("%w: invalid static blob hash", ErrValidationFailed)
)

const (
	blobTypeStatic BlobType = 0x00
)

type staticBlobHandler struct {
	blobType  BlobType
	newHasher func() hash.Hash
}

func (h staticBlobHandler) Type() BlobType {
	return h.blobType
}

func (h staticBlobHandler) Ingest(hash []byte, current, update io.Reader, w io.Writer) error {
	// TODO: Optimize - at this point we are not required to ingest the update stream
	//       so if the current stream is valid (hash correct hash), we should cancel the
	//		 ingestion (but don't return an error, instead say that update is not needed)
	return h.Validate(hash, update, w)
}

func (h staticBlobHandler) Validate(hash []byte, data io.Reader, validated io.Writer) error {
	_, err := io.Copy(
		validated,
		NewChecksumReader(
			data,
			h.newHasher(),
			hash,
			ErrInvalidStaticBlobHash,
		),
	)
	return err
}

func NewStaticBlobHandlerSha256() BlobTypeHandler {
	return staticBlobHandler{
		blobType:  blobTypeStatic,
		newHasher: sha256.New,
	}
}
