/*
Copyright © 2022 Bartłomiej Święcki (byo)

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

package dynamiclink

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"golang.org/x/crypto/chacha20"
)

var (
	ErrInvalidDynamicLinkData = errors.New("invalid dynamic link data")
)

const (
	reservedByteValue byte = 0
)

type DynamicLinkData struct {
	PublicKey      ed25519.PublicKey
	ContentVersion uint64
	Signature      []byte
	IV             []byte
	EncryptedLink  []byte
}

func DynamicLinkDataFromBytes(b []byte) (*DynamicLinkData, error) {
	// Reserved byte
	if len(b) < 1 {
		return nil, ErrInvalidDynamicLinkData
	}
	if b[0] != reservedByteValue {
		return nil, ErrInvalidDynamicLinkData
	}

	// Public key
	if len(b) < ed25519.PrivateKeySize {
		return nil, ErrInvalidDynamicLinkData
	}
	pubKey := ed25519.PublicKey(b[:ed25519.PrivateKeySize])
	b = b[ed25519.PrivateKeySize:]

	// Content version
	if len(b) < 8 {
		return nil, ErrInvalidDynamicLinkData
	}
	contentVersion := binary.BigEndian.Uint64(b)
	b = b[8:]

	// Signature
	if len(b) < ed25519.SignatureSize {
		return nil, ErrInvalidDynamicLinkData
	}
	signature := b[:ed25519.SignatureSize]
	b = b[ed25519.SignatureSize:]

	// IV
	// TODO: The size of IV may potentially leak the cipher used
	if len(b) < chacha20.NonceSizeX {
		return nil, ErrInvalidDynamicLinkData
	}
	iv := b[:chacha20.NonceSizeX]
	b = b[chacha20.NonceSizeX:]

	// Link data
	encryptedLink := b

	return &DynamicLinkData{
		PublicKey:      pubKey,
		ContentVersion: contentVersion,
		Signature:      signature,
		IV:             iv,
		EncryptedLink:  encryptedLink,
	}, nil
}

func (d *DynamicLinkData) ToBytes() []byte {
	b := bytes.NewBuffer(nil)

	// Reserved
	b.WriteByte(reservedByteValue)

	// Public key
	b.Write(d.PublicKey)

	// Content version
	b.Write(binary.BigEndian.AppendUint64(nil, d.ContentVersion))

	// Signature
	b.Write(d.Signature)

	// IV
	b.Write(d.IV)

	// Encrypted link
	b.Write(d.EncryptedLink)

	return b.Bytes()
}

func (d *DynamicLinkData) CalculateIV(unencryptedLink []byte) []byte {
	hasher := sha256.New()

	// Reserved byte
	hasher.Write([]byte{reservedByteValue})

	// Blob name, length-prefixed
	bn := d.BlobName()
	hasher.Write([]byte{byte(len(bn))})
	hasher.Write(bn)

	// Version
	hasher.Write(binary.BigEndian.AppendUint64(nil, d.ContentVersion))

	// Plaintext link
	hasher.Write(unencryptedLink)

	return hasher.Sum(nil)[:chacha20.NonceSizeX]
}

func (d *DynamicLinkData) CalculateSignature(privKey ed25519.PrivateKey) []byte {
	return ed25519.Sign(privKey, d.bytesToSign())
}

func (d *DynamicLinkData) Verify() bool {
	return ed25519.Verify(d.PublicKey, d.bytesToSign(), d.Signature)
}

func (d *DynamicLinkData) bytesToSign() []byte {
	b := bytes.NewBuffer(nil)

	// Blob name, length-prefixed
	bn := d.BlobName()
	b.WriteByte(byte(len(bn)))
	b.Write(bn)

	// Version
	b.Write(binary.BigEndian.AppendUint64(nil, d.ContentVersion))

	// Encrypted link
	b.Write(d.EncryptedLink)

	return b.Bytes()
}

func (d *DynamicLinkData) BlobName() common.BlobName {
	hasher := sha256.New()
	hasher.Write([]byte{reservedByteValue})
	hasher.Write(d.PublicKey)

	bn, _ := common.BlobNameFromHashAndType(hasher.Sum(nil), blobtypes.DynamicLink)
	return bn
}
