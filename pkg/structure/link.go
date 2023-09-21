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

package structure

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/structure/internal/protobuf"
)

var (
	ErrMaxRedirectsReached    = errors.New("maximum limit of dynamic link redirects reached")
	ErrCorruptedLinkData      = errors.New("corrupted link data")
	ErrCorruptedDirectoryData = errors.New("corrupted directory data")
	ErrInvalidEntrypoint      = protobuf.ErrInvalidEntrypoint
	ErrInvalidEntrypointTime  = protobuf.ErrInvalidEntrypointTime
)

func CreateLink(ctx context.Context, be blenc.BE, ep *protobuf.Entrypoint) (*protobuf.Entrypoint, *protobuf.WriterInfo, error) {
	epBytes, err := ep.ToBytes()
	if err != nil {
		return nil, nil, err
	}

	name, key, authInfo, err := be.Create(ctx, blobtypes.DynamicLink, bytes.NewReader(epBytes))
	if err != nil {
		return nil, nil, err
	}

	return &protobuf.Entrypoint{
			BlobName: name.Bytes(),
			KeyInfo: &protobuf.KeyInfo{
				Key: key.Bytes(),
			},
		}, &protobuf.WriterInfo{
			BlobName: name.Bytes(),
			Key:      key.Bytes(),
			AuthInfo: authInfo,
		}, nil
}

func UpdateLink(ctx context.Context, be blenc.BE, wi *protobuf.WriterInfo, ep *protobuf.Entrypoint) (*protobuf.Entrypoint, error) {
	epBytes, err := ep.ToBytes()
	if err != nil {
		return nil, err
	}

	bn, err := common.BlobNameFromBytes(wi.BlobName)
	if err != nil {
		return nil, err
	}

	err = be.Update(
		ctx,
		bn,
		wi.AuthInfo,
		common.BlobKeyFromBytes(wi.Key),
		bytes.NewReader(epBytes),
	)
	if err != nil {
		return nil, err
	}

	return &protobuf.Entrypoint{
		BlobName: wi.BlobName,
		KeyInfo: &protobuf.KeyInfo{
			Key: wi.Key,
		},
	}, nil
}

func DereferenceLink(
	ctx context.Context,
	be blenc.BE,
	link *protobuf.Entrypoint,
	maxRedirects int,
	currentTime time.Time,
) (
	*protobuf.Entrypoint,
	error,
) {
	bn, key, err := link.ValidateAndParse(currentTime)
	if err != nil {
		return nil, err
	}

	for bn.Type() == blobtypes.DynamicLink {
		if maxRedirects == 0 {
			return nil, ErrMaxRedirectsReached
		}
		maxRedirects--

		rc, err := be.Open(ctx, bn, key)
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		// TODO: Constrain the buffer size
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}

		link, err = protobuf.EntryPointFromBytes(data)
		if err != nil {
			return nil, err
		}

		bn, key, err = link.ValidateAndParse(time.Now())
		if err != nil {
			return nil, err
		}
	}

	return link, nil
}
