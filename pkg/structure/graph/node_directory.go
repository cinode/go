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
	"sort"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/structure/internal/protobuf"
	"github.com/cinode/go/pkg/utilities/golang"
)

// directoryNode holds a directory entry loaded into memory
type directoryNode struct {
	entries map[string]node
	stored  *Entrypoint // current entrypoint, will be nil if directory was modified
	dState  dirtyState  // true if any subtree is dirty
}

func (d *directoryNode) dirty() dirtyState {
	return d.dState
}

func (d *directoryNode) flush(ctx context.Context, gc *graphContext) (node, *Entrypoint, error) {
	if d.dState == dsClean {
		// all clear, nothing to flush here or in sub-trees
		return d, d.stored, nil
	}

	if d.dState == dsSubDirty {
		// Some sub-nodes are dirty, need to propagate flush to
		flushedEntries := make(map[string]node, len(d.entries))
		for name, entry := range d.entries {
			target, _, err := entry.flush(ctx, gc)
			if err != nil {
				return nil, nil, err
			}

			flushedEntries[name] = target
		}

		// directory itself was not modified and does not need flush, don't bother
		// saving it to datastore
		return &directoryNode{
			entries: flushedEntries,
			stored:  d.stored,
			dState:  dsClean,
		}, d.stored, nil
	}

	golang.Assert(d.dState == dsDirty, "ensure correct dirtiness state")

	// Directory has changed, have to recalculate its blob and save it in data store
	dir := protobuf.Directory{
		Entries: make([]*protobuf.Directory_Entry, 0, len(d.entries)),
	}
	flushedEntries := make(map[string]node, len(d.entries))
	for name, entry := range d.entries {
		target, targetEP, err := entry.flush(ctx, gc)
		if err != nil {
			return nil, nil, err
		}

		flushedEntries[name] = target
		dir.Entries = append(dir.Entries, &protobuf.Directory_Entry{
			Name: name,
			Ep:   targetEP.ep,
		})
	}

	// Sort by name - that way we gain deterministic order during
	// serialization od the directory
	sort.Slice(dir.Entries, func(i, j int) bool {
		return dir.Entries[i].Name < dir.Entries[j].Name
	})

	ep, err := gc.createProtobufMessage(ctx, blobtypes.Static, &dir)
	if err != nil {
		return nil, nil, err
	}
	ep.ep.MimeType = CinodeDirMimeType

	return &directoryNode{
		entries: flushedEntries,
		stored:  ep,
		dState:  dsClean,
	}, ep, nil
}

func (c *directoryNode) traverse(
	ctx context.Context,
	gc *graphContext,
	path []string,
	pathPosition int,
	linkDepth int,
	isWritable bool,
	opts traverseOptions,
	whenReached traverseGoalFunc,
) (
	node,
	dirtyState,
	error,
) {
	if pathPosition == len(path) {
		return whenReached(ctx, c, isWritable)
	}

	subNode, found := c.entries[path[pathPosition]]
	if !found {
		if !opts.createNodes {
			return nil, 0, ErrEntryNotFound
		}
		if !isWritable {
			return nil, 0, ErrMissingWriterInfo
		}
		// create new sub-path
		newNode, err := c.traverseRecursiveNewPath(
			ctx,
			path,
			pathPosition+1,
			opts,
			whenReached,
		)
		if err != nil {
			return nil, 0, err
		}
		c.entries[path[pathPosition]] = newNode
		c.dState = dsDirty
		return c, dsDirty, nil
	}

	// found path entry, descend to sub-node
	replacement, replacementState, err := subNode.traverse(
		ctx,
		gc,
		path,
		pathPosition+1,
		0,
		isWritable,
		opts,
		whenReached,
	)
	if err != nil {
		return nil, 0, err
	}
	if opts.doNotCache {
		return c, dsClean, nil
	}

	c.entries[path[pathPosition]] = replacement
	if replacementState == dsDirty {
		// child is dirty, this propagates down to the current node
		c.dState = dsDirty
		return c, dsDirty, nil
	}

	if replacementState == dsSubDirty {
		// child itself is not dirty, but some sub-node is, sub-dirtiness
		// propagates to the current node, but if the directory is
		// already directly dirty (stronger dirtiness), keep it as it is
		if c.dState != dsDirty {
			c.dState = dsSubDirty
		}
		return c, dsSubDirty, nil
	}

	golang.Assert(replacementState == dsClean, "ensure correct dirtiness state")
	// leave current state as it is
	return c, dsClean, nil

}

func (c *directoryNode) traverseRecursiveNewPath(
	ctx context.Context,
	path []string,
	pathPosition int,
	opts traverseOptions,
	whenReached traverseGoalFunc,
) (
	node,
	error,
) {
	if len(path) == pathPosition {
		replacement, _, err := whenReached(ctx, nil, true)
		return replacement, err
	}

	sub, err := c.traverseRecursiveNewPath(
		ctx,
		path,
		pathPosition+1,
		opts,
		whenReached,
	)
	if err != nil {
		return nil, err
	}

	return &directoryNode{
		entries: map[string]node{
			path[pathPosition]: sub,
		},
		dState: dsDirty,
	}, nil
}

func (c *directoryNode) entrypoint() (*Entrypoint, error) {
	if c.dState == dsDirty {
		return nil, ErrModifiedDirectory
	}

	golang.Assert(
		c.dState == dsClean || c.dState == dsSubDirty,
		"ensure dirtiness state is valid",
	)

	return c.stored, nil
}

func (c *directoryNode) deleteEntry(name string) bool {
	if _, hasEntry := c.entries[name]; !hasEntry {
		return false
	}
	delete(c.entries, name)
	c.dState = dsDirty
	return true
}
