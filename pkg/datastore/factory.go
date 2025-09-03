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

package datastore

import (
	"fmt"
	"strings"
)

const (
	filePrefix     = "file://"
	rawFilePrefix  = "file-raw://"
	webPrefixHTTP  = "http://"
	webPrefixHTTPS = "https://"
	memoryPrefix   = "memory://"
)

var (
	ErrInvalidMemoryLocation = fmt.Errorf("memory datastore must not use any parameters, use only `%s`", memoryPrefix)
)

// FromLocation creates new instance of the datastore from location string.
//
// The string may be of the following form:
//   - file://<path> - create datastore using local filesystem's path (optimized) as the storage,
//     see InFileSystem for more details
//   - file-raw://<path> - create datastore using local filesystem's path (simplified) as the storage,
//     see InRawFileSystem for more details
//   - http://<address> or https://<address> - connects to datastore exposed through a http protocol,
//     see FromWeb for more details
//   - memory:// - creates a local in-process datastore without persistent storage
//   - <path> - equivalent to file://<path>
func FromLocation(location string) (DS, error) {
	switch {
	case strings.HasPrefix(location, filePrefix):
		return InFileSystem(location[len(filePrefix):])

	case strings.HasPrefix(location, rawFilePrefix):
		return InRawFileSystem(location[len(rawFilePrefix):])

	case strings.HasPrefix(location, webPrefixHTTP),
		strings.HasPrefix(location, webPrefixHTTPS):
		return FromWeb(location)

	case strings.HasPrefix(location, memoryPrefix):
		if location != memoryPrefix {
			return nil, ErrInvalidMemoryLocation
		}
		return InMemory(), nil

	default:
		return InFileSystem(location)
	}
}
