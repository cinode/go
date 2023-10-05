package graph

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/structure/internal/protobuf"
)

const (
	CinodeDirMimeType = "application/cinode-dir"
)

var (
	// Returned when an entry does not exist in a directory
	ErrEntryNotFound = errors.New("entry not found")

	// Returned when there's a directory read error
	ErrCantReadDirectory = errors.New("can not read directory")

	// Returned when directory blob was read correctly but the data is corrupted
	ErrInvalidDirectoryData = errors.New("invalid directory data")

	ErrCantWriteDirectory = errors.New("can not write directory")
)

type Dir struct {
	entries map[string]*Entrypoint
}

func NewEmptyDir() *Dir {
	return &Dir{
		entries: map[string]*Entrypoint{},
	}
}

func LoadDir(ctx context.Context, ep *Entrypoint, c *graphContext) (*Dir, error) {
	dirMessage := protobuf.Directory{}

	err := c.readProtobufMessage(ctx, ep, &dirMessage)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDirectoryData, err)
	}

	dir := &Dir{
		entries: make(map[string]*Entrypoint, len(dirMessage.Entries)),
	}

	for _, entry := range dirMessage.Entries {
		if entry.Name == "" {
			return nil, fmt.Errorf("%w: entry with empty name", ErrInvalidDirectoryData)
		}
		ep, err := entrypointFromProtobuf(entry.Ep)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: invalid entrypoint for %s entry: %w",
				ErrInvalidDirectoryData, entry.Name, err,
			)
		}
		if _, found := dir.entries[entry.Name]; found {
			return nil, fmt.Errorf(
				"%w: invalid entry for %s: duplicate found",
				ErrInvalidDirectoryData,
				entry.Name,
			)
		}
		dir.entries[entry.Name] = ep
	}

	return dir, nil
}

func (d *Dir) FindEntry(n string) (*Entrypoint, error) {
	e, found := d.entries[n]
	if !found {
		return nil, ErrEntryNotFound
	}
	return e, nil
}

type DirEntriesFilter struct {
	NamePrefix string
	NameSuffix string
}

func (f *DirEntriesFilter) matches(name string, ep *Entrypoint) bool {
	if f == nil {
		return true
	}

	if !strings.HasPrefix(name, f.NamePrefix) {
		return false
	}
	if !strings.HasSuffix(name, f.NameSuffix) {
		return false
	}

	return true
}

type DirEntriesFunc func(name string, ep *Entrypoint)

func (d *Dir) EnumerateEntries(
	ctx context.Context,
	filter *DirEntriesFilter,
	callback DirEntriesFunc,
) error {
	for name, entry := range d.entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if filter.matches(name, entry) {
			callback(name, entry)
		}
	}
	return nil
}

func (d *Dir) SetEntry(n string, ep *Entrypoint) {
	d.entries[n] = ep
}

func (d *Dir) DeleteEntry(n string) error {
	if _, found := d.entries[n]; !found {
		return ErrEntryNotFound
	}
	delete(d.entries, n)
	return nil
}

func (d *Dir) Store(ctx context.Context, c *graphContext) (*Entrypoint, error) {
	dir := protobuf.Directory{
		Entries: make([]*protobuf.Directory_Entry, len(d.entries)),
	}

	for name, entry := range d.entries {
		dir.Entries = append(dir.Entries, &protobuf.Directory_Entry{
			Name: name,
			Ep:   entry.ep,
		})
	}

	sort.Slice(dir.Entries, func(i, j int) bool {
		return dir.Entries[i].Name < dir.Entries[j].Name
	})

	ep, err := c.createProtobufMessage(ctx, blobtypes.Static, &dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCantWriteDirectory, err)
	}

	return ep, nil
}
