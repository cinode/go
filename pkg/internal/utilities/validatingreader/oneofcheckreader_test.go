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

package validatingreader_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/cinode/go/pkg/internal/utilities/validatingreader"
	"github.com/stretchr/testify/require"
)

func TestOnCloseCheckReader(t *testing.T) {
	t.Run("properly read if read function does not raise an error", func(t *testing.T) {
		r := validatingreader.CheckOnEOF(
			bytes.NewBufferString("Hello world"),
			func() error { return nil },
		)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, []byte("Hello world"), data)
	})

	t.Run("Properly catch failure during the check callback", func(t *testing.T) {
		injectedError := errors.New("test error")

		r := validatingreader.CheckOnEOF(
			io.NopCloser(bytes.NewBufferString("Hello world")),
			func() error { return injectedError },
		)

		_, err := io.ReadAll(r)
		require.Equal(t, err, injectedError)
	})
}
