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

package blobtypes

import (
	"testing"

	"github.com/cinode/go/pkg/common"
	"github.com/stretchr/testify/require"
)

func TestToName(t *testing.T) {
	t.Run("valid known types", func(t *testing.T) {
		for name, bType := range All {
			require.Equal(t, name, ToName(bType))
		}
	})
	t.Run("invalid type", func(t *testing.T) {
		require.Equal(t, "Invalid(0)", ToName(Invalid))
		require.Equal(t, "Invalid(255)", ToName(common.NewBlobType(255)))
	})
}
