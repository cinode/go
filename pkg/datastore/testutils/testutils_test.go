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

package testutils

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBReaderOnRead(t *testing.T) {
	onReadErr := errors.New("onRead test")
	rdr := BReader([]byte{1, 2, 3}, func() error {
		return onReadErr
	}, nil)

	buf := make([]byte, 3)
	n, err := rdr.Read(buf)
	require.Zero(t, n)
	require.ErrorIs(t, err, onReadErr)
}

func TestBReaderOnEOF(t *testing.T) {
	var onEOFErr error
	rdr := BReader([]byte{1, 2, 3}, nil, func() error {
		return onEOFErr
	})

	buf := make([]byte, 3)
	n, err := rdr.Read(buf)
	require.Equal(t, 3, n)
	require.NoError(t, err)

	n, err = rdr.Read(buf)
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF)

	onEOFErr = errors.New("onEOF test")
	n, err = rdr.Read(buf)
	require.Zero(t, n)
	require.ErrorIs(t, err, onEOFErr)
}
