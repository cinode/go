package blenc

import "io"

type keyGeneratorConstant struct {
	key string
}

func (k *keyGeneratorConstant) GenerateKey(stream io.ReadCloser) (
	key string, origStream io.ReadCloser, err error) {
	return k.key, stream, nil
}

// KeyConstant returns an implementation of KeyGenerator interface that always
// returns the same key.
//
// Note: Do never ever use this in production code. Reusing keys could lead to
//       serious cryptographical vulnerabilities.
func KeyConstant(key string) KeyGenerator {
	return &keyGeneratorConstant{key: key}
}
