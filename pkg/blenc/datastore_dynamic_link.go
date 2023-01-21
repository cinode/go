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
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
)

var (
	ErrInvalidDynamicLinkWriterInfo = errors.New("incorrect dynamic link writer info")
)

func (be *beDatastore) openDynamicLink(
	ctx context.Context,
	name common.BlobName,
	key EncryptionKey,
) (
	io.ReadCloser,
	error,
) {

	// TODO: Protect against long links - there should be max size limit and maybe some streaming involved?
	// TODO: Validate the encryption key to avoid forcing weak keys
	// TODO: Key info should not be a byte array, there should be some more abstract structure to allow more complex key lookup
	// TODO: Prefer a stream-like approach when dealing with link data

	rc, err := be.ds.Open(ctx, name)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	dl, err := dynamiclink.FromReader(name, rc)
	if err != nil {
		return nil, err
	}

	// Decrypt link data
	r, err := cipherfactory.StreamCipherReader(key, dl.IV, bytes.NewBuffer(dl.EncryptedLink))
	if err != nil {
		return nil, err
	}

	unencryptedLink, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Ensure the IV does match to avoid enforcing weak IVs
	if !bytes.Equal(dl.IV, dl.CalculateIV(unencryptedLink)) {
		return nil, fmt.Errorf("%w: invalid iv", blobtypes.ErrValidationFailed)
	}

	return io.NopCloser(bytes.NewReader(unencryptedLink)), nil
}

func (be *beDatastore) createDynamicLink(
	ctx context.Context,
	r io.Reader,
) (
	common.BlobName,
	EncryptionKey,
	AuthInfo,
	error,
) {
	// TODO: Customizable random source
	// TODO: Customizable version source

	version := uint64(time.Now().UnixMicro())
	randSource := rand.Reader

	dl, err := dynamiclink.Create(randSource)
	if err != nil {
		return nil, nil, nil, err
	}

	encryptionKey, err := dl.UpdateLinkData(r, version)
	if err != nil {
		return nil, nil, nil, err
	}

	// Send update packet
	// TODO: A bit awkward to use the pipe here, but that's since the datastore.Update takes reader as a parameter
	// maybe we should consider different interfaces in datastore?
	bn := dl.BlobName()
	err = be.ds.Update(ctx, bn, dl.CreateReader())
	if err != nil {
		return nil, nil, nil, err
	}

	return bn,
		encryptionKey,
		dl.AuthInfo(),
		nil
}

func (be *beDatastore) updateDynamicLink(
	ctx context.Context,
	name common.BlobName,
	authInfo AuthInfo,
	key EncryptionKey,
	r io.Reader,
) error {
	// TODO: Customizable version source
	newVersion := uint64(time.Now().UnixMicro())

	dl, err := dynamiclink.FromAuthInfo(authInfo)
	if err != nil {
		return err
	}

	encryptionKey, err := dl.UpdateLinkData(r, newVersion)
	if err != nil {
		return err
	}

	// Sanity checks
	// TODO: Have some test cases trying to attack lack of those checks - e.g. invalid key generated for the blog

	if !bytes.Equal(encryptionKey, key) {
		return errors.New("could not prepare dynamic link update - encryption key mismatch")
	}
	if !bytes.Equal(name, dl.BlobName()) {
		return errors.New("could not prepare dynamic link update - blob name mismatch")
	}

	// Send update packet
	err = be.ds.Update(ctx, name, dl.CreateReader())
	if err != nil {
		return err
	}

	return nil
}
