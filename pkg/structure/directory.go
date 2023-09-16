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
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"sort"

	_ "embed"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/protobuf"
	"github.com/cinode/go/pkg/utilities/golang"
	"golang.org/x/exp/slog"
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

func UploadStaticDirectory(ctx context.Context, log *slog.Logger, fsys fs.FS, be blenc.BE) (*protobuf.Entrypoint, error) {
	c := dirCompiler{
		ctx:  ctx,
		fsys: fsys,
		be:   be,
		log:  log,
	}

	return c.compilePath(ctx, ".")
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
	log  *slog.Logger
}

func (d *dirCompiler) compilePath(ctx context.Context, path string) (*protobuf.Entrypoint, error) {
	st, err := fs.Stat(d.fsys, path)
	if err != nil {
		d.log.DebugContext(ctx, "failed to stat path", "path", path, "err", err)
		return nil, fmt.Errorf("couldn't check path: %w", err)
	}

	if st.IsDir() {
		return d.compileDir(ctx, path)
	}

	if st.Mode().IsRegular() {
		return d.compileFile(ctx, path)
	}

	d.log.ErrorContext(ctx, "path is neither dir nor a regular file", "path", path)
	return nil, fmt.Errorf("neither dir nor a regular file: %v", path)
}

// UploadStaticBlob uploads blob to the associated datastore and returns entrypoint to that file
//
// if mimeType is an empty string, it will be guessed from the content defaulting to
func UploadStaticBlob(ctx context.Context, be blenc.BE, r io.Reader, mimeType string, log *slog.Logger) (*protobuf.Entrypoint, error) {
	// Use the dataHead to store first 512 bytes of data into a buffer while uploading it to the blenc layer
	// This buffer may then be used to detect the mime type
	dataHead := newHeadWriter(512)

	bn, ki, _, err := be.Create(context.Background(), blobtypes.Static, io.TeeReader(r, &dataHead))
	if err != nil {
		log.ErrorContext(ctx, "failed to upload static file", "err", err)
		return nil, err
	}

	log.DebugContext(ctx, "static file uploaded successfully")

	if mimeType == "" {
		mimeType = http.DetectContentType(dataHead.data)
		log.DebugContext(ctx, "automatically detected content type", "contentType", mimeType)
	}

	return &protobuf.Entrypoint{
		BlobName: bn,
		KeyInfo:  &protobuf.KeyInfo{Key: ki},
		MimeType: mimeType,
	}, nil
}

func (d *dirCompiler) compileFile(ctx context.Context, path string) (*protobuf.Entrypoint, error) {
	d.log.InfoContext(ctx, "compiling file", "path", path)
	fl, err := d.fsys.Open(path)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to open file", "path", path, "err", err)
		return nil, fmt.Errorf("couldn't open file %v: %w", path, err)
	}
	defer fl.Close()

	ep, err := UploadStaticBlob(
		ctx,
		d.be,
		fl,
		mime.TypeByExtension(filepath.Ext(path)),
		d.log.With("path", path),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file %v: %w", path, err)
	}

	return ep, nil
}

func (d *dirCompiler) compileDir(ctx context.Context, p string) (*protobuf.Entrypoint, error) {
	fileList, err := fs.ReadDir(d.fsys, p)
	if err != nil {
		d.log.ErrorContext(ctx, "couldn't read contents of dir", "path", p, "err", err)
		return nil, fmt.Errorf("couldn't read contents of dir %v: %w", p, err)
	}

	dir := StaticDir{}
	for _, e := range fileList {
		subPath := path.Join(p, e.Name())

		ep, err := d.compilePath(ctx, subPath)
		if err != nil {
			return nil, err
		}

		dir.SetEntry(e.Name(), ep)
	}

	ep, err := dir.GenerateEntrypoint(context.Background(), d.be)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to serialize directory", "path", p, "err", err)
		return nil, fmt.Errorf("can not serialize directory %v: %w", p, err)
	}

	d.log.DebugContext(ctx,
		"directory uploaded successfully", "path", p,
		"blobName", common.BlobName(ep.BlobName).String(),
	)
	return ep, nil
}

type StaticDir struct {
	entries map[string]*protobuf.Entrypoint
}

func (s *StaticDir) SetEntry(name string, ep *protobuf.Entrypoint) {
	if s.entries == nil {
		s.entries = map[string]*protobuf.Entrypoint{}
	}
	s.entries[name] = ep
}

//go:embed templates/dir.html
var _dirIndexTemplateStr string
var dirIndexTemplate = golang.Must(
	template.New("dir").
		Funcs(template.FuncMap{
			"isDir": func(entry *protobuf.Entrypoint) bool {
				return entry.MimeType == CinodeDirMimeType
			},
		}).
		Parse(_dirIndexTemplateStr),
)

func (s *StaticDir) GenerateIndex(ctx context.Context, log *slog.Logger, indexName string, be blenc.BE) error {
	buf := bytes.NewBuffer(nil)
	err := dirIndexTemplate.Execute(buf, map[string]any{
		"entries":   s.getProtobufData().GetEntries(),
		"indexName": indexName,
	})
	if err != nil {
		return err
	}

	ep, err := UploadStaticBlob(ctx, be, bytes.NewReader(buf.Bytes()), "text/html", log)
	if err != nil {
		return err
	}

	s.entries[indexName] = ep
	return nil
}

func (s *StaticDir) getProtobufData() *protobuf.Directory {
	// Convert to protobuf format
	protoData := protobuf.Directory{
		Entries: make([]*protobuf.Directory_Entry, 0, len(s.entries)),
	}
	for name, ep := range s.entries {
		protoData.Entries = append(protoData.Entries, &protobuf.Directory_Entry{
			Name: name,
			Ep:   ep,
		})
	}

	// Sort by name
	sort.Slice(protoData.Entries, func(i, j int) bool {
		return protoData.Entries[i].Name < protoData.Entries[j].Name
	})

	return &protoData
}

func (s *StaticDir) GenerateEntrypoint(ctx context.Context, be blenc.BE) (*protobuf.Entrypoint, error) {
	// TODO: Introduce various directory split strategies
	data, err := proto.Marshal(s.getProtobufData())
	if err != nil {
		return nil, err
	}

	bn, ki, _, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return &protobuf.Entrypoint{
		BlobName: bn,
		KeyInfo:  &protobuf.KeyInfo{Key: ki},
		MimeType: CinodeDirMimeType,
	}, nil
}
