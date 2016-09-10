package blenc

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestKeyGenertorContentsLimits(t *testing.T) {

	kg := ContentsHashKey()
	keyData := make([]byte, 1024)
	_, err := kg.GenerateKeyData(ioutil.NopCloser(bytes.NewReader([]byte{})), keyData)
	if err != errKeyDataToLarge {
		t.Fatalf("Invalid error received when trying to read to much key data: %v", err)
	}
}
