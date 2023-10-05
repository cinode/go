package graph

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"sort"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/cinode/go/pkg/structure/internal/protobuf"
)

var (
	ErrInvalidBE          = errors.New("invalid BE argument")
	ErrCantOpenDir        = errors.New("can not open directory")
	ErrTooManyRedirects   = errors.New("too many link redirects")
	ErrCantComputeBlobKey = errors.New("can not compute blob keys")
	ErrModifiedDirectory  = errors.New("can not get entrypoint for a directory, unsaved content")
	ErrCantDeleteRoot     = errors.New("can not delete root object")
	ErrNotADirectory      = errors.New("entry is not a directory")
	ErrNilEntrypoint      = errors.New("nil entrypoint")
)

// Directory structure
type dirCache = map[string]*cachedEntrypoint

type linkCache struct {
	ep     *Entrypoint       // entrypoint of the link itself
	target *cachedEntrypoint // target for the link
}

// A single entry in directory cache, only one of entries below must be non-nil
type cachedEntrypoint struct {
	// target data is stored and we've got valid entrypoint to it
	stored *Entrypoint

	// Target is a link and contains modified data
	link *linkCache

	// Target is a directory containing partially modified content
	dir dirCache
}

type cinodeFS struct {
	c                graphContext
	maxLinkRedirects int
	timeFunc         func() time.Time
	randSource       io.Reader

	rootEP *cachedEntrypoint
}

type CinodeFS = *cinodeFS

func NewCinodeFS(
	ctx context.Context,
	be blenc.BE,
	options ...CinodeFSOption,
) (*cinodeFS, error) {
	if be == nil {
		return nil, ErrInvalidBE
	}

	ret := cinodeFS{
		maxLinkRedirects: DefaultMaxLinksRedirects,
		timeFunc:         time.Now,
		randSource:       rand.Reader,
		c: graphContext{
			be:          be,
			writerInfos: map[string][]byte{},
		},
	}

	for _, opt := range options {
		err := opt.apply(ctx, &ret)
		if err != nil {
			return nil, err
		}
	}

	return &ret, nil
}

func (fs *cinodeFS) SetEntryFile(
	ctx context.Context,
	path []string,
	data io.Reader,
	mimeType string,
) (*Entrypoint, error) {
	if mimeType == "" && len(path) > 0 {
		// Detect mime type by file extension
		mimeType = mime.TypeByExtension(filepath.Ext(path[len(path)-1]))
	}

	ep, err := fs.CreateFileEntrypoint(ctx, data, mimeType)
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
	mimeType string,
) (*Entrypoint, error) {
	var hw headWriter

	if mimeType == "" {
		hw = newHeadWriter(512)
		data = io.TeeReader(data, &hw)
	}

	bn, key, _, err := fs.c.be.Create(ctx, blobtypes.Static, data)
	if err != nil {
		return nil, err
	}

	if mimeType == "" {
		mimeType = http.DetectContentType(hw.data)
	}

	ep := entrypointFromBlobNameAndKey(bn, key)
	ep.ep.MimeType = mimeType
	return ep, nil
}

func (fs *cinodeFS) SetEntry(
	ctx context.Context,
	path []string,
	ep *Entrypoint,
) error {
	rootEP, err := fs.setEntry(ctx, fs.rootEP, path, ep, 0)
	if err != nil {
		return err
	}
	fs.rootEP = rootEP
	return nil
}

func (fs *cinodeFS) setEntry(
	ctx context.Context,
	current *cachedEntrypoint,
	path []string,
	ep *Entrypoint,
	linkDepth int,
) (*cachedEntrypoint, error) {
	if current == nil {
		// creating brand new path that does not exist yet
		if len(path) == 0 {
			return &cachedEntrypoint{stored: ep}, nil
		}
		// New empty directory
		current = &cachedEntrypoint{dir: map[string]*cachedEntrypoint{}}
	}

	// entry not yet loaded, we only know the entrypoint, load it then
	loaded, err := fs.loadEntrypoint(ctx, current)
	if err != nil {
		return nil, err
	}
	current = loaded

	if current.link != nil {
		if linkDepth >= fs.maxLinkRedirects {
			return nil, ErrTooManyRedirects
		}

		if _, hasWriterInfo := fs.c.writerInfos[current.link.ep.BlobName().String()]; !hasWriterInfo {
			// We won't be able to update data behind given link
			// TODO: This is false for recursive links, we only have to check this at the last level
			return nil, ErrMissingWriterInfo
		}

		// Update the target of the link
		target, err := fs.setEntry(ctx, current.link.target, path, ep, linkDepth+1)
		if err != nil {
			return nil, err
		}
		current.link.target = target
		return current, nil
	}

	if len(path) == 0 {
		// reached the final spot for the entrypoint, replace the current content
		// TODO: This could be a very destructive change, should we do additional checks here?
		//       e.g. if there's a directory here, prevent replacing with a file
		return &cachedEntrypoint{stored: ep}, nil
	}

	if current.dir == nil {
		// we need to have directory at this level
		current = &cachedEntrypoint{dir: map[string]*cachedEntrypoint{}}
	}

	if currentDirEntry, found := current.dir[path[0]]; found {
		// Overwrite existing entry including descending into sub-dirs
		updatedEntry, err := fs.setEntry(ctx, currentDirEntry, path[1:], ep, 0)
		if err != nil {
			return nil, err
		}
		current.dir[path[0]] = updatedEntry
		return current, nil
	}

	// No entry, create completely new path
	newEntry, err := fs.setEntry(ctx, nil, path[1:], ep, 0)
	if err != nil {
		return nil, err
	}
	current.dir[path[0]] = newEntry
	return current, nil
}

