package blenc

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"io"

	"github.com/Yawning/chacha20"
	base58 "github.com/jbenet/go-base58"
)

// ErrInvalidKey is used as an error when given string representation if the
// key can not be interpreted
var ErrInvalidKey = errors.New("Invalid encryption key")

const (
	keyTypeAES      = 0x01
	keyTypeChaCha20 = 0x02

	keyTypeDefault = keyTypeChaCha20
)

const (
	maxKeyDataSizeUsed = 32
	keyTypeAESKeySize  = 24
)

type streamReader struct {
	cipher.StreamReader
}

func (s *streamReader) Close() error {
	if closer, ok := s.R.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func streamCipherReaderForKey(key string, r io.ReadCloser) (io.ReadCloser, error) {

	keyData := base58.Decode(key)
	if len(keyData) == 0 {
		return nil, ErrInvalidKey
	}

	return streamCipherReaderForKeyData(keyData[0], keyData[1:], r)
}

func keyFromKeyData(keyType byte, keyData []byte) string {

	keyBytes := make([]byte, 1, maxKeyDataSizeUsed+1)
	keyBytes[0] = keyType

	switch keyType {
	case keyTypeAES:
		keyBytes = append(keyBytes, keyData[:keyTypeAESKeySize]...)

	case keyTypeChaCha20:
		keyBytes = append(keyBytes, keyData[:chacha20.KeySize]...)

	}

	return base58.Encode(keyBytes)
}

func streamCipherReaderForKeyData(keyType byte, keyData []byte, r io.ReadCloser) (io.ReadCloser, error) {

	switch keyType {
	case keyTypeAES:
		block, err := aes.NewCipher(keyData[:keyTypeAESKeySize])
		if err != nil {
			return nil, err
		}
		var iv [aes.BlockSize]byte
		stream := cipher.NewCTR(block, iv[:])
		return &streamReader{cipher.StreamReader{S: stream, R: r}}, nil

	case keyTypeChaCha20:
		var nonce [chacha20.NonceSize]byte
		stream, err := chacha20.NewCipher(keyData[:chacha20.KeySize], nonce[:])
		if err != nil {
			return nil, err
		}
		return &streamReader{cipher.StreamReader{S: stream, R: r}}, nil

	default:
		return nil, ErrInvalidKey
	}
}
