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
	"github.com/cinode/go/pkg/cinodefs/protobuf"
	"github.com/cinode/go/testvectors/testblobs"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestEntrypointFromStringFailures(t *testing.T) {
	for _, d := range []struct {
		s           string
		errContains string
	}{
		{"", "empty string"},
		{"not-a-base64-string!!!!!!!!", "not a base58 string"},
		{"aaaaaaaa", "protobuf parse error"},
	} {
		t.Run(d.s, func(t *testing.T) {
			wi, err := cinodefs.EntrypointFromString(d.s)
			require.ErrorIs(t, err, cinodefs.ErrInvalidEntrypointData)
			require.ErrorContains(t, err, d.errContains)
			require.Nil(t, wi)
		})
	}
}

func TestInvalidEntrypointData(t *testing.T) {
	for _, d := range []struct {
		n           string
		p           *protobuf.Entrypoint
		errContains string
	}{
		{
			"invalid blob name",
			&protobuf.Entrypoint{},
			"invalid blob name",
		},
		{
			"mime type set for link",
			&protobuf.Entrypoint{
				BlobName: testblobs.DynamicLink.BlobName.Bytes(),
				MimeType: "test-mimetype",
			},
			"link can not have mimetype set",
		},
	} {
		t.Run(d.n, func(t *testing.T) {
			bytes, err := proto.Marshal(d.p)
			require.NoError(t, err)

			ep, err := cinodefs.EntrypointFromBytes(bytes)
			require.ErrorIs(t, err, cinodefs.ErrInvalidEntrypointData)
			require.ErrorContains(t, err, d.errContains)
			require.Nil(t, ep)
		})
	}
}
