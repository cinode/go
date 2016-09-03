package blenc

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"io"

	base58 "github.com/jbenet/go-base58"
	"github.com/yawning/chacha20"
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
	maxKeyDataSizeUsed = 32
	keyTypeAESKeySize  = 24
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

	return streamCipherReaderForKeyData(keyData[0], keyData[1:], r, true)
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

func testKeyData(keyData []byte, minKeySize int, strict bool) error {
	if strict {
		if len(keyData) != minKeySize {
			return ErrInvalidKey
		}
	} else {
		if len(keyData) < minKeySize {
			return ErrInvalidKey
		}
	}
	return nil
}

func streamCipherReaderForKeyData(keyType byte, keyData []byte, r io.ReadCloser, strict bool) (io.ReadCloser, error) {

	switch keyType {
	case keyTypeAES:
		if err := testKeyData(keyData, keyTypeAESKeySize, strict); err != nil {
			return nil, err
		}
		// Ignore error below, aes will fail only if the size of key is
		// invalid. We fully control it and pass valid value.
		block, _ := aes.NewCipher(keyData[:keyTypeAESKeySize])
		var iv [aes.BlockSize]byte
		stream := cipher.NewCTR(block, iv[:])
		return &streamReader{cipher.StreamReader{S: stream, R: r}}, nil

	case keyTypeChaCha20:
		if err := testKeyData(keyData, chacha20.KeySize, strict); err != nil {
			return nil, err
		}
		var nonce [chacha20.NonceSize]byte
		// Ignore error below, chacha20 will fail only if the size of key or
		// nonce are invalid. We fully control those and pass valid values.
		stream, _ := chacha20.NewCipher(keyData[:chacha20.KeySize], nonce[:])
		return &streamReader{cipher.StreamReader{S: stream, R: r}}, nil

	default:
		return nil, ErrInvalidKey
	}
}