func (fs *cinodeFS) loadEntrypoint(
	ctx context.Context,
	ep *cachedEntrypoint,
) (
	*cachedEntrypoint,
	error,
) {
	if ep.stored != nil {
		// Data is behind some entrypoint, try to load it
		if ep.stored.IsLink() {
			return fs.loadEntrypointLink(ctx, ep.stored)
		}
		if ep.stored.IsDir() {
			return fs.loadEntrypointDir(ctx, ep.stored)
		}
	}

	return ep, nil
}

func (fs *cinodeFS) loadEntrypointLink(
	ctx context.Context,
	ep *Entrypoint,
) (
	*cachedEntrypoint,
	error,
) {
	msg := &protobuf.Entrypoint{}
	err := fs.c.readProtobufMessage(ctx, ep, msg)
	if err != nil {
		return nil, err
	}

	targetEP, err := entrypointFromProtobuf(msg)
	if err != nil {
		return nil, err
	}

	return &cachedEntrypoint{
		link: &linkCache{
			ep: ep,
			target: &cachedEntrypoint{
				stored: targetEP,
			},
		},
	}, nil
}

func (fs *cinodeFS) loadEntrypointDir(
	ctx context.Context,
	ep *Entrypoint,
) (
	*cachedEntrypoint,
	error,
) {
	msg := &protobuf.Directory{}
	err := fs.c.readProtobufMessage(ctx, ep, msg)
	if err != nil {
		return nil, err
	}

	dir := make(map[string]*cachedEntrypoint, len(msg.Entries))

	for _, entry := range msg.Entries {
		if entry.Name == "" {
			return nil, errors.New("empty name")
		}
		if _, exists := dir[entry.Name]; exists {
			return nil, errors.New("entry doubled")
		}

		ep, err := entrypointFromProtobuf(entry.Ep)
		if err != nil {
			return nil, err
		}

		dir[entry.Name] = &cachedEntrypoint{stored: ep}
	}

	return &cachedEntrypoint{dir: dir}, nil
}

func (fs *cinodeFS) Flush(ctx context.Context) error {
	newRoot, err := fs.flush(ctx, fs.rootEP)
	if err != nil {
		return err
	}
	fs.rootEP = &cachedEntrypoint{stored: newRoot}
	return nil
}

func (fs *cinodeFS) flush(ctx context.Context, current *cachedEntrypoint) (*Entrypoint, error) {
	if current.link != nil {
		return fs.flushLink(ctx, current)
	}
	if current.dir != nil {
		return fs.flushDir(ctx, current)
	}
	// already stored, no need to flush
	return current.stored, nil
}

func (fs *cinodeFS) flushLink(ctx context.Context, current *cachedEntrypoint) (*Entrypoint, error) {
	target, err := fs.flush(ctx, current.link.target)
	if err != nil {
		return nil, err
	}

	err = fs.c.updateProtobufMessage(ctx, current.link.ep, target.ep)
	if err != nil {
		return nil, err
	}

	return current.link.ep, nil
}

func (fs *cinodeFS) flushDir(ctx context.Context, current *cachedEntrypoint) (*Entrypoint, error) {
	dir := protobuf.Directory{
		Entries: make([]*protobuf.Directory_Entry, 0, len(current.dir)),
	}

	for name, entry := range current.dir {
		flushed, err := fs.flush(ctx, entry)
		if err != nil {
			return nil, err
		}

		dir.Entries = append(dir.Entries, &protobuf.Directory_Entry{
			Name: name,
			Ep:   flushed.ep,
		})
	}

	sort.Slice(dir.Entries, func(i, j int) bool {
		return dir.Entries[i].Name < dir.Entries[j].Name
	})

	ep, err := fs.c.createProtobufMessage(ctx, blobtypes.Static, &dir)
	if err != nil {
		return nil, err
	}
	ep.ep.MimeType = CinodeDirMimeType

	return ep, nil
}

