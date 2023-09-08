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
	"context"
	"io"
	"strings"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/utilities/cipherfactory"
	"github.com/cinode/go/pkg/protobuf"
	"google.golang.org/protobuf/proto"
)

type CinodeFS struct {
	BE               blenc.BE
	RootEntrypoint   *protobuf.Entrypoint
	MaxLinkRedirects int
	CurrentTimeF     func() time.Time
}

func (d *CinodeFS) OpenContent(ctx context.Context, ep *protobuf.Entrypoint) (io.ReadCloser, error) {
	return d.BE.Open(
		ctx,
		common.BlobName(ep.BlobName),
		cipherfactory.Key(ep.GetKeyInfo().GetKey()),
	)
}

func (d *CinodeFS) FindEntrypoint(ctx context.Context, path string) (*protobuf.Entrypoint, error) {
	return d.findEntrypointInDir(ctx, d.RootEntrypoint, path, d.currentTime())
}

func (d *CinodeFS) findEntrypointInDir(
	ctx context.Context,
	ep *protobuf.Entrypoint,
	remainingPath string,
	currentTime time.Time,
) (
	*protobuf.Entrypoint,
	error,
) {
	ep, err := DereferenceLink(ctx, d.BE, ep, d.MaxLinkRedirects, currentTime)
	if err != nil {
		return nil, err
	}

	if ep.MimeType != CinodeDirMimeType {
		return nil, ErrNotADirectory
	}

	rc, err := d.OpenContent(ctx, ep)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	dirStruct := protobuf.Directory{}
	err = proto.Unmarshal(data, &dirStruct)
	if err != nil {
		return nil, err
	}

	pathParts := strings.SplitN(remainingPath, "/", 2)
	entryName := pathParts[0]
	var entry *protobuf.Entrypoint
	var exists bool
	for _, dirEntry := range dirStruct.GetEntries() {
		if entryName != dirEntry.GetName() {
			continue
		}
		if exists {
			// Doubled entry - reject such directory structure
			// to avoid ambiguity-based attacks
			return nil, ErrCorruptedLinkData
		}
		exists = true
		entry = dirEntry.GetEp()
	}
	if !exists {
		return nil, ErrNotFound
	}

	if len(pathParts) == 1 {
		// Found the entry, no need to descend any further, only dereference the link
		entry, err = DereferenceLink(ctx, d.BE, entry, d.MaxLinkRedirects, currentTime)
		if err != nil {
			return nil, err
		}
		return entry, nil
	}

	return d.findEntrypointInDir(ctx, entry, pathParts[1], currentTime)
}

func (d *CinodeFS) currentTime() time.Time {
	if d.CurrentTimeF != nil {
		return d.CurrentTimeF()
	}
	return time.Now()
}
