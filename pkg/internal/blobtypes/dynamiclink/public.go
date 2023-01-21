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
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"testing/iotest"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"github.com/cinode/go/pkg/internal/utilities/validatingreader"
	"golang.org/x/crypto/chacha20"
)

var (
	ErrInvalidDynamicLinkData             = fmt.Errorf("%w for dynamic link", blobtypes.ErrValidationFailed)
	ErrInvalidDynamicLinkDataReservedByte = fmt.Errorf("%w: invalid value of the reserved byte", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkDataBlobName     = fmt.Errorf("%w: blob name mismatch", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkDataSignature    = fmt.Errorf("%w: signature mismatch", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkDataTruncated    = fmt.Errorf("%w: data truncated", ErrInvalidDynamicLinkData)

	ErrInvalidDynamicLinkIV = fmt.Errorf("%w: invalid iv", ErrInvalidDynamicLinkData)
)

const (
	reservedByteValue byte = 0

	signatureForLinkData                byte = 0
	signatureForEncryptionKeyGeneration byte = 0xFF
)

// Public represents public link static data
//
// That identity corresponds to a single blob name
type Public struct {
	publicKey ed25519.PublicKey
	nonce     uint64
}

func (d *Public) BlobName() common.BlobName {
	hasher := sha256.New()
	hasher.Write([]byte{reservedByteValue})
	hasher.Write(d.publicKey)
	hasher.Write(storeUint64(d.nonce))

	bn, _ := common.BlobNameFromHashAndType(hasher.Sum(nil), blobtypes.DynamicLink)
	return bn
}

// PublicReader can be used to read publicly available information
// from given public data stream (or validate and stream the data out)
// The data can only be read once due to a streaming nature (it read
// the data on-the-fly from another reader).
type PublicReader struct {
	Public
	contentVersion uint64
	signature      []byte
	iv             []byte
	r              io.Reader
}

// FromPublicData creates an encrypted dynamic link data (public part) from given io.Reader
//
// Invalid links are rejected - i.e. if there's any error while reading the data
// or when the validation of the link fails for whatever reason
func FromPublicData(name common.BlobName, r io.Reader) (*PublicReader, error) {
	dl := PublicReader{
		Public: Public{
			publicKey: make([]byte, ed25519.PublicKeySize),
		},
		signature: make([]byte, ed25519.SignatureSize),
		iv:        make([]byte, chacha20.NonceSizeX),
	}

	// 1. Static data independent from the linked blob

	reserved, err := readByte(r, "reserved byte")
	if err != nil {
		return nil, err
	}
	if reserved != reservedByteValue {
		return nil, fmt.Errorf(
			"%w: %d, expected 0",
			ErrInvalidDynamicLinkDataReservedByte, reserved,
		)
	}

	err = readBuff(r, dl.publicKey, "public key")
	if err != nil {
		return nil, err
	}

	dl.nonce, err = readUint64(r, "nonce")
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(dl.BlobName(), name) {
		return nil, ErrInvalidDynamicLinkDataBlobName
	}

	// 2. Structures related to dynamic link data

	dl.contentVersion, err = readUint64(r, "content version")
	if err != nil {
		return nil, err
	}

	err = readBuff(r, dl.signature, "signature")
	if err != nil {
		return nil, err
	}

	err = readBuff(r, dl.iv, "iv")
	if err != nil {
		return nil, err
	}

	// Starting from validations at this point, errors are returned while reading.
	// This is to prepare for future improvements when real streaming is
	// introduced where those validations cal only be performed
	// after the whole data is read

	elink, err := func() ([]byte, error) {

		elink, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}

		encryptedLinkDataHasher := dl.toSignDataHasherPrefilled()
		encryptedLinkDataHasher.Write(elink)
		encryptedLinkDataHash := encryptedLinkDataHasher.Sum(nil)

		if !ed25519.Verify(
			dl.publicKey,
			encryptedLinkDataHash,
			dl.signature,
		) {
			return nil, ErrInvalidDynamicLinkDataSignature
		}

		return elink, nil

	}()

	if err != nil {
		dl.r = iotest.ErrReader(err)
	} else {
		dl.r = bytes.NewReader(elink)
	}

	return &dl, nil
}

func (d *PublicReader) GetEncryptedLinkReader() io.Reader {
	// Sanity check - the reader can only be taken once
	defer func() { d.r = nil }()
	if d.r == nil {
		panic("Reader can only be fetched once")
	}
	return d.r
}

func (d *PublicReader) GetPublicDataReader() io.Reader {

	// Preamble - static link data
	w := bytes.NewBuffer(nil)
	w.Write([]byte{reservedByteValue})
	w.Write(d.publicKey)
	w.Write(storeUint64(d.nonce))

	// Preamble - dynamic link data
	w.Write(storeUint64(d.contentVersion))
	w.Write(d.signature)
	w.Write(d.iv)

	return io.MultiReader(
		bytes.NewReader(w.Bytes()), // Preamble
		d.GetEncryptedLinkReader(), // Main link data
	)
}

func (d *PublicReader) toSignDataHasherPrefilled() hash.Hash {
	h := sha512.New()

	// Content indicator
	h.Write([]byte{signatureForLinkData})

	// Blob name, length-prefixed
	bn := d.BlobName()
	h.Write([]byte{byte(len(bn))})
	h.Write(bn)

	// Version
	h.Write(storeUint64(d.contentVersion))

	return h
}

func (d *PublicReader) GreaterThan(d2 *PublicReader) bool {
	// First step - compare versions
	if d.contentVersion > d2.contentVersion {
		return true
	}
	if d.contentVersion < d2.contentVersion {
		return false
	}

	// Second step - compare hashed signatures.

	hs1 := sha256.Sum256(d.signature)
	hs2 := sha256.Sum256(d2.signature)

	return bytes.Compare(hs1[:], hs2[:]) > 0
}

func (d *PublicReader) ivCalculationHasherPrefilled() hash.Hash {
	hasher := sha256.New()

	// Reserved byte
	hasher.Write([]byte{reservedByteValue})

	// Blob name, length-prefixed
	bn := d.BlobName()
	hasher.Write([]byte{byte(len(bn))})
	hasher.Write(bn)

	// Version
	hasher.Write(storeUint64(d.contentVersion))

	return hasher
}

func (d *PublicReader) GetLinkDataReader(key []byte) (io.Reader, error) {

	// TODO: Validate the key with key validation block

	r, err := cipherfactory.StreamCipherReader(key, d.iv, d.GetEncryptedLinkReader())
	if err != nil {
		return nil, err
	}

	// While reading the data, it will be tee-ed to the hasher for IV calculation.
	// That hasher will then
	ivHasher := d.ivCalculationHasherPrefilled()

	return validatingreader.CheckOnEOF(
		io.TeeReader(r, ivHasher),
		func() error {
			calculatedIV := ivHasher.Sum(nil)[:chacha20.NonceSizeX]

			if !bytes.Equal(calculatedIV, d.iv) {
				return ErrInvalidDynamicLinkIV
			}

			return nil
		},
	), nil
}
