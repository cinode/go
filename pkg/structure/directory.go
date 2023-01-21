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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	CinodeDirMimeType = "application/cinode-dir"
)

var (
	ErrNotFound      = blenc.ErrNotFound
	ErrNotADirectory = errors.New("entry is not a directory")
	ErrNotAFile      = errors.New("entry is not a file")
)

func UploadStaticDirectory(ctx context.Context, fsys fs.FS, be blenc.BE) (*protobuf.Entrypoint, error) {
	c := dirCompiler{
		ctx:  ctx,
		fsys: fsys,
		be:   be,
	}

	return c.compilePath(".")
}

type headWriter struct {
	limit int
	data  []byte
}

func newHeadWriter(limit int) headWriter {
	return headWriter{
		limit: limit,
		data:  make([]byte, limit),
	}
}

func (h *headWriter) Write(b []byte) (int, error) {
	if len(h.data) >= h.limit {
		return len(b), nil
	}

	if len(h.data)+len(b) > h.limit {
		h.data = append(h.data, b[:h.limit-len(h.data)]...)
		return len(b), nil
	}

	h.data = append(h.data, b...)
	return len(b), nil
}

type dirCompiler struct {
	ctx  context.Context
	fsys fs.FS
	be   blenc.BE
}

func (d *dirCompiler) compilePath(path string) (*protobuf.Entrypoint, error) {
	st, err := fs.Stat(d.fsys, path)
	if err != nil {
		return nil, fmt.Errorf("couldn't check path: %w", err)
	}

	if st.IsDir() {
		return d.compileDir(path)
	}

	if st.Mode().IsRegular() {
		return d.compileFile(path)
	}

	return nil, fmt.Errorf("neither dir nor a regular file: %v", path)
}

func (d *dirCompiler) compileFile(path string) (*protobuf.Entrypoint, error) {
	// fmt.Println(" *", path)
	fl, err := d.fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't read file %v: %w", path, err)
	}
	defer fl.Close()

	// Use the dataHead to store first 512 bytes of data into a buffer while uploading it to the blenc layer
	// This buffer may then be used to detect the mime type
	dataHead := newHeadWriter(512)

	bn, ki, _, err := d.be.Create(context.Background(), blobtypes.Static, io.TeeReader(fl, &dataHead))
	if err != nil {
		return nil, err
	}

	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		mimeType = http.DetectContentType(dataHead.data)
	}

	return &protobuf.Entrypoint{
		BlobName: bn,
		KeyInfo:  &protobuf.KeyInfo{Key: ki},
		MimeType: mimeType,
	}, nil
}

func (d *dirCompiler) compileDir(p string) (*protobuf.Entrypoint, error) {
	fileList, err := fs.ReadDir(d.fsys, p)
	if err != nil {
		return nil, fmt.Errorf("couldn't read contents of dir %v: %w", p, err)
	}

	dirStruct := protobuf.Directory{
		Entries: make(map[string]*protobuf.Entrypoint),
	}
	for _, e := range fileList {
		subPath := path.Join(p, e.Name())

		ep, err := d.compilePath(subPath)
		if err != nil {
			return nil, err
		}

		dirStruct.Entries[e.Name()] = ep
	}

	data, err := proto.Marshal(&dirStruct)
	if err != nil {
		return nil, fmt.Errorf("can not serialize directory %v: %w", p, err)
	}

	bn, ki, _, err := d.be.Create(context.Background(), blobtypes.Static, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return &protobuf.Entrypoint{
		BlobName: bn,
		KeyInfo:  &protobuf.KeyInfo{Key: ki},
		MimeType: CinodeDirMimeType,
	}, nil
}
