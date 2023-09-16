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
	"errors"
	"fmt"
	"io"
	"testing"
	"testing/iotest"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/utilities/securefifo"
	"github.com/stretchr/testify/require"
)

type sfwWrapper struct {
	w       securefifo.Writer
	writeFn func([]byte) (int, error)
	closeFn func() error
	doneFn  func() (securefifo.Reader, error)
}

func (w *sfwWrapper) Write(b []byte) (int, error) {
	if w.writeFn != nil {
		return w.writeFn(b)
	}
	return w.w.Write(b)
}

func (w *sfwWrapper) Close() error {
	if w.closeFn != nil {
		return w.closeFn()
	}
	return w.w.Close()
}

func (w *sfwWrapper) Done() (securefifo.Reader, error) {
	if w.doneFn != nil {
		return w.doneFn()
	}
	return w.w.Done()
}

func TestStaticErrorTruncatedDatastore(t *testing.T) {
	dsw := dsWrapper{DS: datastore.InMemory()}
	be := FromDatastore(&dsw)

	bn, key, _, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader([]byte("Hello world!")))
	require.NoError(t, err)

	t.Run("handle error while opening blob", func(t *testing.T) {
		injectedErr := errors.New("test")
		dsw.openFn = func(ctx context.Context, name common.BlobName) (io.ReadCloser, error) { return nil, injectedErr }

		rc, err := be.Open(context.Background(), bn, key)
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, rc)

		dsw.openFn = nil
	})

	t.Run("handle error while opening blob", func(t *testing.T) {
		injectedErr := errors.New("test")
		dsw.openFn = func(ctx context.Context, name common.BlobName) (io.ReadCloser, error) { return nil, injectedErr }

		rc, err := be.Open(context.Background(), bn, key)
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, rc)

		dsw.openFn = nil
	})

	t.Run("handle failures to create secure fifo", func(t *testing.T) {
		t.Run("first securefifo", func(t *testing.T) {
			be := FromDatastore(datastore.InMemory())
			injectedErr := errors.New("test")
			be.(*beDatastore).newSecureFifo = func() (securefifo.Writer, error) { return nil, injectedErr }

			bn, key, ai, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(nil))
			require.ErrorIs(t, err, injectedErr)
			require.Nil(t, bn)
			require.Nil(t, key)
			require.Nil(t, ai)
		})

		t.Run("second securefifo", func(t *testing.T) {
			be := FromDatastore(datastore.InMemory())
			injectedErr := errors.New("test")
			firstSecureFifoCreated := false
			firstSecureFifoClosed := false
			be.(*beDatastore).newSecureFifo = func() (securefifo.Writer, error) {
				if firstSecureFifoCreated {
					return nil, injectedErr
				}

				firstSecureFifoCreated = true
				w, err := securefifo.New()
				require.NoError(t, err)

				return &sfwWrapper{
					w: w,
					closeFn: func() error {
						firstSecureFifoClosed = true
						return w.Close()
					},
				}, nil
			}

			bn, key, ai, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(nil))
			require.ErrorIs(t, err, injectedErr)
			require.Nil(t, bn)
			require.Nil(t, key)
			require.Nil(t, ai)
			require.True(t, firstSecureFifoCreated)
			require.True(t, firstSecureFifoClosed)
		})
	})

	t.Run("fail to call Done on secure fifos", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				be := FromDatastore(datastore.InMemory())
				injectedErr := errors.New("test")
				secureFifosCreated := 0
				secureFifosClosed := 0

				be.(*beDatastore).newSecureFifo = func() (securefifo.Writer, error) {
					shouldReturnError := secureFifosCreated == i // Inject error on Done for i'th secure fifo
					secureFifosCreated++
					sf, err := securefifo.New()
					require.NoError(t, err)
					return &sfwWrapper{
						w: sf,
						closeFn: func() error {
							secureFifosClosed++
							return sf.Close()
						},
						doneFn: func() (securefifo.Reader, error) {
							if shouldReturnError {
								return nil, injectedErr
							}
							return sf.Done()
						},
					}, nil
				}

				bn, key, ai, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(nil))
				require.ErrorIs(t, err, injectedErr)
				require.Nil(t, bn)
				require.Nil(t, key)
				require.Nil(t, ai)
				require.Equal(t, 2, secureFifosCreated)
				require.Equal(t, secureFifosCreated, secureFifosClosed)
			})
		}
	})

	t.Run("fail to call write to secure fifos", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				be := FromDatastore(datastore.InMemory())
				injectedErr := errors.New("test")
				secureFifosCreated := 0
				secureFifosClosed := 0

				be.(*beDatastore).newSecureFifo = func() (securefifo.Writer, error) {
					shouldReturnError := secureFifosCreated == i // Inject error on Done for i'th secure fifo
					secureFifosCreated++
					sf, err := securefifo.New()
					require.NoError(t, err)
					return &sfwWrapper{
						w: sf,
						closeFn: func() error {
							secureFifosClosed++
							return sf.Close()
						},
						writeFn: func(b []byte) (int, error) {
							if shouldReturnError {
								return 0, injectedErr
							}
							return sf.Write(b)
						},
					}, nil
				}

				bn, key, ai, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader([]byte("Hello world")))
				require.ErrorIs(t, err, injectedErr)
				require.Nil(t, bn)
				require.Nil(t, key)
				require.Nil(t, ai)
				require.Equal(t, 2, secureFifosCreated)
				require.Equal(t, secureFifosCreated, secureFifosClosed)
			})
		}
	})

	t.Run("fail to read data", func(t *testing.T) {
		be := FromDatastore(datastore.InMemory())
		injectedErr := errors.New("test")
		secureFifosCreated := 0
		secureFifosClosed := 0

		// To check if secure fifos are closed correctly
		be.(*beDatastore).newSecureFifo = func() (securefifo.Writer, error) {
			secureFifosCreated++
			w, err := securefifo.New()
			require.NoError(t, err)

			return &sfwWrapper{
				w: w,
				closeFn: func() error {
					secureFifosClosed++
					return w.Close()
				},
			}, nil
		}

		bn, key, ai, err := be.Create(context.Background(), blobtypes.Static, iotest.ErrReader(injectedErr))
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, bn)
		require.Nil(t, key)
		require.Nil(t, ai)
		require.Equal(t, 2, secureFifosCreated)
		require.Equal(t, secureFifosCreated, secureFifosClosed)
	})

	t.Run("fail to store blob", func(t *testing.T) {
		injectedErr := errors.New("test")

		dsw := dsWrapper{DS: datastore.InMemory()}
		be := FromDatastore(&dsw)

		secureFifosCreated := 0
		secureFifosClosed := 0

		// To check if secure fifos are closed correctly
		be.(*beDatastore).newSecureFifo = func() (securefifo.Writer, error) {
			secureFifosCreated++
			w, err := securefifo.New()
			require.NoError(t, err)

			return &sfwWrapper{
				w: w,
				closeFn: func() error {
					secureFifosClosed++
					return w.Close()
				},
			}, nil
		}

		dsw.updateFn = func(ctx context.Context, name common.BlobName, r io.Reader) error { return injectedErr }

		bn, key, ai, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(nil))
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, bn)
		require.Nil(t, key)
		require.Nil(t, ai)

		require.Equal(t, 2, secureFifosCreated)
		require.Equal(t, secureFifosCreated, secureFifosClosed)
	})
}
