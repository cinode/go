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

package cinodefs

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/cinode/go/pkg/internal/utilities/headwriter"
	"github.com/cinode/go/pkg/utilities/golang"
)

var (
	ErrInvalidBE                 = errors.New("invalid BE argument")
	ErrCantOpenDir               = errors.New("can not open directory")
	ErrCantOpenDirDuplicateEntry = fmt.Errorf("%w: duplicate entry", ErrCantOpenDir)
	ErrCantOpenLink              = errors.New("can not open link")
	ErrTooManyRedirects          = errors.New("too many link redirects")
	ErrCantComputeBlobKey        = errors.New("can not compute blob keys")
	ErrModifiedDirectory         = errors.New("can not get entrypoint for a directory, unsaved content")
	ErrCantDeleteRoot            = errors.New("can not delete root object")
	ErrNotADirectory             = errors.New("entry is not a directory")
	ErrNotALink                  = errors.New("entry is not a link")
	ErrNilEntrypoint             = errors.New("nil entrypoint")
	ErrEmptyName                 = errors.New("entry name can not be empty")
	ErrEntryNotFound             = errors.New("entry not found")
	ErrIsADirectory              = errors.New("entry is a directory")
	ErrInvalidDirectoryData      = errors.New("invalid directory data")
	ErrCantWriteDirectory        = errors.New("can not write directory")
	ErrMissingRootInfo           = errors.New("root info not specified")
)

const (
	CinodeDirMimeType = "application/cinode-dir"
)

type FS interface {
	SetEntryFile(
		ctx context.Context,
		path []string,
		data io.Reader,
		opts ...EntrypointOption,
	) (*Entrypoint, error)

	CreateFileEntrypoint(
		ctx context.Context,
		data io.Reader,
		opts ...EntrypointOption,
	) (*Entrypoint, error)

	SetEntry(
		ctx context.Context,
		path []string,
		ep *Entrypoint,
	) error

	ResetDir(
		ctx context.Context,
		path []string,
	) error

	Flush(
		ctx context.Context,
	) error

	FindEntry(
		ctx context.Context,
		path []string,
	) (*Entrypoint, error)

	DeleteEntry(
		ctx context.Context,
		path []string,
	) error

	InjectDynamicLink(
		ctx context.Context,
		path []string,
	) (
		*WriterInfo,
		error,
	)

	OpenEntryData(
		ctx context.Context,
		path []string,
	) (io.ReadCloser, error)

	OpenEntrypointData(
		ctx context.Context,
		ep *Entrypoint,
	) (io.ReadCloser, error)

	RootEntrypoint() (*Entrypoint, error)

	EntrypointWriterInfo(
		ctx context.Context,
		ep *Entrypoint,
	) (*WriterInfo, error)

	RootWriterInfo(
		ctx context.Context,
	) (*WriterInfo, error)
}

type cinodeFS struct {
	c                graphContext
	randSource       io.Reader
	rootEP           node
	timeFunc         func() time.Time
	maxLinkRedirects int
}

func New(
	ctx context.Context,
	be blenc.BE,
	options ...Option,
) (FS, error) {
	if be == nil {
		return nil, ErrInvalidBE
	}

	ret := cinodeFS{
		maxLinkRedirects: DefaultMaxLinksRedirects,
		timeFunc:         time.Now,
		randSource:       rand.Reader,
		c: graphContext{
			be:        be,
			authInfos: map[string]*common.AuthInfo{},
		},
	}

	for _, opt := range options {
		err := opt.apply(ctx, &ret)
		if err != nil {
			return nil, err
		}
	}

	if ret.rootEP == nil {
		return nil, ErrMissingRootInfo
	}

	return &ret, nil
}

