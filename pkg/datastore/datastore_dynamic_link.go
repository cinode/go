/*
Copyright © 2022 Bartłomiej Święcki (byo)

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

package datastore

import (
	"context"
	"errors"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
)

func (ds *datastore) readDynamicLink(ctx context.Context, name common.BlobName, output io.Writer) error {
	rc, err := ds.s.openReadStream(ctx, name)
	if err != nil {
		return err
	}
	defer rc.Close()

	// Reading the link will validate if it is cryptographically correct,
	// the TeeReader will send the link data to the output while it's being validated
	_, err = dynamiclink.FromReader(name, io.TeeReader(rc, output))
	if err != nil {
		return err
	}

	return nil
}

func (ds *datastore) loadCurrentDynamicLink(ctx context.Context, name common.BlobName) (*dynamiclink.DynamicLinkData, error) {
	rc, err := ds.s.openReadStream(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	dl, err := dynamiclink.FromReader(name, rc)
	if err != nil {
		return nil, err
	}

	return dl, nil
}

func (ds *datastore) updateDynamicLink(ctx context.Context, name common.BlobName, updateStream io.Reader) error {
	ws, err := ds.s.openWriteStream(ctx, name)
	if err != nil {
		return err
	}
	defer ws.Cancel()

	// Parse and validate the link content, any invalid link will not pass,
	// The TeeReader will send the new link to the storage, however that update will not be
	// confirmed until the selection between two versions of the link is done
	updatedLink, err := dynamiclink.FromReader(name, io.TeeReader(updateStream, ws))
	if err != nil {
		return err
	}

	// Load existing link
	currentLink, err := ds.loadCurrentDynamicLink(ctx, name)
	if err != nil {
		return err
	}

	// Update the link if the new one turns out to be a better choice
	if currentLink == nil || updatedLink.GreaterThan(currentLink) {

		// Only now confirm the update - a successful close replaces
		// the current link with the updated one
		err = ws.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
