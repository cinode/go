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

package graph

import (
	"context"
	"time"

	"github.com/cinode/go/pkg/structure/internal/protobuf"
)

type EntrypointOption interface {
	apply(ctx context.Context, opts *entrypointOptions) error
}

type entrypointOptionBasicFunc func(opts *entrypointOptions)

func (ep entrypointOptionBasicFunc) apply(ctx context.Context, opts *entrypointOptions) error {
	ep(opts)
	return nil
}

type entrypointOptions struct {
	ep *protobuf.Entrypoint
}

func SetMimeType(mimeType string) EntrypointOption {
	return entrypointOptionBasicFunc(func(ep *entrypointOptions) {
		ep.ep.MimeType = mimeType
	})
}

func SetNotValidBefore(t time.Time) EntrypointOption {
	return entrypointOptionBasicFunc(func(opts *entrypointOptions) {
		opts.ep.NotValidBeforeUnixMicro = t.UnixMicro()
	})
}

func SetNotValidAfter(t time.Time) EntrypointOption {
	return entrypointOptionBasicFunc(func(opts *entrypointOptions) {
		opts.ep.NotValidAfterUnixMicro = t.UnixMicro()
	})
}

func protoEntrypointFromOptions(ctx context.Context, opts ...EntrypointOption) (*protobuf.Entrypoint, error) {
	scratchpad := entrypointOptions{ep: &protobuf.Entrypoint{}}
	for _, o := range opts {
		if err := o.apply(ctx, &scratchpad); err != nil {
			return nil, err
		}
	}
	return scratchpad.ep, nil
}
