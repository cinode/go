package blenc

import "io"

type keyDataGeneratorConstant struct {
	keyData []byte
}

func (k *keyDataGeneratorConstant) GenerateKeyData(stream io.ReadCloser) (
	keyData []byte, origStream io.ReadCloser, err error) {
	return k.keyData, stream, nil
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
