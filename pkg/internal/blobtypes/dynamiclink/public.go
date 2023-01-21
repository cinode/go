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
	"fmt"
	"hash"
	"io"
	"testing/iotest"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"github.com/cinode/go/pkg/internal/utilities/validatingreader"
)

var (
	ErrInvalidDynamicLinkData             = fmt.Errorf("%w for dynamic link", blobtypes.ErrValidationFailed)
	ErrInvalidDynamicLinkDataReservedByte = fmt.Errorf("%w: invalid value of the reserved byte", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkDataBlobName     = fmt.Errorf("%w: blob name mismatch", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkDataSignature    = fmt.Errorf("%w: signature mismatch", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkDataTruncated    = fmt.Errorf("%w: data truncated", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkDataBlockSize    = fmt.Errorf("%w: block size too large", ErrInvalidDynamicLinkData)

	ErrInvalidDynamicLinkIVMismatch                     = fmt.Errorf("%w: iv mismatch", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkKeyMismatch                    = fmt.Errorf("%w: key mismatch", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkKeyValidationBlock             = fmt.Errorf("%w: invalid key validation block", ErrInvalidDynamicLinkData)
	ErrInvalidDynamicLinkKeyValidationBlockReservedByte = fmt.Errorf("%w reserved byte", ErrInvalidDynamicLinkKeyValidationBlock)
	ErrInvalidDynamicLinkKeyValidationBlockSignature    = fmt.Errorf("%w signature", ErrInvalidDynamicLinkKeyValidationBlock)
)

const (
	reservedByteValue byte = 0

	signatureForLinkData                byte = 0x00
	signatureForEncryptionKeyGeneration byte = 0x01
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

	storeByte(hasher, reservedByteValue)
	storeBuff(hasher, d.publicKey)
	storeUint64(hasher, d.nonce)

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

	dl.iv, err = readDynamicSizeBuff(r, "iv")
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

	storeByte(w, reservedByteValue)
	storeBuff(w, d.publicKey)
	storeUint64(w, d.nonce)

	// Preamble - dynamic link data
	storeUint64(w, d.contentVersion)
	storeBuff(w, d.signature)
	storeDynamicSizeBuff(w, d.iv)

	return io.MultiReader(
		bytes.NewReader(w.Bytes()), // Preamble
		d.GetEncryptedLinkReader(), // Main link data
	)
}

func (d *PublicReader) toSignDataHasherPrefilled() hash.Hash {
	h := sha256.New()

	storeByte(h, signatureForLinkData)
	storeDynamicSizeBuff(h, d.BlobName())
	storeUint64(h, d.contentVersion)

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

func (d *PublicReader) ivGeneratorPrefilled() cipherfactory.IVGenerator {
	ivGenerator := cipherfactory.NewIVGenerator(blobtypes.DynamicLink)

	storeDynamicSizeBuff(ivGenerator, d.BlobName())
	storeUint64(ivGenerator, d.contentVersion)

	return ivGenerator
}

func (d *PublicReader) validateKeyInLinkData(key cipherfactory.Key, r io.Reader) error {
	// At the beginning of the data there's the key validation block,
	// that block contains a proof that the encryption key was deterministically derived
	// from the blob name (thus preventing weak key attack)

	kvb, err := readDynamicSizeBuff(r, "key validation block")
	if err != nil {
		return err
	}
	if len(kvb) == 0 || kvb[0] != reservedByteValue {
		return ErrInvalidDynamicLinkKeyValidationBlockReservedByte
	}
	signature := kvb[1:]

	dataSeed := append(
		[]byte{signatureForEncryptionKeyGeneration},
		d.BlobName()...,
	)

	// Key validation block contains the signature of data seed
	if !ed25519.Verify(d.publicKey, dataSeed, signature) {
		return ErrInvalidDynamicLinkKeyValidationBlockSignature
	}

	// That signature is fed into the key generator and builds the key
	keyGenerator := cipherfactory.NewKeyGenerator(blobtypes.DynamicLink)
	keyGenerator.Write(signature)
	generatedKey := keyGenerator.Generate()

	if !bytes.Equal(generatedKey, key) {
		return ErrInvalidDynamicLinkKeyMismatch
	}

	return nil
}

func (d *PublicReader) GetLinkDataReader(key cipherfactory.Key) (io.Reader, error) {

	r, err := cipherfactory.StreamCipherReader(key, d.iv, d.GetEncryptedLinkReader())
	if err != nil {
		return nil, err
	}

	// While reading the data, it will be tee-ed to the hasher for IV calculation.
	// That hasher will then
	ivHasher := d.ivGeneratorPrefilled()
	r = io.TeeReader(r, ivHasher)

	err = d.validateKeyInLinkData(key, r)
	if err != nil {
		return nil, err
	}

	return validatingreader.CheckOnEOF(
		r,
		func() error {
			if !bytes.Equal(ivHasher.Generate(), d.iv) {
				return ErrInvalidDynamicLinkIVMismatch
			}

			return nil
		},
	), nil
}
