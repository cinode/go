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

import "context"

// Entry is a file with its entrypoint
type nodeFile struct {
	ep Entrypoint
}

func (c *nodeFile) dirty() dirtyState {
	return dsClean
}

func (c *nodeFile) flush(ctx context.Context, gc *graphContext) (*Entrypoint, error) {
	return &c.ep, nil
}

func (c *nodeFile) traverse(
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

	// We're supposed to traverse into sub-path but it's not a directory
	return nil, 0, ErrNotADirectory
}

func (c *nodeFile) entrypoint() (*Entrypoint, error) {
	return &c.ep, nil
}
