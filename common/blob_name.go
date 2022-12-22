package common

import (
	"errors"

	base58 "github.com/jbenet/go-base58"
)

var (
	ErrInvalidBlobName = errors.New("invalid blob name")
)

// BlobName is used to identify blobs.
// Internally it is a single array of bytes that represents
// both the type of the blob and internal hash used to create that blob.
// The type of the blob is not stored directly. Instead it is mixed
// with the hash of the blob to make sure that all bytes in the blob name
// are randomly distributed.
type BlobName []byte

// BlobNameFromHashAndType generates the name of a blob from some hash (e.g. sha256 of blob's content)
// and given blob type
func BlobNameFromHashAndType(hash []byte, t BlobType) (BlobName, error) {
	if len(hash) == 0 {
		return nil, ErrInvalidBlobName
	}

	ret := make([]byte, len(hash)+1)

	copy(ret[1:], hash)

	scrambledTypeByte := byte(t.t)
	for _, b := range hash {
		scrambledTypeByte ^= b
	}
	ret[0] = scrambledTypeByte

	return BlobName(ret), nil
}

// BlobNameFromString decodes base58-encoded string into blob name
func BlobNameFromString(s string) (BlobName, error) {
	decoded := base58.Decode(s)
	if len(decoded) == 0 {
		return nil, ErrInvalidBlobName
	}
	return BlobName(decoded), nil
}

// Returns base58-encoded blob name
func (b BlobName) String() string {
	return base58.Encode(b)
}

// Extracts hash from blob name
func (b BlobName) Hash() []byte {
	return b[1:]
}

// Extracts blob type from the name
func (b BlobName) Type() BlobType {
	ret := byte(0)
	for _, by := range b {
		ret ^= by
	}
	return BlobType{t: ret}
}
