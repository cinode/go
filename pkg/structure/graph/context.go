package graph

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cinode/go/pkg/blenc"
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
) (common.BlobKey, error) {
	if ep.ep == nil ||
		ep.ep.KeyInfo == nil ||
		ep.ep.KeyInfo.Key == nil {
		return common.BlobKey{}, ErrMissingKeyInfo
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

	return entrypointFromBlobNameAndKey(bn, key), nil
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
