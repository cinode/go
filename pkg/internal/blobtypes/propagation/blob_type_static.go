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

package propagation

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"

	"github.com/cinode/go/pkg/internal/blobtypes"
)

var (
	ErrInvalidStaticBlobHash = fmt.Errorf("%w: invalid static blob hash", blobtypes.ErrValidationFailed)
)

type staticBlobHandler struct {
	newHasher func() hash.Hash
}

func (h staticBlobHandler) Ingest(hash []byte, current, update io.Reader, w io.Writer) error {
	// TODO: Optimize - at this point we are not required to ingest the update stream
	//       so if the current stream is valid (hash correct hash), we should cancel the
	//		 ingestion (but don't return an error, instead say that update is not needed)
	return h.Validate(hash, io.TeeReader(update, w))
}

func (h staticBlobHandler) Validate(hash []byte, data io.Reader) error {
	hasher := h.newHasher()

	if hasher.Size() != len(hash) {
		return ErrInvalidStaticBlobHash
	}

	_, err := io.Copy(hasher, data)
	if err != nil {
		return err
	}

	calculatedHash := hasher.Sum(nil)
	if !bytes.Equal(hash, calculatedHash) {
		return ErrInvalidStaticBlobHash
	}

	return nil
}

func newStaticBlobHandlerSha256() Handler {
	return staticBlobHandler{
		newHasher: sha256.New,
	}
}
