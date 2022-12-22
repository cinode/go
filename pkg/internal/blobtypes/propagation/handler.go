/*
Copyright © 2022 Bartłomiej Święcki (byo)

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

package propagation

import (
	"io"
)

// Handler is an object responsible for processing public propagation of blobs
// with given type. It is not meant to process a particular blob.
type Handler interface {
	// Validate reads data from given reader and ensures it passes the validation
	// according to the blob type. In case of validation error, this method should
	// return an error that satisfies `errors.Is(err, ErrValidationFailed)` filter.
	Validate(hash []byte, data io.Reader) error

	// Ingest is responsible for merging the `current` dataset with an incoming `update`
	// data. The result of the merge will be written into the `result` stream.
	//
	// It is the responsibility of this method to ensure data in the `current` and `update`
	// streams contains valid data.
	Ingest(hash []byte, current, update io.Reader, result io.Writer) error
}
