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
	"fmt"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
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
