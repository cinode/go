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

package datastore

import (
	"bytes"
	"crypto/sha256"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
)

var emptyBlobNameStatic = func() common.BlobName {
	bn, err := common.BlobNameFromHashAndType(sha256.New().Sum(nil), blobtypes.Static)
	if err != nil {
		panic(err)
	}
	return bn
}()

var emptyBlobNameDynamicLink = func() common.BlobName {
	bn, err := common.BlobNameFromHashAndType(sha256.New().Sum(nil), blobtypes.DynamicLink)
	if err != nil {
		panic(err)
	}
	return bn
}()

var emptyBlobNamesOfAllTypes = []common.BlobName{
	emptyBlobNameStatic,
	emptyBlobNameDynamicLink,
}

type helperReader struct {
	buf    io.Reader
	onRead func() error
	onEOF  func() error
}

func bReader(b []byte, onRead func() error, onEOF func() error) *helperReader {

	nop := func() error {
		return nil
	}

	if onRead == nil {
		onRead = nop
	}
	if onEOF == nil {
		onEOF = nop
	}

	return &helperReader{
		buf:    bytes.NewReader(b),
		onRead: onRead,
		onEOF:  onEOF,
	}
}

func (h *helperReader) Read(b []byte) (n int, err error) {
	err = h.onRead()
	if err != nil {
		return 0, err
	}

	n, err = h.buf.Read(b)
	if err == io.EOF {
		err = h.onEOF()
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}

	return n, err
}
