package blenc

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
)

func TestStreamCipherReaderForKey(t *testing.T) {

	for _, d := range []struct {
		key  string
		err  error
		desc string
	}{
		{"", ErrInvalidKey, "Empty key"},
		{"#", ErrInvalidKey, "Non base58 character"},
		{"UBYspbTy9mx6SBnRrVeDyEcQQqnytz9WZ", nil, "Valid AES key"},
		{"eJEA1E4dRcx3zy59mu4BcD25kf86n5ydSEuDAojhQu3F", nil, "Valid chacha20 key"},
	} {
		sc, err := streamCipherReaderForKey(
			d.key, ioutil.NopCloser(bytes.NewReader([]byte{})))
		if d.err != err {
			t.Fatalf("In test for %v: Invalid error returned, expected %v, got %v",
				d.desc, d.err, err)
		}

		if err == nil {
			if sc == nil {
				t.Fatalf("In test for %v: Empty stream reader received but no error reported",
					d.desc)
			}
		}
	}
}

func TestKeyFromKeyData(t *testing.T) {

	for _, d := range []struct {
		keyType byte
		keyData []byte
		key     string
		desc    string
	}{
		{keyTypeAES, []byte(strings.Repeat("*", 24)), "UBYspbTy9mx6SBnRrVeDyEcQQqnytz9WZ", "Standard AES key"},
		{keyTypeAES, []byte(strings.Repeat("*", 64)), "UBYspbTy9mx6SBnRrVeDyEcQQqnytz9WZ", "AES key with extra bytes"},
		{keyTypeChaCha20, []byte(strings.Repeat("*", 32)), "eJEA1E4dRcx3zy59mu4BcD25kf86n5ydSEuDAojhQu3F", "Standard ChaCha20 key"},
		{keyTypeChaCha20, []byte(strings.Repeat("*", 64)), "eJEA1E4dRcx3zy59mu4BcD25kf86n5ydSEuDAojhQu3F", "ChaCha20 key with extra bytes"},
	} {

		key := keyFromKeyData(d.keyType, d.keyData)
		if key != d.key {
			t.Fatalf(
				"In test for %v: Invalid key returned, got '%v', expected '%v'",
				d.desc, key, d.key)
		}
	}
}

func TestStreamCipherReaderForKeyData(t *testing.T) {
	for _, d := range []struct {
		keyType byte
		keyData []byte
		strict  bool
		err     error
		desc    string
	}{
		{keyTypeAES, []byte(strings.Repeat("*", 24)), false, nil, "Normal AES key - no strict"},
		{keyTypeAES, []byte(strings.Repeat("*", 24)), true, nil, "Normal AES key - strict"},
		{keyTypeAES, []byte(strings.Repeat("*", 24-1)), false, ErrInvalidKey, "Normal AES key - short key"},
		{keyTypeAES, []byte(strings.Repeat("*", 24-1)), true, ErrInvalidKey, "Normal AES key - short key, strict"},
		{keyTypeAES, []byte(strings.Repeat("*", 24+1)), false, nil, "Normal AES key - long key"},
		{keyTypeAES, []byte(strings.Repeat("*", 24+1)), true, ErrInvalidKey, "Normal AES key - long key, strict"},
		{keyTypeChaCha20, []byte(strings.Repeat("*", 32)), false, nil, "Normal ChaCha20 key - no strict"},
		{keyTypeChaCha20, []byte(strings.Repeat("*", 32)), true, nil, "Normal ChaCha20 key - strict"},
		{keyTypeChaCha20, []byte(strings.Repeat("*", 32-1)), false, ErrInvalidKey, "Normal ChaCha20 key - short key"},
		{keyTypeChaCha20, []byte(strings.Repeat("*", 32-1)), true, ErrInvalidKey, "Normal ChaCha20 key - short key, strict"},
		{keyTypeChaCha20, []byte(strings.Repeat("*", 32+1)), false, nil, "Normal ChaCha20 key - long key"},
		{keyTypeChaCha20, []byte(strings.Repeat("*", 32+1)), true, ErrInvalidKey, "Normal ChaCha20 key - long key, strict"},
		{keyTypeInvalid, []byte(strings.Repeat("*", 32+1)), false, ErrInvalidKey, "Invalid key type"},
	} {
		rc, err := streamCipherReaderForKeyData(d.keyType, d.keyData,
			ioutil.NopCloser(bytes.NewReader([]byte{})), d.strict)
		if err != d.err {
			t.Fatalf("In test for %v: Invalid error received, expected %v, got %v",
				d.desc, d.err, err)
		}
		if rc != nil {
			if err != nil {
				t.Fatalf("In test for %v: Got reader back although error (%v) received",
					d.desc, err)
			}
			rc.Close()
		} else {
			if err == nil {
				t.Fatalf("In test for %v: Neither reader nor error returned",
					d.desc)
			}
		}
	}
}
