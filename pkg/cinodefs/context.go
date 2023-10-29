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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs/internal/protobuf"
	"github.com/cinode/go/pkg/common"
	"google.golang.org/protobuf/proto"
)

var (
	ErrMissingKeyInfo    = errors.New("missing key info")
	ErrMissingWriterInfo = errors.New("missing writer info")
)

type graphContext struct {
	// blenc layer used in the graph
	be blenc.BE

	// known writer info data
	writerInfos map[string][]byte
}

// Get symmetric encryption key for given entrypoint.
//
// Note: Currently the key will be stored inside entrypoint data,
// but more advanced methods of obtaining the key may be added
// through this function in the future.
func (c *graphContext) keyFromEntrypoint(
	ctx context.Context,
	ep *Entrypoint,
) (*common.BlobKey, error) {
	if ep.ep.KeyInfo == nil ||
		ep.ep.KeyInfo.Key == nil {
		return nil, ErrMissingKeyInfo
	}
	return common.BlobKeyFromBytes(ep.ep.GetKeyInfo().GetKey()), nil
}

// open io.ReadCloser for data behind given entrypoint
func (c *graphContext) getDataReader(
	ctx context.Context,
	ep *Entrypoint,
) (
	io.ReadCloser,
	error,
) {
	key, err := c.keyFromEntrypoint(ctx, ep)
	if err != nil {
		return nil, err
	}
	rc, err := c.be.Open(ctx, ep.BlobName(), key)
	if err != nil {
		return nil, fmt.Errorf("failed to open blob: %w", err)
	}
	return rc, nil
}

// return data behind entrypoint
func (c *graphContext) readProtobufMessage(
	ctx context.Context,
	ep *Entrypoint,
	msg proto.Message,
) error {
	rc, err := c.getDataReader(ctx, ep)
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read blob: %w", err)
	}

	err = proto.Unmarshal(data, msg)
	if err != nil {
		return fmt.Errorf("malformed data: %w", err)
	}

	return nil
}

func (c *graphContext) createProtobufMessage(
	ctx context.Context,
	blobType common.BlobType,
	msg proto.Message,
) (
	*Entrypoint,
	error,
) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("serialization failed: %w", err)
	}

	bn, key, wi, err := c.be.Create(ctx, blobType, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("write failed: %w", err)
	}

	if wi != nil {
		c.writerInfos[bn.String()] = wi
	}

	return &Entrypoint{
		bn: bn,
		ep: protobuf.Entrypoint{
			BlobName: bn.Bytes(),
			KeyInfo: &protobuf.KeyInfo{
				Key: key.Bytes(),
			},
		},
	}, nil
}

func (c *graphContext) updateProtobufMessage(
	ctx context.Context,
	ep *Entrypoint,
	msg proto.Message,
) error {
	wi, found := c.writerInfos[ep.BlobName().String()]
	if !found {
		return ErrMissingWriterInfo
	}

	key, err := c.keyFromEntrypoint(ctx, ep)
	if err != nil {
		return err
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	err = c.be.Update(ctx, ep.BlobName(), wi, key, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}
