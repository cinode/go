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
	"encoding/binary"
	"io"

	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
)

type PublisherDynamicLinkData struct {
	DynamicLinkData
	privKey ed25519.PrivateKey
}

func Create(randSource io.Reader) (*PublisherDynamicLinkData, error) {
	pubKey, privKey, err := ed25519.GenerateKey(randSource)
	if err != nil {
		return nil, err
	}

	var nonceBytes [8]byte
	_, err = io.ReadFull(randSource, nonceBytes[:])
	if err != nil {
		return nil, err
	}
	nonce := binary.BigEndian.Uint64(nonceBytes[:])

	return &PublisherDynamicLinkData{
		DynamicLinkData: DynamicLinkData{
			PublicKey: pubKey,
			Nonce:     nonce,
		},
		privKey: privKey,
	}, nil
}

func FromAuthInfo(authInfo []byte) (*PublisherDynamicLinkData, error) {
	if len(authInfo) != 1+ed25519.SeedSize+8 || authInfo[0] != 0 {
		return nil, ErrInvalidDynamicLinkAuthInfo
	}

	privKey := ed25519.NewKeyFromSeed(authInfo[1 : 1+ed25519.SeedSize])
	pubKey := privKey.Public().(ed25519.PublicKey)
	nonce := binary.BigEndian.Uint64(authInfo[1+ed25519.SeedSize:])

	return &PublisherDynamicLinkData{
		DynamicLinkData: DynamicLinkData{
			PublicKey: pubKey,
			Nonce:     nonce,
		},
		privKey: privKey,
	}, nil
}

func (dl *PublisherDynamicLinkData) AuthInfo() []byte {
	var ret [1 + ed25519.SeedSize + 8]byte
	ret[0] = 0
	copy(ret[1:], dl.privKey.Seed())
	binary.BigEndian.PutUint64(ret[1+ed25519.SeedSize:], dl.Nonce)
	return ret[:]
}

func (dl *PublisherDynamicLinkData) UpdateLinkData(r io.Reader, version uint64) ([]byte, error) {
	unencryptedLink, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	dl.ContentVersion = version
	dl.IV = dl.CalculateIV(unencryptedLink)

	encryptionKey := dl.CalculateEncryptionKey(dl.privKey)

	encryptedLinkBuff := bytes.NewBuffer(nil)
	w, err := cipherfactory.StreamCipherWriter(encryptionKey, dl.IV, encryptedLinkBuff)
	if err != nil {
		return nil, err
	}

	_, err = w.Write(unencryptedLink)
	if err != nil {
		return nil, err
	}

	dl.EncryptedLink = encryptedLinkBuff.Bytes()
	dl.Signature = ed25519.Sign(dl.privKey, dl.bytesToSign())

	return encryptionKey, nil
}
