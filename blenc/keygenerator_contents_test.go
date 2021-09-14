package blenc

import (
	"bytes"
	"io"
	"testing"
)

func TestKeyGeneratorContentsLimits(t *testing.T) {

	kg := ContentsHashKey()
	keyData := make([]byte, 1024)
	_, err := kg.GenerateKeyData(io.NopCloser(bytes.NewReader([]byte{})), keyData)
	if err != errKeyDataToLarge {
		t.Fatalf("Invalid error received when trying to read to much key data: %v", err)
	}
}
