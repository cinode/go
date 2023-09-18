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

package blenc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
)

var (
	ErrDynamicLinkUpdateFailed           = errors.New("could not prepare dynamic link update")
	ErrDynamicLinkUpdateFailedWriterInfo = fmt.Errorf("%w: invalid writer info", ErrDynamicLinkUpdateFailed)
	ErrDynamicLinkUpdateFailedWrongKey   = fmt.Errorf("%w: encryption key mismatch", ErrDynamicLinkUpdateFailed)
	ErrDynamicLinkUpdateFailedWrongName  = fmt.Errorf("%w: blob name mismatch", ErrDynamicLinkUpdateFailed)
)

func (be *beDatastore) openDynamicLink(
	ctx context.Context,
	name common.BlobName,
	key common.BlobKey,
) (
	io.ReadCloser,
	error,
) {

	// TODO: Protect against long links - there should be max size limit and maybe some streaming involved?

	rc, err := be.ds.Open(ctx, name)
	if err != nil {
		return nil, err
	}

	dl, err := dynamiclink.FromPublicData(name, rc)
	if err != nil {
		rc.Close()
		return nil, err
	}

	linkReader, err := dl.GetLinkDataReader(key)
	if err != nil {
		rc.Close()
		return nil, err
	}

	// Decrypt link data
	return struct {
		io.Reader
		io.Closer
	}{
		Reader: linkReader,
		Closer: rc,
	}, nil
}

func (be *beDatastore) createDynamicLink(
	ctx context.Context,
	r io.Reader,
) (
	common.BlobName,
	common.BlobKey,
	AuthInfo,
	error,
) {
	version := be.generateVersion()

	dl, err := dynamiclink.Create(be.rand)
	if err != nil {
		return nil, nil, nil, err
	}

	pr, encryptionKey, err := dl.UpdateLinkData(r, version)
	if err != nil {
		return nil, nil, nil, err
	}

	// Send update packet
	bn := dl.BlobName()
	err = be.ds.Update(ctx, bn, pr.GetPublicDataReader())
	if err != nil {
		return nil, nil, nil, err
	}

	return bn,
		encryptionKey,
		dl.AuthInfo(),
		nil
}

func (be *beDatastore) updateDynamicLink(
	ctx context.Context,
	name common.BlobName,
	authInfo AuthInfo,
	key common.BlobKey,
	r io.Reader,
) error {
	newVersion := be.generateVersion()

	dl, err := dynamiclink.FromAuthInfo(authInfo)
	if err != nil {
		return err
	}

	pr, encryptionKey, err := dl.UpdateLinkData(r, newVersion)
	if err != nil {
		return err
	}

	// Sanity checks
	if !bytes.Equal(encryptionKey, key) {
		return ErrDynamicLinkUpdateFailedWrongKey
	}
	if !bytes.Equal(name, dl.BlobName()) {
		return ErrDynamicLinkUpdateFailedWrongName
	}

	// Send update packet
	err = be.ds.Update(ctx, name, pr.GetPublicDataReader())
	if err != nil {
		return err
	}

	return nil
}
