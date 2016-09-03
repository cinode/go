package blenc

import (
	"errors"
	"io"
)

var errInsufficientKeyData = errors.New("Insufficient key data")

type keyDataGeneratorConstant struct {
	keyData []byte
}

func (k *keyDataGeneratorConstant) GenerateKeyData(origStream io.ReadCloser, keyData []byte) (
	equalStream io.ReadCloser, err error) {
	if len(keyData) > len(k.keyData) {
		return nil, errInsufficientKeyData
	}
	copy(keyData, k.keyData)
	return origStream, nil
}

func (k *keyDataGeneratorConstant) IsDeterministic() bool {
	return true
}

// KeyConstant returns an implementation of KeyGenerator interface that always
// returns the same key.
//
// Note: Since reusing of the same key could lead to security vulnerabilities,
//       this key generator has been made private and is only used for internal
//       functionality.
func constantKey(keyData []byte) KeyDataGenerator {
	return &keyDataGeneratorConstant{keyData: keyData}
}
