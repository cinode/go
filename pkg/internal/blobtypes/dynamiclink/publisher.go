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
	"io"

	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"golang.org/x/crypto/chacha20"
)

var (
	ErrInvalidDynamicLinkAuthInfo = errors.New("invalid dynamic link auth info")
)

type Publisher struct {
	Public
	privKey ed25519.PrivateKey
}

func nonceFromRand(randSource io.Reader) (uint64, error) {
	var nonceBytes [8]byte
	_, err := io.ReadFull(randSource, nonceBytes[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(nonceBytes[:]), nil
}

func Create(randSource io.Reader) (*Publisher, error) {
	pubKey, privKey, err := ed25519.GenerateKey(randSource)
	if err != nil {
		return nil, err
	}

	nonce, err := nonceFromRand(randSource)
	if err != nil {
		return nil, err
	}

	return &Publisher{
		Public: Public{
			publicKey: pubKey,
			nonce:     nonce,
		},
		privKey: privKey,
	}, nil
}

func FromAuthInfo(authInfo []byte) (*Publisher, error) {
	if len(authInfo) != 1+ed25519.SeedSize+8 || authInfo[0] != 0 {
		return nil, ErrInvalidDynamicLinkAuthInfo
	}

	privKey := ed25519.NewKeyFromSeed(authInfo[1 : 1+ed25519.SeedSize])
	pubKey := privKey.Public().(ed25519.PublicKey)
	nonce := binary.BigEndian.Uint64(authInfo[1+ed25519.SeedSize:])

	return &Publisher{
		Public: Public{
			publicKey: pubKey,
			nonce:     nonce,
		},
		privKey: privKey,
	}, nil
}

func ReNonce(p *Publisher, randSource io.Reader) (*Publisher, error) {
	nonce, err := nonceFromRand(randSource)
	if err != nil {
		return nil, err
	}

	return &Publisher{
		Public: Public{
			publicKey: p.publicKey,
			nonce:     nonce,
		},
		privKey: p.privKey,
	}, nil
}

func (dl *Publisher) AuthInfo() []byte {
	var ret [1 + ed25519.SeedSize + 8]byte
	ret[0] = reservedByteValue
	copy(ret[1:], dl.privKey.Seed())
	binary.BigEndian.PutUint64(ret[1+ed25519.SeedSize:], dl.nonce)
	return ret[:]
}

func (dl *Publisher) calculateEncryptionKey() []byte {
	dataSeed := append(
		[]byte{signatureForEncryptionKeyGeneration},
		dl.BlobName()...,
	)

	// TODO: Add key validation block

	signature := ed25519.Sign(dl.privKey, dataSeed)
	signatureHash := sha256.Sum256(signature)
	return signatureHash[:chacha20.KeySize]
}

func (dl *Publisher) UpdateLinkData(r io.Reader, version uint64) (*PublicReader, []byte, error) {
	unencryptedLink, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}

	pr := PublicReader{
		Public: dl.Public,
	}

	pr.contentVersion = version

	hasher := pr.ivCalculationHasherPrefilled()
	hasher.Write(unencryptedLink)
	pr.iv = hasher.Sum(nil)[:chacha20.NonceSizeX]

	encryptionKey := dl.calculateEncryptionKey()

	encryptedLinkBuff := bytes.NewBuffer(nil)
	w, err := cipherfactory.StreamCipherWriter(encryptionKey, pr.iv, encryptedLinkBuff)
	if err != nil {
		return nil, nil, err
	}

	_, err = w.Write(unencryptedLink)
	if err != nil {
		return nil, nil, err
	}

	signatureHasher := pr.toSignDataHasherPrefilled()
	signatureHasher.Write(encryptedLinkBuff.Bytes())

	pr.signature = ed25519.Sign(dl.privKey, signatureHasher.Sum(nil))
	pr.r = bytes.NewReader(encryptedLinkBuff.Bytes())

	return &pr, encryptionKey, nil
}
