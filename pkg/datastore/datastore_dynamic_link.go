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

func (ds *datastore) openDynamicLink(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	rc, err := ds.s.openReadStream(ctx, name)
	if err != nil {
		return nil, err
	}

	dl, err := dynamiclink.FromPublicData(name, rc)
	if err != nil {
		rc.Close()
		return nil, err
	}

	return struct {
		io.Reader
		io.Closer
	}{
		Reader: dl.GetPublicDataReader(),
		Closer: rc,
	}, nil
}

// loadCurrentDynamicLink returns existing dynamic link data if present (nil if not found). That link is not meant to be
// read from - only for comparison
func (ds *datastore) newLinkGreaterThanCurrent(
	ctx context.Context,
	name common.BlobName,
	newLink *dynamiclink.PublicReader,
) (
	bool, error,
) {
	rc, err := ds.s.openReadStream(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer rc.Close()

	dl, err := dynamiclink.FromPublicData(name, rc)
	if err != nil {
		return false, err
	}

	return newLink.GreaterThan(dl), nil
}

func (ds *datastore) updateDynamicLink(ctx context.Context, name common.BlobName, updateStream io.Reader) error {
	ws, err := ds.s.openWriteStream(ctx, name)
	if err != nil {
		return err
	}
	defer ws.Cancel()

	// Start parsing the update link - it will raise an error if link can not be validated
	updatedLink, err := dynamiclink.FromPublicData(name, updateStream)
	if err != nil {
		return err
	}

	greater, err := ds.newLinkGreaterThanCurrent(ctx, name, updatedLink)
	if err != nil {
		return err
	}

	// Update the link if the new one turns out to be a better choice
	if greater {
		// Copy the data to local storage, this will also additionally
		// validate the data from the link
		_, err := io.Copy(ws, updatedLink.GetPublicDataReader())
		if err != nil {
			return err
		}

		// Only now confirm the update - a successful close replaces
		// the current link with the updated one
		err = ws.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
