/*
Copyright © 2023 Bartłomiej Święcki (byo)

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
	"testing"
	"testing/iotest"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/stretchr/testify/require"
)

type dsWrapper struct {
	datastore.DS
	openFn   func(ctx context.Context, name common.BlobName) (io.ReadCloser, error)
	updateFn func(ctx context.Context, name common.BlobName, r io.Reader) error
}

func (w *dsWrapper) Open(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	if w.openFn != nil {
		return w.openFn(ctx, name)
	}
	return w.DS.Open(ctx, name)
}

func (w *dsWrapper) Update(ctx context.Context, name common.BlobName, r io.Reader) error {
	if w.updateFn != nil {
		return w.updateFn(ctx, name, r)
	}
	return w.DS.Update(ctx, name, r)
}

type closeFunc struct {
	io.Reader
	closeFn func() error
}

func (c *closeFunc) Close() error { return c.closeFn() }

func TestDynamicLinkErrors(t *testing.T) {
	dsw := dsWrapper{DS: datastore.InMemory()}
	be := FromDatastore(&dsw)

	bn, key, _, err := be.Create(context.Background(), blobtypes.DynamicLink, bytes.NewReader([]byte("Hello world!")))
	require.NoError(t, err)

	t.Run("handle error while opening blob", func(t *testing.T) {
		injectedErr := errors.New("test")
		dsw.openFn = func(ctx context.Context, name common.BlobName) (io.ReadCloser, error) { return nil, injectedErr }

		rc, err := be.Open(context.Background(), bn, key)
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, rc)
	})

	t.Run("handle blob read errors", func(t *testing.T) {
		injectedErr := errors.New("test")

		for i := 0; ; i++ {
			closed := false
			dataLen := 0

			t.Run(fmt.Sprintf("error at byte %d", i), func(t *testing.T) {

				dsw.openFn = func(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
					origRC, err := dsw.DS.Open(ctx, name)
					require.NoError(t, err)

					data, err := io.ReadAll(origRC)
					require.NoError(t, err)

					dataLen = len(data) // store to figure out when to break

					err = origRC.Close()
					require.NoError(t, err)

					return &closeFunc{
						Reader:  io.MultiReader(bytes.NewReader(data[:i]), iotest.ErrReader(injectedErr)),
						closeFn: func() error { closed = true; return nil },
					}, nil
				}

				rc, err := be.Open(context.Background(), bn, key)
				require.ErrorIs(t, err, injectedErr)
				require.Nil(t, rc)
				require.True(t, closed)
			})

			if i >= dataLen {
				break
			}
		}

		dsw.openFn = nil
	})

	t.Run("fail to create dynamic link keypair", func(t *testing.T) {

		injectedErr := errors.New("test")

		be.(*beDatastore).rand = iotest.ErrReader(injectedErr)

		bn, key, ai, err := be.Create(context.Background(), blobtypes.DynamicLink, bytes.NewReader(nil))
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, bn)
		require.Nil(t, key)
		require.Nil(t, ai)

		be.(*beDatastore).rand = rand.Reader

	})

	t.Run("fail to store new dynamic link blob", func(t *testing.T) {
		injectedErr := errors.New("test")

		dsw.updateFn = func(ctx context.Context, name common.BlobName, r io.Reader) error { return injectedErr }

		bn, key, ai, err := be.Create(context.Background(), blobtypes.DynamicLink, bytes.NewReader(nil))
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, bn)
		require.Nil(t, key)
		require.Nil(t, ai)

		dsw.updateFn = nil
	})

	t.Run("fail to update new dynamic link blob", func(t *testing.T) {
		injectedErr := errors.New("test")

		bn, key, ai, err := be.Create(context.Background(), blobtypes.DynamicLink, bytes.NewReader(nil))
		require.NoError(t, err)

		dsw.updateFn = func(ctx context.Context, name common.BlobName, r io.Reader) error { return injectedErr }

		err = be.Update(context.Background(), bn, ai, key, bytes.NewReader(nil))
		require.ErrorIs(t, err, injectedErr)

		dsw.updateFn = nil
	})
}
