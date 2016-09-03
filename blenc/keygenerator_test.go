package blenc

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"strings"
	"testing"
)

func TestEqualData(t *testing.T) {
	for _, data := range []string{
		"",
		"a",
		"abc",
		"9876543210123456789098765432101234567890",
		strings.Repeat("data", 1025),
	} {
		data := []byte(data)
		allKG(func(kg KeyDataGenerator) {
			key := make([]byte, 32)
			s, err := kg.GenerateKeyData(ioutil.NopCloser(bytes.NewReader(data)), key)
			errPanic(err)
			defer s.Close()
			read, err := ioutil.ReadAll(s)
			errPanic(err)
			if !bytes.Equal(data, read) {
				t.Fatalf("Data read from stream after key generation is invalid")
			}

			key2 := make([]byte, 32)
			_, err = kg.GenerateKeyData(ioutil.NopCloser(bytes.NewReader(data)), key2)
			errPanic(err)
			if kg.IsDeterministic() {
				if !bytes.Equal(key, key2) {
					t.Fatalf("Deterministic key generator produced different key for the same data")
				}
			} else {
				if bytes.Equal(key, key2) {
					t.Fatalf("Non-deterministic key generator produced same key for the same data\n"+
						"Key data: %v", hex.EncodeToString(key))
				}
			}
		})
	}
}
