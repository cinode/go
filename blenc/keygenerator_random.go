package blenc

import (
	"crypto/rand"
	"io"
)

type keyDataGeneratorRandom struct {
}

func (k *keyDataGeneratorRandom) GenerateKeyData(stream io.ReadCloser, keyData []byte) (
	origStream io.ReadCloser, err error) {
	_, err = rand.Read(keyData)
	return stream, err
}

func (k *keyDataGeneratorRandom) IsDeterministic() bool {
	return false
}

// RandomKey returns an implementation of KeyGenerator interface that always
// returns fully random key. Generated key is built using cryptographically
// strong random data.
func RandomKey() KeyDataGenerator {
	return &keyDataGeneratorRandom{}
}
