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

package blenc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
)

var (
	ErrInvalidDynamicLinkWriterInfo = errors.New("incorrect dynamic link writer info")
)

func (be *beDatastore) readDynamicLink(
	ctx context.Context,
	name common.BlobName,
	key EncryptionKey,
	w io.Writer,
) error {

	// TODO: Protect against long links - there should be max size limit and maybe some streaming involved?
	// TODO: Validate the encryption key to avoid forcing weak keys
	// TODO: Key info should not be a byte array, there should be some more abstract structure to allow more complex key lookup
	// TODO: Prefer a stream-like approach when dealing with link data

	buff := bytes.NewBuffer(nil)

	err := be.ds.Read(ctx, name, buff)
	if err != nil {
		return err
	}

	dl, err := dynamiclink.DynamicLinkDataFromBytes(buff.Bytes())
	if err != nil {
		return err
	}

	// Ensure the public key is correct
	if !bytes.Equal(name, dl.BlobName()) {
		return fmt.Errorf("%w: blob name does not match the public key", blobtypes.ErrValidationFailed)
	}

	// Ensure the signature is correct
	if !dl.Verify() {
		return fmt.Errorf("%w: invalid signature", blobtypes.ErrValidationFailed)
	}

	// Decrypt link data
	r, err := streamCipherReader(key, dl.IV, bytes.NewBuffer(dl.EncryptedLink))
	if err != nil {
		return err
	}

	unencryptedLink, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Send the link to writer
	// Note: we're doing this before validating the IV - that's to ensure that this code can easily be converted
	// to a stream-based approach later where the writer can get the data before the link is fully validated
	_, err = w.Write(unencryptedLink)
	if err != nil {
		return err
	}

	// Ensure the IV does match to avoid enforcing weak IVs
	if !bytes.Equal(dl.IV, dl.CalculateIV(unencryptedLink)) {
		return fmt.Errorf("%w: invalid iv", blobtypes.ErrValidationFailed)
	}

	return nil
}

func (be *beDatastore) createDynamicLink(
	ctx context.Context,
	r io.Reader,
) (
	common.BlobName,
	EncryptionKey,
	WriterInfo,
	error,
) {
	// TODO: Customizable random source
	// TODO: Customizable version source

	unencryptedLink, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, nil, err
	}

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	dl := dynamiclink.DynamicLinkData{
		PublicKey:      pubKey,
		ContentVersion: uint64(time.Now().UnixMicro()),
	}

	dl.IV = dl.CalculateIV(unencryptedLink)

	// Generate deterministic encryption key
	encryptionKey := dl.CalculateEncryptionKey(privKey)

	// Encrypt the link
	encryptedLinkBuff := bytes.NewBuffer(nil)
	w, err := streamCipherWriter(encryptionKey, dl.IV, encryptedLinkBuff)
	if err != nil {
		return nil, nil, nil, err
	}

	_, err = w.Write(unencryptedLink)
	if err != nil {
		return nil, nil, nil, err
	}

	dl.EncryptedLink = encryptedLinkBuff.Bytes()

	// Signature
	dl.Signature = dl.CalculateSignature(privKey)

	// Send update packet
	bn := dl.BlobName()
	err = be.ds.Update(ctx, bn, bytes.NewReader(dl.ToBytes()))
	if err != nil {
		return nil, nil, nil, err
	}

	return bn,
		encryptionKey,
		append([]byte{0}, privKey.Seed()...),
		nil
}

func (be *beDatastore) updateDynamicLink(
	ctx context.Context,
	name common.BlobName,
	wi WriterInfo,
	key EncryptionKey,
	r io.Reader,
) error {
	// TODO: Customizable version source

	unencryptedLink, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if len(wi) != 1+ed25519.SeedSize || wi[0] != 0 {
		return ErrInvalidDynamicLinkWriterInfo
	}

	privKey := ed25519.NewKeyFromSeed(wi[1:])
	pubKey := privKey.Public().(ed25519.PublicKey)

	dl := dynamiclink.DynamicLinkData{
		PublicKey:      pubKey,
		ContentVersion: uint64(time.Now().UnixMicro()),
	}

	dl.IV = dl.CalculateIV(unencryptedLink)

	// Generate deterministic encryption key
	if !bytes.Equal(dl.CalculateEncryptionKey(privKey), key) {
		return errors.New("could not prepare dynamic link update - encryption key mismatch")
	}

	// Encrypt the link
	encryptedLinkBuff := bytes.NewBuffer(nil)
	w, err := streamCipherWriter(key, dl.IV, encryptedLinkBuff)
	if err != nil {
		return err
	}

	_, err = w.Write(unencryptedLink)
	if err != nil {
		return err
	}

	dl.EncryptedLink = encryptedLinkBuff.Bytes()

	// Signature
	dl.Signature = dl.CalculateSignature(privKey)

	// Send update packet
	if !bytes.Equal(name, dl.BlobName()) {
		return errors.New("could not prepare dynamic link update - blob name mismatch")
	}
	err = be.ds.Update(ctx, name, bytes.NewReader(dl.ToBytes()))
	if err != nil {
		return err
	}

	return nil
}
