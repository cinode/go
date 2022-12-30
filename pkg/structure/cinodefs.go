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

package structure

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/protobuf"
	"google.golang.org/protobuf/proto"
)

type CinodeFS struct {
	BE               blenc.BE
	RootEntrypoint   *protobuf.Entrypoint
	MaxLinkRedirects int
	IndexFile        string
}

func (d *CinodeFS) FetchContent(ctx context.Context, ep *protobuf.Entrypoint, output io.Writer) error {
	return d.BE.Read(
		ctx,
		common.BlobName(ep.BlobName),
		blenc.EncryptionKey(ep.GetKeyInfo().GetKey()),
		output,
	)
}

func (d *CinodeFS) FindEntrypoint(ctx context.Context, path string) (*protobuf.Entrypoint, error) {
	return d.findEntrypointInDir(ctx, d.RootEntrypoint, path)
}

func (d *CinodeFS) findEntrypointInDir(ctx context.Context, ep *protobuf.Entrypoint, remainingPath string) (*protobuf.Entrypoint, error) {
	ep, err := DereferenceLink(ctx, d.BE, ep, d.MaxLinkRedirects)
	if err != nil {
		return nil, err
	}

	if ep.MimeType != CinodeDirMimeType {
		return nil, ErrNotADirectory
	}

	if remainingPath == "" {
		remainingPath = d.IndexFile
	}

	buff := bytes.NewBuffer(nil)
	err = d.FetchContent(ctx, ep, buff)
	if err != nil {
		return nil, err
	}

	dirStruct := protobuf.Directory{}
	err = proto.Unmarshal(buff.Bytes(), &dirStruct)
	if err != nil {
		return nil, err
	}

	pathParts := strings.SplitN(remainingPath, "/", 2)
	entry, exists := dirStruct.GetEntries()[pathParts[0]]
	if !exists {
		return nil, ErrNotFound
	}

	if len(pathParts) == 1 {
		// Found the entry, no need to descend any further, only dereference the link
		entry, err = DereferenceLink(ctx, d.BE, entry, d.MaxLinkRedirects)
		if err != nil {
			return nil, err
		}
		return entry, nil
	}

	return d.findEntrypointInDir(ctx, entry, pathParts[1])
}