func (fs *cinodeFS) FindEntry(ctx context.Context, path []string) (*Entrypoint, error) {
	return fs.findEntry(ctx, fs.rootEP, path, 0)
}

func (fs *cinodeFS) findEntry(
	ctx context.Context,
	current *cachedEntrypoint,
	path []string,
	linkDepth int,
) (*Entrypoint, error) {
	current, err := fs.loadEntrypoint(ctx, current)
	if err != nil {
		return nil, err
	}

	if current.link != nil {
		if linkDepth >= fs.maxLinkRedirects {
			return nil, ErrTooManyRedirects
		}
		return fs.findEntry(ctx, current.link.target, path, linkDepth+1)
	}

	if current.dir != nil {
		return fs.findEntryInDir(ctx, current.dir, path)
	}

	if len(path) > 0 {
		return nil, ErrNotADirectory
	}

	return current.stored, nil
}

func (fs *cinodeFS) findEntryInDir(ctx context.Context, dir dirCache, path []string) (*Entrypoint, error) {
	if len(path) == 0 {
		return nil, ErrModifiedDirectory
	}

	entry, found := dir[path[0]]
	if !found {
		return nil, ErrEntryNotFound
	}

	return fs.findEntry(ctx, entry, path[1:], 0)
}

func (fs *cinodeFS) DeleteEntry(ctx context.Context, path []string) error {
	// Entry removal is done on the parent level, we find the parent directory
	// and remove the entry from its list

	if len(path) == 0 {
		return ErrCantDeleteRoot
	}

	newRoot, err := fs.deleteEntry(
		ctx,
		fs.rootEP,
		path[:len(path)-1],
		path[len(path)-1],
		0,
	)
	if err != nil {
		return err
	}
	fs.rootEP = newRoot
	return nil
}

func (fs *cinodeFS) deleteEntry(
	ctx context.Context,
	current *cachedEntrypoint,
	path []string,
	entryName string,
	linkDepth int,
) (
	*cachedEntrypoint,
	error,
) {
	current, err := fs.loadEntrypoint(ctx, current)
	if err != nil {
		return nil, err
	}

	if current.link != nil {
		if linkDepth >= fs.maxLinkRedirects {
			return nil, ErrTooManyRedirects
		}

		if _, hasWriterInfo := fs.c.writerInfos[current.link.ep.BlobName().String()]; !hasWriterInfo {
			// We won't be able to update data behind given link
			// TODO: This is false for recursive links, we only have to check this at the last level
			return nil, ErrMissingWriterInfo
		}

		newTarget, err := fs.deleteEntry(
			ctx,
			current.link.target,
			path,
			entryName,
			linkDepth+1,
		)
		if err != nil {
			return nil, err
		}
		current.link.target = newTarget
		return current, nil
	}

	if current.dir == nil {
		return nil, ErrNotADirectory
	}

	if len(path) == 0 {
		// Got to the target directory, try to remove the entry
		if _, found := current.dir[entryName]; !found {
			return nil, ErrEntryNotFound
		}
		delete(current.dir, entryName)
		return current, nil
	}

	// Not yet at the target, descend to sub-directory
	subDir, found := current.dir[path[0]]
	if !found {
		return nil, ErrEntryNotFound
	}

	subDir, err = fs.deleteEntry(ctx, subDir, path[1:], entryName, 0)
	if err != nil {
		return nil, err
	}

	current.dir[path[0]] = subDir
	return current, nil
}

func (fs *cinodeFS) generateNewDynamicLinkEntrypoint() (*Entrypoint, error) {
	// Generate new entrypoint link data but do not yet store it in datastore
	link, err := dynamiclink.Create(fs.randSource)
	if err != nil {
		return nil, err
	}

	bn := link.BlobName()
	key := link.EncryptionKey()

	fs.c.writerInfos[bn.String()] = link.AuthInfo()

	return entrypointFromBlobNameAndKey(bn, key), nil
}

func (fs *cinodeFS) OpenEntrypointData(ctx context.Context, ep *Entrypoint) (io.ReadCloser, error) {
	if ep == nil {
		return nil, ErrNilEntrypoint
	}

	return fs.c.getDataReader(ctx, ep)
}
