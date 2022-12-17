package blenc

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20"
)

// ErrInvalidKey is used as an error when given string representation if the
// key can not be interpreted
var ErrInvalidKey = errors.New("invalid encryption key")

const (
	keyTypeAES      byte = 0x01
	keyTypeChaCha20 byte = 0x02
	keyTypeInvalid  byte = 0xFF

	keyTypeDefault = keyTypeChaCha20
)

const (
	keySizeAES = 24
)

func streamCipherReaderForKeyInfo(ki KeyInfo, r io.Reader) (io.Reader, error) {
	stream, err := _cipherForKi(ki)
	if err != nil {
		return nil, err
	}
	return &cipher.StreamReader{S: stream, R: r}, nil
}

func streamCipherWriterForKeyInfo(ki KeyInfo, w io.Writer) (io.Writer, error) {
	stream, err := _cipherForKi(ki)
	if err != nil {
		return nil, err
	}
	return cipher.StreamWriter{S: stream, W: w}, nil
}

func _cipherForKi(ki KeyInfo) (cipher.Stream, error) {

	keyType, key, iv, err := ki.getSymmetricKey()
	if err != nil {
		return nil, err
	}

	switch keyType {
	case keyTypeAES:
		if len(key) != keySizeAES {
			return nil, fmt.Errorf("%w: invalid AES key size, expected %d bytes", ErrInvalidKey, keySizeAES)
		}
		if len(iv) != aes.BlockSize {
			return nil, fmt.Errorf("%w: invalid AES iv size, expected %d bytes", ErrInvalidKey, aes.BlockSize)
		}
		// Ignore error below, aes will fail only if the size of key is
		// invalid. We fully control it and pass valid value.
		block, _ := aes.NewCipher(key)
		return cipher.NewCTR(block, iv), nil

	case keyTypeChaCha20:
		if len(key) != chacha20.KeySize {
			return nil, fmt.Errorf("%w: invalid ChaCha20 key size, expected %d bytes", ErrInvalidKey, chacha20.KeySize)
		}
		if len(iv) != chacha20.NonceSize {
			return nil, fmt.Errorf("%w: invalid ChaCha20 iv size, expected %d bytes", ErrInvalidKey, chacha20.NonceSize)
		}
		return chacha20.NewUnauthenticatedCipher(key, iv)

	default:
		return nil, ErrInvalidKey
	}
}
