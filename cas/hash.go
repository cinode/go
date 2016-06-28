package cas

import (
	"crypto/sha256"
	"hash"

	base58 "github.com/jbenet/go-base58"
)

const hashNamePrefix = "s"

type hasher struct {
	h hash.Hash
}

func (h *hasher) Write(p []byte) (n int, err error) {
	return h.h.Write(p)
}

func (h *hasher) Name() string {
	return hashNamePrefix + base58.Encode(h.h.Sum(nil))
}

func newHasher() *hasher {
	return &hasher{sha256.New()}
}
