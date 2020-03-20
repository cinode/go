package blenc

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"io"

	"github.com/jbenet/go-base58"
	"golang.org/x/crypto/chacha20"
)

// ErrInvalidKey is used as an error when given string representation if the
// key can not be interpreted
var ErrInvalidKey = errors.New("Invalid encryption key")

const (
	keyTypeAES      = 0x01
	keyTypeChaCha20 = 0x02
	keyTypeInvalid  = 0xFF

	keyTypeDefault = keyTypeChaCha20
)

const (
	keyTypeDefaultLen = chacha20.KeySize
	keyTypeAESKeySize = 24
)

type streamReader struct {
	cipher.StreamReader
}

func (s *streamReader) Close() error {
	closer := s.R.(io.Closer)
	return closer.Close()
}

func streamCipherReaderForKey(key string, r io.ReadCloser) (io.ReadCloser, error) {

	keyData := base58.Decode(key)
	if len(keyData) == 0 {
		return nil, ErrInvalidKey
	}

	return streamCipherReaderForKeyData(keyData[0], keyData[1:], r)
}

func streamCipherReaderForKeyGenerator(kg KeyDataGenerator, r io.ReadCloser) (stream io.ReadCloser, key string, err error) {

	keyData := make([]byte, 1+keyTypeDefaultLen)
	keyData[0] = keyTypeDefault
	r2, err := kg.GenerateKeyData(r, keyData[1:])
	if err != nil {
		r.Close()
		return nil, "", err
	}

	// Ignore error below, we control arguments and pass valid sizes
	stream, _ = streamCipherReaderForKeyData(keyData[0], keyData[1:], r2)

	key = base58.Encode(keyData)
	return stream, key, nil
}

func streamCipherReaderForKeyData(keyType byte, keyData []byte, r io.ReadCloser) (io.ReadCloser, error) {

	switch keyType {
	case keyTypeAES:
		if len(keyData) != keyTypeAESKeySize {
			return nil, ErrInvalidKey
		}
		// Ignore error below, aes will fail only if the size of key is
		// invalid. We fully control it and pass valid value.
		block, _ := aes.NewCipher(keyData)
		var iv [aes.BlockSize]byte
		stream := cipher.NewCTR(block, iv[:])
		return &streamReader{cipher.StreamReader{S: stream, R: r}}, nil

	case keyTypeChaCha20:
		if len(keyData) != chacha20.KeySize {
			return nil, ErrInvalidKey
		}
		var nonce [chacha20.NonceSize]byte
		// Ignore error below, chacha20 will fail only if the size of key or
		// nonce are invalid. We fully control those and pass valid values.
		stream, _ := chacha20.NewUnauthenticatedCipher(keyData, nonce[:])
		return &streamReader{cipher.StreamReader{S: stream, R: r}}, nil

	default:
		return nil, ErrInvalidKey
	}
}
