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
	"errors"
	"fmt"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"golang.org/x/crypto/chacha20"
)

var (
	ErrMaxDynamicLinkRecursionDepthReached = errors.New("max dynamic link recursion depth reached")
)

const maxDynamicLinkRecursionDepth = 10 // TODO: This is just a Jeronimo number, most likely this should be 2 or 3

func (be *beDatastore) readDynamicLink(
	ctx context.Context,
	name common.BlobName,
	key EncryptionKey,
	w io.Writer,
	recursionDepth int,
) error {

	// TODO: Protect against long links - there should be max size limit and maybe some streaming involved?
	// TODO: Validate the encryption key to avoid forcing weak keys
	// TODO: Key info should not be a byte array, there should be some more abstract structure to allow more complex key lookup

	if recursionDepth >= maxDynamicLinkRecursionDepth {
		return ErrMaxDynamicLinkRecursionDepthReached
	}

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

	// Decrypt link info

	r, err := streamCipherReader(key, dl.IV, bytes.NewBuffer(dl.EncryptedLink))
	if err != nil {
		return err
	}

	unencryptedLink, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Ensure the IV does match to avoid enforcing weak IVs
	if !bytes.Equal(dl.IV, dl.CalculateIV(unencryptedLink)) {
		return fmt.Errorf("%w: invalid iv", blobtypes.ErrValidationFailed)
	}

	// Analyze and follow the link
	if len(unencryptedLink) < 1+chacha20.KeySize+1 {
		return fmt.Errorf("%w: invalid link data", blobtypes.ErrValidationFailed)
	}
	if unencryptedLink[0] != 0 { // reserved
		return fmt.Errorf("%w: invalid link data: reserved byte is not zero", blobtypes.ErrValidationFailed)
	}
	unencryptedLink = unencryptedLink[1:]

	redirectKey := unencryptedLink[:chacha20.KeySize]
	redirectName := common.BlobName(unencryptedLink[chacha20.KeySize:])

	return be.read(ctx, redirectName, redirectKey, w, recursionDepth+1)
}
