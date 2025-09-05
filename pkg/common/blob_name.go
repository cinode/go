/*
Copyright © 2025 Bartłomiej Święcki (byo)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"crypto/subtle"
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
type BlobName struct {
	bn []byte
}

// BlobNameFromHashAndType generates the name of a blob from some hash (e.g. sha256 of blob's content)
// and given blob type
func BlobNameFromHashAndType(hash []byte, t BlobType) (*BlobName, error) {
	if len(hash) == 0 || len(hash) > 0x7E {
		return nil, ErrInvalidBlobName
	}

	bn := make([]byte, len(hash)+1)

	copy(bn[1:], hash)

	scrambledTypeByte := t.t
	for _, b := range hash {
		scrambledTypeByte ^= b
	}
	bn[0] = scrambledTypeByte

	return &BlobName{bn: bn}, nil
}

// BlobNameFromString decodes base58-encoded string into blob name
func BlobNameFromString(s string) (*BlobName, error) {
	return BlobNameFromBytes(base58.Decode(s))
}

func BlobNameFromBytes(n []byte) (*BlobName, error) {
	if len(n) == 0 || len(n) > 0x7F {
		return nil, ErrInvalidBlobName
	}
	return &BlobName{bn: copyBytes(n)}, nil
}

// Returns base58-encoded blob name
func (b *BlobName) String() string {
	return base58.Encode(b.bn)
}

// Extracts hash from blob name
func (b *BlobName) Hash() []byte {
	return b.bn[1:]
}

// Extracts blob type from the name
func (b *BlobName) Type() BlobType {
	ret := byte(0)
	for _, by := range b.bn {
		ret ^= by
	}
	return BlobType{t: ret}
}

func (b *BlobName) Bytes() []byte {
	return copyBytes(b.bn)
}

func (b *BlobName) Equal(b2 *BlobName) bool {
	return subtle.ConstantTimeCompare(b.bn, b2.bn) == 1
}
