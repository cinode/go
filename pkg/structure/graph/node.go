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

//
// cached entries:
//  * unloaded entry -  we only have entrypoint data
//  * directory - either clean (with existing entrypoint) or dirty (modified entries, not yet flushed)
//  * link - either clean (with tarted stored) or dirty (target changed but not yet flushed)
//  * file - entrypoint to static blob
//
// node states:
//  * if unloaded entry - contains entrypoint to the element, from entrypoint it can be deduced if this
//    is a dynamic link (from blob name) or directory (from mime type), this node does not need flushing
//  * node is dirty directly - the node was modified, its entrypoint can not be deduced before the node
//    is flushed, some modifications are kept in memory and can still be lost
//  * sub-nodes are dirty - the node itself is not dirty but some sub-nodes are. The node itself can have
//    entrypoint deduced because it will not change, but some sub-nodes will need flushing to persist the
//    data. Such situation is caused by dynamic links - the target can require flushing but the link itself
//    will preserve its entrypoint.
//

import (
	"context"
)

type dirtyState byte

const (
	// node and its sub-nodes are all clear, this sub-graph does not require flushing and is fully persisted
	dsClean dirtyState = 0

	// node is dirty, requires flushing to persist data
	dsDirty dirtyState = 1

	// node is itself clean, but some sub-nodes are dirty, flushing will be forwarded to sub-nodes
	dsSubDirty dirtyState = 2
)

// node is a base interface required by all cached entries
type node interface {
	// returns dirty state of this entrypoint
	dirty() dirtyState

	// flush this entrypoint
	flush(ctx context.Context, gc *graphContext) (node, *Entrypoint, error)

	// traverse node
	traverse(
		ctx context.Context,
		gc *graphContext,
		path []string,
		pathPosition int,
		linkDepth int,
		isWritable bool,
		opts traverseOptions,
		whenReached traverseGoalFunc,
	) (
		replacementNode node,
		state dirtyState,
		err error,
	)

	// get current entrypoint value, do not flush before, if node is not flushed
	// it must return appropriate error
	entrypoint() (*Entrypoint, error)
}
