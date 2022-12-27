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
	"fmt"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"golang.org/x/crypto/chacha20"
)

var (
	ErrInvalidDynamicLinkData          = errors.New("invalid dynamic link data")
	ErrInvalidDynamicLinkDataBlobName  = fmt.Errorf("%w: blob name mismatch", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkDataSignature = fmt.Errorf("%w: signature is invalid", ErrInvalidDynamicLinkData)
)

const (
	reservedByteValue byte = 0

	signatureForLinkData                byte = 0
	signatureForEncryptionKeyGeneration byte = 0xFF
)

type DynamicLinkData struct {
	PublicKey      ed25519.PublicKey
	ContentVersion uint64
	Signature      []byte
	IV             []byte
	EncryptedLink  []byte
}

func readBuff(r io.Reader, buff []byte, fieldName string) error {
	_, err := io.ReadFull(r, buff)
	if err != nil {
		return fmt.Errorf(
			"%w: error while reading %s: %v",
			ErrInvalidDynamicLinkData, fieldName, err,
		)
	}
	return nil
}

func readByte(r io.Reader, fieldName string) (byte, error) {
	var b [1]byte
	err := readBuff(r, b[:], fieldName)
	return b[0], err
}

func readUint64(r io.Reader, fieldName string) (uint64, error) {
	var b [8]byte
	err := readBuff(r, b[:], fieldName)
	return binary.BigEndian.Uint64(b[:]), err
}

// FromReader creates an encrypted dynamic link data from given io.Reader
//
// Invalid links are rejected - i.e. if there's any error while reading the data
// or when the validation of the link fails for whatever reason
func FromReader(name common.BlobName, r io.Reader) (*DynamicLinkData, error) {
	dl := DynamicLinkData{
		PublicKey: make([]byte, ed25519.PublicKeySize),
		Signature: make([]byte, ed25519.SignatureSize),
		IV:        make([]byte, chacha20.NonceSizeX),
	}

	reserved, err := readByte(r, "reserved byte")
	if err != nil {
		return nil, err
	}
	if reserved != reservedByteValue {
		return nil, fmt.Errorf(
			"%w: invalid value of the reserved byte: %d, expected 0",
			ErrInvalidDynamicLinkData, reserved,
		)
	}

	err = readBuff(r, dl.PublicKey, "public key")
	if err != nil {
		return nil, err
	}

	dl.ContentVersion, err = readUint64(r, "content version")
	if err != nil {
		return nil, err
	}

	err = readBuff(r, dl.Signature, "signature")
	if err != nil {
		return nil, err
	}

	err = readBuff(r, dl.IV, "iv")
	if err != nil {
		return nil, err
	}

	dl.EncryptedLink, err = io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	err = dl.verifyPublicData(name)
	if err != nil {
		return nil, err
	}

	return &dl, nil
}

func (d *DynamicLinkData) SendToWriter(w io.Writer) error {
	// Reserved
	_, err := w.Write([]byte{reservedByteValue})
	if err != nil {
		return err
	}

	// Public key
	_, err = w.Write(d.PublicKey)
	if err != nil {
		return err
	}

	// Content version
	_, err = w.Write(binary.BigEndian.AppendUint64(nil, d.ContentVersion))
	if err != nil {
		return err
	}

	// Signature
	_, err = w.Write(d.Signature)
	if err != nil {
		return err
	}

	// IV
	_, err = w.Write(d.IV)
	if err != nil {
		return err
	}

	// Encrypted link
	_, err = w.Write(d.EncryptedLink)
	if err != nil {
		return err
	}

	return nil
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

func (d *DynamicLinkData) verifyPublicData(name common.BlobName) error {
	if !bytes.Equal(name, d.BlobName()) {
		return ErrInvalidDynamicLinkDataBlobName
	}

	if !ed25519.Verify(d.PublicKey, d.bytesToSign(), d.Signature) {
		return ErrInvalidDynamicLinkDataSignature
	}

	return nil
}

func (d *DynamicLinkData) bytesToSign() []byte {
	b := bytes.NewBuffer(nil)

	// Content indicator
	b.WriteByte(signatureForLinkData)

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

func (d *DynamicLinkData) CalculateEncryptionKey(privKey ed25519.PrivateKey) []byte {
	dataSeed := append(
		[]byte{signatureForEncryptionKeyGeneration},
		d.BlobName()...,
	)

	// TODO: Add key validation block

	signature := ed25519.Sign(privKey, dataSeed)
	signatureHash := sha256.Sum256(signature)
	return signatureHash[:chacha20.KeySize]
}

func (d *DynamicLinkData) GreaterThan(d2 *DynamicLinkData) bool {
	// First step - compare versions
	if d.ContentVersion > d2.ContentVersion {
		return true
	}
	if d.ContentVersion < d2.ContentVersion {
		return false
	}

	// Second step - compare hashed signatures.

	hs1 := sha256.Sum256(d.Signature)
	hs2 := sha256.Sum256(d2.Signature)

	return bytes.Compare(hs1[:], hs2[:]) > 0
}
