package datastore

import (
	"crypto/subtle"
	"errors"
	"hash"
	"io"
)

type checksumReader struct {
	src               io.Reader
	hasher            hash.Hash
	expectedHash      []byte
	hashMismatchError error
}

func (cr *checksumReader) Read(b []byte) (int, error) {
	n, err := cr.src.Read(b)
	cr.hasher.Write(b[:n])

	if errors.Is(err, io.EOF) {
		calculatedHash := cr.hasher.Sum(nil)
		if subtle.ConstantTimeCompare(cr.expectedHash, calculatedHash) != 1 {
			return n, cr.hashMismatchError
		}
	}

	return n, err
}

func NewChecksumReader(src io.Reader, hasher hash.Hash, expectedHash []byte, hashMismatchError error) io.Reader {
	return &checksumReader{
		src:               src,
		hasher:            hasher,
		expectedHash:      expectedHash,
		hashMismatchError: hashMismatchError,
	}
}
