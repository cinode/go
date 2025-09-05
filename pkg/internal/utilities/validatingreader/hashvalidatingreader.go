/*
Copyright © 2025 Bartłomiej Święcki (byo)

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

package validatingreader

import (
	"bytes"
	"hash"
	"io"
)

type hashValidatingReader struct {
	r            io.Reader
	hasher       hash.Hash
	err          error
	expectedHash []byte
}

func (h hashValidatingReader) Read(b []byte) (int, error) {
	n, err := h.r.Read(b)
	h.hasher.Write(b[:n])

	if err == io.EOF && !bytes.Equal(h.expectedHash, h.hasher.Sum(nil)) {
		return n, h.err
	}

	return n, err
}

func NewHashValidation(
	r io.Reader,
	hasher hash.Hash,
	expectedHash []byte,
	err error,
) io.Reader {
	return &hashValidatingReader{
		r:            r,
		hasher:       hasher,
		expectedHash: expectedHash,
		err:          err,
	}
}
