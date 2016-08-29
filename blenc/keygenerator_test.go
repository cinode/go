package blenc

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
)

func allKG(f func(kg KeyGenerator)) {

	func() {
		f(constantKey("testkey"))
	}()

}

func TestEqualData(t *testing.T) {
	for _, data := range []string{
		"",
		"a",
		"abc",
		"9876543210123456789098765432101234567890",
		strings.Repeat("data", 1025),
	} {
		data := []byte(data)
		allKG(func(kg KeyGenerator) {
			_, s, err := kg.GenerateKey(ioutil.NopCloser(bytes.NewReader(data)))
			errPanic(err)
			defer s.Close()
			read, err := ioutil.ReadAll(s)
			errPanic(err)
			if !bytes.Equal(data, read) {
				t.Fatalf("Data read from stream after key generation is invalid")
			}
		})
	}
}
