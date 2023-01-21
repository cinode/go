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

package dynamiclink

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPanicIf(t *testing.T) {
	t.Run("no panic", func(t *testing.T) {
		panicIf(false, "no panic")
	})
	t.Run("panic", func(t *testing.T) {
		require.Panics(t, func() {
			panicIf(true, "panic")
		})
	})
}
