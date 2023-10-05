package graph

import (
	"context"
	"fmt"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/structure/internal/protobuf"
)

type Link struct {
	ep  *Entrypoint
	tep *Entrypoint
}

func NewLink(
	ctx context.Context,
	targetEP *Entrypoint,
	c *graphContext,
) (*Link, error) {
	ep, err := c.createProtobufMessage(ctx, blobtypes.DynamicLink, targetEP.ep)
	if err != nil {
		return nil, err
	}

	return &Link{
		ep:  ep,
		tep: targetEP,
	}, nil
}

func OpenLink(
	ctx context.Context,
	ep *Entrypoint,
	c *graphContext,
) (*Link, error) {
	tepRaw := protobuf.Entrypoint{}

	err := c.readProtobufMessage(ctx, ep, &tepRaw)
	if err != nil {
		return nil, err
	}

	tep, err := entrypointFromProtobuf(&tepRaw)
	if err != nil {
		return nil, err
	}

	return &Link{
		ep:  ep,
		tep: tep,
	}, nil
}

func (l *Link) Update(ctx context.Context, tep *Entrypoint, c *graphContext) error {
	err := c.updateProtobufMessage(
		ctx,
		l.ep,
		tep.ep,
	)
	if err != nil {
		return fmt.Errorf("link update failed: %w", err)
	}
	l.tep = tep
	return nil
}