func (fs *cinodeFS) SetEntryFile(
	ctx context.Context,
	path []string,
	data io.Reader,
	opts ...EntrypointOption,
) (*Entrypoint, error) {
	ep := entrypointFromOptions(ctx, opts...)
	if ep.ep.MimeType == "" && len(path) > 0 {
		// Try detecting mime type from filename extension
		ep.ep.MimeType = mime.TypeByExtension(filepath.Ext(path[len(path)-1]))
	}

	ep, err := fs.createFileEntrypoint(ctx, data, ep)
	if err != nil {
		return nil, err
	}

	err = fs.SetEntry(ctx, path, ep)
	if err != nil {
		return nil, err
	}

	return ep, nil
}

func (fs *cinodeFS) CreateFileEntrypoint(
	ctx context.Context,
	data io.Reader,
	opts ...EntrypointOption,
) (*Entrypoint, error) {
	ep := entrypointFromOptions(ctx, opts...)
	return fs.createFileEntrypoint(ctx, data, ep)
}

func (fs *cinodeFS) createFileEntrypoint(
	ctx context.Context,
	data io.Reader,
	ep *Entrypoint,
) (*Entrypoint, error) {
	var hw headwriter.Writer

	if ep.ep.MimeType == "" {
		// detect mimetype from the content
		hw = headwriter.New(512)
		data = io.TeeReader(data, &hw)
	}

	bn, key, _, err := fs.c.be.Create(ctx, blobtypes.Static, data)
	if err != nil {
		return nil, err
	}

	if ep.ep.MimeType == "" {
		ep.ep.MimeType = http.DetectContentType(hw.Head())
	}

	return setEntrypointBlobNameAndKey(bn, key, ep), nil
}

func (fs *cinodeFS) SetEntry(
	ctx context.Context,
	path []string,
	ep *Entrypoint,
) error {
	whenReached := func(
		ctx context.Context,
		current node,
		isWriteable bool,
	) (node, dirtyState, error) {
		if !isWriteable {
			return nil, 0, ErrMissingWriterInfo
		}
		return &nodeUnloaded{ep: ep}, dsDirty, nil
	}

	return fs.traverseGraph(
		ctx,
		path,
		traverseOptions{
			createNodes:      true,
			maxLinkRedirects: fs.maxLinkRedirects,
		},
		whenReached,
	)
}

func (fs *cinodeFS) ResetDir(ctx context.Context, path []string) error {
	whenReached := func(
		ctx context.Context,
		current node,
		isWriteable bool,
	) (node, dirtyState, error) {
		if !isWriteable {
			return nil, 0, ErrMissingWriterInfo
		}
		return &nodeDirectory{
			entries: map[string]node{},
			dState:  dsDirty,
		}, dsDirty, nil
	}

	return fs.traverseGraph(
		ctx,
		path,
		traverseOptions{
			createNodes:      true,
			maxLinkRedirects: fs.maxLinkRedirects,
		},
		whenReached,
	)
}

func (fs *cinodeFS) Flush(ctx context.Context) error {
	_, newRootEP, err := fs.rootEP.flush(ctx, &fs.c)
	if err != nil {
		return err
	}

	fs.rootEP = &nodeUnloaded{ep: newRootEP}
	return nil
}

