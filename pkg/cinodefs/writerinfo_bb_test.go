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
	"testing"

	"github.com/cinode/go/pkg/cinodefs"
	"github.com/stretchr/testify/require"
)

func TestWriterInfoFromStringFailures(t *testing.T) {
	for _, d := range []struct {
		s           string
		errContains string
	}{
		{"", "empty string"},
		{"not-a-base64-string!!!!!!!!", "not a base58 string"},
		{"aaaaaaaa", "protobuf parse error"},
	} {
		t.Run(d.s, func(t *testing.T) {
			wi, err := cinodefs.WriterInfoFromString(d.s)
			require.ErrorIs(t, err, cinodefs.ErrInvalidWriterInfoData)
			require.ErrorContains(t, err, d.errContains)
			require.Nil(t, wi)
		})
	}
}
