package generation

import (
	"crypto/sha256"
	"hash"
	"io"
)

type staticBlobHandler struct {
	newHasher func() hash.Hash
}

// PrepareNewBlob generates the blob name from blob's content hash.
// There's no WriterInfo generated since anybody can create any static blob.
func (h staticBlobHandler) PrepareNewBlob(data io.Reader) ([]byte, WriterInfo, error) {
	hasher := h.newHasher()

	_, err := io.Copy(hasher, data)
	if err != nil {
		return nil, nil, err
	}

	return hasher.Sum(nil), nil, nil
}

func (h staticBlobHandler) SendUpdate(hash []byte, wi WriterInfo, data io.Reader, out io.Writer) error {
	// TODO: Shouldn't we validate the data?
	_, err := io.Copy(out, data)
	return err
}

func newStaticBlobHandlerSha256() Handler {
	return staticBlobHandler{
		newHasher: sha256.New,
	}
}
