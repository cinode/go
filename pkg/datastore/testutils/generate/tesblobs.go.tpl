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
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/jbenet/go-base58"
)

// nolint:lll // test data vectors
var TestBlobs = []struct {
	Name     *common.BlobName
	Data     []byte
	Expected []byte
}{
{{- range .TestBlobs }}
	{
		golang.Must(common.BlobNameFromString("{{ .Name }}")),
		base58.Decode("{{ .Data }}"),
		base58.Decode("{{ .Expected }}"),
	},
{{- end }}
}

// nolint:lll // test data vectors
var DynamicLinkPropagationData = []struct {
	Name     *common.BlobName
	Data     []byte
	Expected []byte
}{
{{- range .DynamicLinkPropagationData }}
	{
		golang.Must(common.BlobNameFromString("{{ .Name }}")),
		base58.Decode("{{ .Data }}"),
		base58.Decode("{{ .Expected }}"),
	},
{{- end }}
}
