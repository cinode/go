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

package generation

import (
	"crypto/sha256"
	"hash"
	"io"
)

type staticBlobHandler struct {
	newHasher func() hash.Hash
}

// PrepareNewBlob generates the blob name from blob's content hash.
// There's no WriterInfo generated since anybody can create any static blob.
func (h staticBlobHandler) PrepareNewBlob(data io.Reader) ([]byte, WriterInfo, error) {
	hasher := h.newHasher()

	_, err := io.Copy(hasher, data)
	if err != nil {
		return nil, nil, err
	}

	return hasher.Sum(nil), nil, nil
}

func (h staticBlobHandler) SendUpdate(hash []byte, wi WriterInfo, data io.Reader, out io.Writer) error {
	// TODO: Shouldn't we validate the data?
	_, err := io.Copy(out, data)
	return err
}

func newStaticBlobHandlerSha256() Handler {
	return staticBlobHandler{
		newHasher: sha256.New,
	}
}
