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

package cinodefs_test

import (
	"context"
	"errors"
	"testing"
	"testing/iotest"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/datastore"
	"github.com/stretchr/testify/require"
)

func TestInvalidCinodeFSOptions(t *testing.T) {
	t.Run("no blenc", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), nil)
		require.ErrorIs(t, err, cinodefs.ErrInvalidBE)
		require.Nil(t, cfs)
	})

	be := blenc.FromDatastore(datastore.InMemory())

	t.Run("no root info", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), be)
		require.ErrorIs(t, err, cinodefs.ErrMissingRootInfo)
		require.Nil(t, cfs)
	})

	t.Run("negative max links redirects", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), be,
			cinodefs.NewRootStaticDirectory(),
			cinodefs.MaxLinkRedirects(-1),
		)
		require.ErrorIs(t, err, cinodefs.ErrNegativeMaxLinksRedirects)
		require.Nil(t, cfs)
	})

	t.Run("invalid entrypoint string", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), be,
			cinodefs.RootEntrypointString(""),
		)
		require.ErrorIs(t, err, cinodefs.ErrInvalidEntrypointData)
		require.Nil(t, cfs)
	})

	t.Run("invalid writer info string", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), be,
			cinodefs.RootWriterInfoString(""),
		)
		require.ErrorIs(t, err, cinodefs.ErrInvalidWriterInfoData)
		require.Nil(t, cfs)
	})

	t.Run("invalid nil writer info", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), be,
			cinodefs.RootWriterInfo(nil),
		)
		require.ErrorIs(t, err, cinodefs.ErrInvalidWriterInfoData)
		require.Nil(t, cfs)
	})

	t.Run("invalid writer info", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), be,
			cinodefs.RootWriterInfo(&cinodefs.WriterInfo{}),
		)
		require.ErrorIs(t, err, cinodefs.ErrInvalidWriterInfoData)
		require.Nil(t, cfs)
	})

	t.Run("invalid time func", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), be,
			cinodefs.TimeFunc(nil),
		)
		require.ErrorIs(t, err, cinodefs.ErrInvalidNilTimeFunc)
		require.Nil(t, cfs)
	})

	t.Run("invalid nil random source", func(t *testing.T) {
		cfs, err := cinodefs.New(context.Background(), be,
			cinodefs.RandSource(nil),
		)
		require.ErrorIs(t, err, cinodefs.ErrInvalidNilRandSource)
		require.Nil(t, cfs)
	})

	t.Run("invalid random source", func(t *testing.T) {
		// Error will manifest itself while random data source
		// is needed which only takes place when new random
		// dynamic link is requested
		injectedErr := errors.New("random source error")
		cfs, err := cinodefs.New(context.Background(), be,
			cinodefs.RandSource(iotest.ErrReader(injectedErr)),
			cinodefs.NewRootDynamicLink(),
		)
		require.ErrorIs(t, err, injectedErr)
		require.Nil(t, cfs)
	})
}
