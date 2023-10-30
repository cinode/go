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
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cinode/go/pkg/common"
)

const (
	DefaultMaxLinksRedirects = 10
)

var (
	ErrNegativeMaxLinksRedirects = errors.New("negative value of maximum links redirects")
	ErrInvalidNilTimeFunc        = errors.New("nil time function")
	ErrInvalidNilRandSource      = errors.New("nil random source")
)

type Option interface {
	apply(ctx context.Context, fs *cinodeFS) error
}

type errOption struct{ err error }

func (e errOption) apply(ctx context.Context, fs *cinodeFS) error { return e.err }

type optionFunc func(ctx context.Context, fs *cinodeFS) error

func (f optionFunc) apply(ctx context.Context, fs *cinodeFS) error {
	return f(ctx, fs)
}

func MaxLinkRedirects(maxLinkRedirects int) Option {
	if maxLinkRedirects < 0 {
		return errOption{ErrNegativeMaxLinksRedirects}
	}
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.maxLinkRedirects = maxLinkRedirects
		return nil
	})
}

func RootEntrypoint(ep *Entrypoint) Option {
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.rootEP = &nodeUnloaded{ep: ep}
		return nil
	})
}

func RootEntrypointString(eps string) Option {
	ep, err := EntrypointFromString(eps)
	if err != nil {
		return errOption{err}
	}
	return RootEntrypoint(ep)
}

func RootWriterInfo(wi *WriterInfo) Option {
	if wi == nil {
		return errOption{fmt.Errorf(
			"%w: nil",
			ErrInvalidWriterInfoData,
		)}
	}
	bn, err := common.BlobNameFromBytes(wi.wi.BlobName)
	if err != nil {
		return errOption{fmt.Errorf(
			"%w: %w",
			ErrInvalidWriterInfoData,
			err,
		)}
	}

	key := common.BlobKeyFromBytes(wi.wi.Key)
	ep := EntrypointFromBlobNameAndKey(bn, key)

	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.rootEP = &nodeUnloaded{ep: ep}
		fs.c.authInfos[bn.String()] = common.AuthInfoFromBytes(wi.wi.AuthInfo)
		return nil
	})
}

func RootWriterInfoString(wis string) Option {
	wi, err := WriterInfoFromString(wis)
	if err != nil {
		return errOption{err}
	}

	return RootWriterInfo(wi)
}

func TimeFunc(f func() time.Time) Option {
	if f == nil {
		return errOption{ErrInvalidNilTimeFunc}
	}
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.timeFunc = f
		return nil
	})
}

func RandSource(r io.Reader) Option {
	if r == nil {
		return errOption{ErrInvalidNilRandSource}
	}
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.randSource = r
		return nil
	})
}

// NewRootDynamicLink option can be used to create completely new, random
// dynamic link as the root
func NewRootDynamicLink() Option {
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		newLinkEntrypoint, _, err := fs.generateNewDynamicLinkEntrypoint()
		if err != nil {
			return err
		}

		// Generate a simple dummy structure consisting of a root link
		// and an empty directory, all the entries are in-memory upon
		// creation and have to be flushed first to generate any
		// blobs
		fs.rootEP = &nodeLink{
			ep:     newLinkEntrypoint,
			dState: dsSubDirty,
			target: &nodeDirectory{
				entries: map[string]node{},
				dState:  dsDirty,
			},
		}
		return nil
	})
}

// NewRootDynamicLink option can be used to create completely new, random
// dynamic link as the root
func NewRootStaticDirectory() Option {
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.rootEP = &nodeDirectory{
			entries: map[string]node{},
			dState:  dsDirty,
		}
		return nil
	})
}
