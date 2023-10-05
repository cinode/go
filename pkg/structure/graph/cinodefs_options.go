package graph

import (
	"context"
	"io"
	"time"
)

const (
	DefaultMaxLinksRedirects = 10
)

type CinodeFSOption interface {
	apply(ctx context.Context, fs *cinodeFS) error
}

type optionFunc func(ctx context.Context, fs *cinodeFS) error

func (f optionFunc) apply(ctx context.Context, fs *cinodeFS) error {
	return f(ctx, fs)
}

func MaxLinkRedirects(maxLinkRedirects int) CinodeFSOption {
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.maxLinkRedirects = maxLinkRedirects
		return nil
	})
}

func RootEntrypoint(ep *Entrypoint) CinodeFSOption {
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.rootEP = &cachedEntrypoint{stored: ep}
		return nil
	})
}

func RootEntrypointString(eps string) CinodeFSOption {
	ep, err := EntrypointFromString(eps)
	if err != nil {
		return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
			return err
		})
	}
	return RootEntrypoint(ep)
}

func TimeFunc(f func() time.Time) CinodeFSOption {
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.timeFunc = f
		return nil
	})
}

func RandSource(r io.Reader) CinodeFSOption {
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		fs.randSource = r
		return nil
	})
}

// NewRootDynamicLink option can be used to create completely new, random
// dynamic link as the root
func NewRootDynamicLink() CinodeFSOption {
	return optionFunc(func(ctx context.Context, fs *cinodeFS) error {
		newLinkEntrypoint, err := fs.generateNewDynamicLinkEntrypoint()
		if err != nil {
			return err
		}

		// Generate a simple dummy structure consisting of a root link
		// and an empty directory, all the entries are in-memory upon
		// creation and have to be flushed first to generate any
		// blobs
		fs.rootEP = &cachedEntrypoint{
			link: &linkCache{
				ep: newLinkEntrypoint,
				target: &cachedEntrypoint{
					dir: map[string]*cachedEntrypoint{},
				},
			},
		}
		return nil
	})
}
