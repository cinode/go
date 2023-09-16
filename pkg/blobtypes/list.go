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
	"fmt"

	"github.com/cinode/go/pkg/common"
)

var (
	Invalid     = common.NewBlobType(0x00)
	Static      = common.NewBlobType(0x01)
	DynamicLink = common.NewBlobType(0x02)
)

var All = map[string]common.BlobType{
	"Static":      Static,
	"DynamicLink": DynamicLink,
}

func ToName(t common.BlobType) string {
	for name, tp := range All {
		if tp == t {
			return name
		}
	}
	return fmt.Sprintf("Invalid(%d)", t.IDByte())
}