func (fs *cinodeFS) FindEntry(ctx context.Context, path []string) (*Entrypoint, error) {
	var ret *Entrypoint
	err := fs.traverseGraph(
		ctx,
		path,
		traverseOptions{
			doNotCache: true,
		},
		func(_ context.Context, ep node, _ bool) (node, dirtyState, error) {
			var subErr error
			ret, subErr = ep.entrypoint()
			return ep, dsClean, subErr
		},
	)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (fs *cinodeFS) DeleteEntry(ctx context.Context, path []string) error {
	// Entry removal is done on the parent level, we find the parent directory
	// and remove the entry from its list
	if len(path) == 0 {
		return ErrCantDeleteRoot
	}

	return fs.traverseGraph(
		ctx,
		path[:len(path)-1],
		traverseOptions{createNodes: true},
		func(_ context.Context, reachedEntrypoint node, isWriteable bool) (node, dirtyState, error) {
			if !isWriteable {
				return nil, 0, ErrMissingWriterInfo
			}

			dir, isDir := reachedEntrypoint.(*nodeDirectory)
			if !isDir {
				return nil, 0, ErrNotADirectory
			}

			if !dir.deleteEntry(path[len(path)-1]) {
				return nil, 0, ErrEntryNotFound
			}

			return dir, dsDirty, nil
		},
	)
}

func (fs *cinodeFS) InjectDynamicLink(
	ctx context.Context,
	path []string,
) (
	*WriterInfo,
	error,
) {
	var retWi *WriterInfo

	whenReached := func(
		ctx context.Context,
		current node,
		isWriteable bool,
	) (node, dirtyState, error) {
		if !isWriteable {
			return nil, 0, ErrMissingWriterInfo
		}

		ep, ai, err := fs.generateNewDynamicLinkEntrypoint()
		if err != nil {
			return nil, 0, err
		}

		key, err := fs.c.keyFromEntrypoint(ctx, ep)
		if err != nil {
			return nil, 0, err
		}

		retWi = writerInfoFromBlobNameKeyAndAuthInfo(ep.BlobName(), key, ai)
		return &nodeLink{
				ep:     ep,
				target: current,
				// Link itself must be marked as dirty - even if the content is clean,
				// the link itself must be persisted
				dState: dsSubDirty,
			},
			// Parent node becomes dirty - new link is a new blob
			dsDirty,
			nil
	}

	err := fs.traverseGraph(
		ctx,
		path,
		traverseOptions{
			createNodes:      true,
			maxLinkRedirects: fs.maxLinkRedirects,
		},
		whenReached,
	)
	if err != nil {
		return nil, err
	}

	return retWi, nil
}

func (fs *cinodeFS) generateNewDynamicLinkEntrypoint() (*Entrypoint, *common.AuthInfo, error) {
	// Generate new entrypoint link data but do not yet store it in datastore
	link, err := dynamiclink.Create(fs.randSource)
	if err != nil {
		return nil, nil, err
	}

	bn := link.BlobName()
	key := link.EncryptionKey()
	ai := link.AuthInfo()

	fs.c.authInfos[bn.String()] = ai

	return EntrypointFromBlobNameAndKey(bn, key), ai, nil
}

func (fs *cinodeFS) OpenEntryData(ctx context.Context, path []string) (io.ReadCloser, error) {
	ep, err := fs.FindEntry(ctx, path)
	if err != nil {
		return nil, err
	}
	if ep.IsDir() {
		return nil, ErrIsADirectory
	}
	golang.Assert(
		!ep.IsLink(),
		"assumed that fs.FindEntry does not return a link",
	)

	return fs.OpenEntrypointData(ctx, ep)
}

func (fs *cinodeFS) OpenEntrypointData(ctx context.Context, ep *Entrypoint) (io.ReadCloser, error) {
	if ep == nil {
		return nil, ErrNilEntrypoint
	}

	return fs.c.getDataReader(ctx, ep)
}

func (fs *cinodeFS) RootEntrypoint() (*Entrypoint, error) {
	return fs.rootEP.entrypoint()
}

func (fs *cinodeFS) EntrypointWriterInfo(ctx context.Context, ep *Entrypoint) (*WriterInfo, error) {
	if !ep.IsLink() {
		return nil, ErrNotALink
	}

	bn := ep.BlobName()

	key, err := fs.c.keyFromEntrypoint(ctx, ep)
	if err != nil {
		return nil, err
	}

	authInfo, found := fs.c.authInfos[bn.String()]
	if !found {
		return nil, ErrMissingWriterInfo
	}

	return writerInfoFromBlobNameKeyAndAuthInfo(bn, key, authInfo), nil
}

func (fs *cinodeFS) RootWriterInfo(ctx context.Context) (*WriterInfo, error) {
	rootEP, err := fs.RootEntrypoint()
	if err != nil {
		return nil, err
	}

	return fs.EntrypointWriterInfo(ctx, rootEP)
}
