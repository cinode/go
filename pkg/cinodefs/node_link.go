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

	"github.com/cinode/go/pkg/utilities/golang"
)

// Entry is a link loaded into memory
type nodeLink struct {
	ep     *Entrypoint // entrypoint of the link itself
	target node        // target for the link
	dState dirtyState
}

func (c *nodeLink) dirty() dirtyState {
	return c.dState
}

func (c *nodeLink) flush(ctx context.Context, gc *graphContext) (node, *Entrypoint, error) {
	if c.dState == dsClean {
		// all clear
		return c, c.ep, nil
	}

	golang.Assert(c.dState == dsSubDirty, "link can be clean or sub-dirty")
	target, targetEP, err := c.target.flush(ctx, gc)
	if err != nil {
		return nil, nil, err
	}

	err = gc.updateProtobufMessage(ctx, c.ep, &targetEP.ep)
	if err != nil {
		return nil, nil, err
	}

	ret := &nodeLink{
		ep:     c.ep,
		target: target,
		dState: dsClean,
	}

	return ret, ret.ep, nil
}

func (c *nodeLink) traverse(
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
	if linkDepth >= opts.maxLinkRedirects {
		return nil, 0, ErrTooManyRedirects
	}

	// Note: we don't stop here even if we've reached the end of
	// traverse path, delegate traversal to target node instead

	// crossing link border, whether sub-graph is writeable is determined
	// by availability of corresponding writer info
	_, hasAuthInfo := gc.authInfos[c.ep.bn.String()]

	newTarget, targetState, err := c.target.traverse(
		ctx,
		gc,
		path,
		pathPosition,
		linkDepth+1,
		hasAuthInfo,
		opts,
		whenReached,
	)
	if err != nil {
		return nil, 0, err
	}

	if opts.doNotCache {
		return c, dsClean, nil
	}

	c.target = newTarget
	if targetState == dsClean {
		// Nothing to do
		//
		// Note: this path will happen once we keep clean nodes
		//       in the memory for caching purposes
		return c, dsClean, nil
	}

	golang.Assert(
		targetState == dsDirty || targetState == dsSubDirty,
		"ensure correct dirtiness state",
	)

	// sub-dirty propagates normally, dirty becomes sub-dirty
	// because link's entrypoint never changes
	c.dState = dsSubDirty
	return c, dsSubDirty, nil
}

func (c *nodeLink) entrypoint() (*Entrypoint, error) {
	return c.ep, nil
}
