package propagation

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"

	"github.com/cinode/go/internal/blobtypes"
)

var (
	ErrInvalidStaticBlobHash = fmt.Errorf("%w: invalid static blob hash", blobtypes.ErrValidationFailed)
)

type staticBlobHandler struct {
	newHasher func() hash.Hash
}

func (h staticBlobHandler) Ingest(hash []byte, current, update io.Reader, w io.Writer) error {
	// TODO: Optimize - at this point we are not required to ingest the update stream
	//       so if the current stream is valid (hash correct hash), we should cancel the
	//		 ingestion (but don't return an error, instead say that update is not needed)
	return h.Validate(hash, io.TeeReader(update, w))
}

func (h staticBlobHandler) Validate(hash []byte, data io.Reader) error {
	hasher := h.newHasher()

	if hasher.Size() != len(hash) {
		return ErrInvalidStaticBlobHash
	}

	_, err := io.Copy(hasher, data)
	if err != nil {
		return err
	}

	calculatedHash := hasher.Sum(nil)
	if !bytes.Equal(hash, calculatedHash) {
		return ErrInvalidStaticBlobHash
	}

	return nil
}

func newStaticBlobHandlerSha256() Handler {
	return staticBlobHandler{
		newHasher: sha256.New,
	}
}
