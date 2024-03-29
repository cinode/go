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
)

type traverseGoalFunc func(
	ctx context.Context,
	reachedEntrypoint node,
	isWriteable bool,
) (
	replacementEntrypoint node,
	changeResult dirtyState,
	err error,
)

type traverseOptions struct {
	createNodes      bool
	doNotCache       bool
	maxLinkRedirects int
}

// Generic graph traversal function, it follows given path, once the endpoint
// is reached, it executed given callback function.
func (fs *cinodeFS) traverseGraph(
	ctx context.Context,
	path []string,
	opts traverseOptions,
	whenReached traverseGoalFunc,
) error {
	for _, p := range path {
		if p == "" {
			return ErrEmptyName
		}
	}

	opts.maxLinkRedirects = fs.maxLinkRedirects

	changedEntrypoint, _, err := fs.rootEP.traverse(
		ctx,         // context
		&fs.c,       // graph context
		path,        // path
		0,           // pathPosition - start at the beginning
		0,           // linkDepth - we don't come from any link
		true,        // isWritable - root is always writable
		opts,        // traverseOptions
		whenReached, // callback
	)
	if err != nil {
		return err
	}
	if !opts.doNotCache {
		fs.rootEP = changedEntrypoint
	}
	return nil
}
