package blenc

import (
	"crypto/sha256"
	"errors"
	"io"
)

var errKeyDataToLarge = errors.New("key data too large")

type keyDataGeneratorContents struct {
}

func (k *keyDataGeneratorContents) GenerateKeyData(stream io.ReadCloser, keyData []byte) (
	equalStream io.ReadCloser, err error) {

	// Ensure the stream is closed once we're out of here
	defer stream.Close()

	if len(keyData) > sha256.Size {
		// This should not happen when called from within this package. We
		// don't use ciphers that need more than 32 bytes of key data.
		return nil, errKeyDataToLarge
	}

	// We need a temporary buffer to save the contents of the stream
	tempBuffer, err := newSecureTempBuffer()
	if err != nil {
		return nil, err
	}
	defer tempBuffer.Close()

	// Store in temp buffer and calculate sha in the meantime
	hasher := sha256.New()
	_, err = io.Copy(tempBuffer, io.TeeReader(stream, hasher))
	if err != nil {
		return nil, err
	}
	hash := hasher.Sum(nil)

	// Use hash to fill the keyData array
	copy(keyData, hash)

	// Ensure we read the same contents by requiring same hash on the data
	return newReaderHashValidator(tempBuffer.Reader(), sha256.New(), hash), nil
}

func (k *keyDataGeneratorContents) IsDeterministic() bool {
	return true
}

// ContentsHashKey returns key data generator instance that's calculating keys
// by doing the hash of contents to be encrypted
func ContentsHashKey() KeyDataGenerator {
	return &keyDataGeneratorContents{}
}
