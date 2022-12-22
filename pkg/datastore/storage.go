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

package datastore

import (
	"context"
	"io"

	"github.com/cinode/go/pkg/common"
)

type WriteCloseCanceller interface {
	io.WriteCloser
	Cancel()
}

type storage interface {
	kind() string
	openReadStream(ctx context.Context, name common.BlobName) (io.ReadCloser, error)
	openWriteStream(ctx context.Context, name common.BlobName) (WriteCloseCanceller, error)
	exists(ctx context.Context, name common.BlobName) (bool, error)
	delete(ctx context.Context, name common.BlobName) error
}
