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
	"fmt"

	"github.com/cinode/go/pkg/cinodefs/internal/protobuf"
)

type nodeUnloaded struct {
	ep *Entrypoint
}

func (c *nodeUnloaded) dirty() dirtyState {
	return dsClean
}

func (c *nodeUnloaded) flush(ctx context.Context, gc *graphContext) (node, *Entrypoint, error) {
	return c, c.ep, nil
}

func (c *nodeUnloaded) traverse(
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
	loaded, err := c.load(ctx, gc)
	if err != nil {
		return nil, 0, err
	}

	return loaded.traverse(
		ctx,
		gc,
		path,
		pathPosition,
		linkDepth,
		isWritable,
		opts,
		whenReached,
	)
}

func (c *nodeUnloaded) load(ctx context.Context, gc *graphContext) (node, error) {
	// Data is behind some entrypoint, try to load it
	if c.ep.IsLink() {
		return c.loadEntrypointLink(ctx, gc)
	}

	if c.ep.IsDir() {
		return c.loadEntrypointDir(ctx, gc)
	}

	return &nodeFile{ep: c.ep}, nil
}

func (c *nodeUnloaded) loadEntrypointLink(ctx context.Context, gc *graphContext) (node, error) {
	targetEP := &Entrypoint{}
	err := gc.readProtobufMessage(ctx, c.ep, &targetEP.ep)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCantOpenLink, err)
	}

	err = expandEntrypointProto(targetEP)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCantOpenLink, err)
	}

	return &nodeLink{
		ep:     c.ep,
		target: &nodeUnloaded{ep: targetEP},
		dState: dsClean,
	}, nil
}

func (c *nodeUnloaded) loadEntrypointDir(ctx context.Context, gc *graphContext) (node, error) {
	msg := &protobuf.Directory{}
	err := gc.readProtobufMessage(ctx, c.ep, msg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCantOpenDir, err)
	}

	dir := make(map[string]node, len(msg.Entries))

	for _, entry := range msg.Entries {
		if entry.Name == "" {
			return nil, fmt.Errorf("%w: %w", ErrCantOpenDir, ErrEmptyName)
		}
		if _, exists := dir[entry.Name]; exists {
			return nil, fmt.Errorf("%w: %s", ErrCantOpenDirDuplicateEntry, entry.Name)
		}

		ep, err := entrypointFromProtobuf(entry.Ep)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCantOpenDir, err)
		}

		dir[entry.Name] = &nodeUnloaded{ep: ep}
	}

	return &nodeDirectory{
		stored:  c.ep,
		entries: dir,
		dState:  dsClean,
	}, nil
}

func (c *nodeUnloaded) entrypoint() (*Entrypoint, error) {
	return c.ep, nil
}
