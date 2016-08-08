package datastore

import (
	"crypto/sha256"
	"hash"
	"regexp"

	base58 "github.com/jbenet/go-base58"
)

const hashNameBytePrefix = 0x01

var nameValidator = regexp.MustCompile(`^[a-zA-Z0-9]{16,}$`)

type hasher struct {
	h hash.Hash
}

func (h *hasher) Write(p []byte) (n int, err error) {
	return h.h.Write(p)
}

func (h *hasher) Name() string {
	return base58.Encode(h.h.Sum([]byte{hashNameBytePrefix}))
}

func newHasher() *hasher {
	return &hasher{sha256.New()}
}

func validateBlobName(n string) bool {
	return nameValidator.MatchString(n)
}
