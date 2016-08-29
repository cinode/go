package blenc

import "io"

type keyGeneratorConstant struct {
	key string
}

func (k *keyGeneratorConstant) GenerateKey(stream io.ReadCloser) (
	key string, origStream io.ReadCloser, err error) {
	return k.key, stream, nil
}

func (k *keyGeneratorConstant) IsDeterministic() bool {
	return true
}

// KeyConstant returns an implementation of KeyGenerator interface that always
// returns the same key.
//
// Note: Since reusing of the same key could lead to security vulnerabilities,
//       this key generator has been made private and is only used for internal
//       functionality.
func constantKey(key string) KeyGenerator {
	return &keyGeneratorConstant{key: key}
}
