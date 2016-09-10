package blenc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"testing"
)

func TestReaderHashValidatorResults(t *testing.T) {

	for _, d := range []struct {
		data string
		hash string
		err  error
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil},
		{"", "a3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", errHashValidationError},
	} {
		hash, _ := hex.DecodeString(d.hash)
		rdr := newReaderHashValidator(
			ioutil.NopCloser(bytes.NewReader([]byte(d.data))),
			sha256.New(),
			hash,
		)

		buff := bytes.Buffer{}

		_, err := io.Copy(&buff, rdr)
		if err != d.err {
			t.Fatalf("When hashing buffer '%v': got error %v instead of %v",
				d.data, err, d.err)
		}
	}

}
