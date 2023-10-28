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

package cinodefs

import (
	"context"
	"time"
)

type EntrypointOption interface {
	apply(ctx context.Context, ep *Entrypoint) error
}

type entrypointOptionBasicFunc func(ep *Entrypoint)

func (f entrypointOptionBasicFunc) apply(ctx context.Context, ep *Entrypoint) error {
	f(ep)
	return nil
}

func SetMimeType(mimeType string) EntrypointOption {
	return entrypointOptionBasicFunc(func(ep *Entrypoint) {
		ep.ep.MimeType = mimeType
	})
}

func SetNotValidBefore(t time.Time) EntrypointOption {
	return entrypointOptionBasicFunc(func(ep *Entrypoint) {
		ep.ep.NotValidBeforeUnixMicro = t.UnixMicro()
	})
}

func SetNotValidAfter(t time.Time) EntrypointOption {
	return entrypointOptionBasicFunc(func(ep *Entrypoint) {
		ep.ep.NotValidAfterUnixMicro = t.UnixMicro()
	})
}

func entrypointFromOptions(ctx context.Context, opts ...EntrypointOption) (*Entrypoint, error) {
	ep := &Entrypoint{}
	for _, o := range opts {
		if err := o.apply(ctx, ep); err != nil {
			return nil, err
		}
	}
	return ep, nil
}
