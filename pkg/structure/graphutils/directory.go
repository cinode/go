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

package graphutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"

	_ "embed"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/structure/graph"
	"github.com/cinode/go/pkg/utilities/golang"
	"golang.org/x/exp/slog"
)

const (
	CinodeDirMimeType = "application/cinode-dir"
)

var (
	ErrNotFound      = blenc.ErrNotFound
	ErrNotADirectory = errors.New("entry is not a directory")
	ErrNotAFile      = errors.New("entry is not a file")
)

func UploadStaticDirectory(
	ctx context.Context,
	fsys fs.FS,
	cfs graph.CinodeFS,
	opts ...UploadStaticDirectoryOption,
) error {
	c := dirCompiler{
		ctx:  ctx,
		fsys: fsys,
		cfs:  cfs,
		log:  slog.Default(),
	}
	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return err
		}
	}

	_, err := c.compilePath(ctx, ".", c.basePath)
	if err != nil {
		return err
	}

	err = cfs.Flush(ctx)
	if err != nil {
		return err
	}

	return nil
}

type UploadStaticDirectoryOption func(d *dirCompiler) error

func BasePath(path []string) UploadStaticDirectoryOption {
	return UploadStaticDirectoryOption(func(d *dirCompiler) error {
		d.basePath = path
		return nil
	})
}

func CreateIndexFile(indexFile string) UploadStaticDirectoryOption {
	return UploadStaticDirectoryOption(func(d *dirCompiler) error {
		d.createIndexFile = true
		d.indexFileName = indexFile
		return nil
	})
}

type dirCompiler struct {
	ctx             context.Context
	fsys            fs.FS
	cfs             graph.CinodeFS
	log             *slog.Logger
	basePath        []string
	createIndexFile bool
	indexFileName   string
}

type dirEntry struct {
	Name     string
	MimeType string
	IsDir    bool
	Size     int64
}

func (d *dirCompiler) compilePath(
	ctx context.Context,
	srcPath string,
	destPath []string,
) (*dirEntry, error) {
	st, err := fs.Stat(d.fsys, srcPath)
	if err != nil {
		d.log.ErrorCtx(ctx, "failed to stat path", "path", srcPath, "err", err)
		return nil, fmt.Errorf("couldn't check path: %w", err)
	}

	var name string
	if len(destPath) > 0 {
		name = destPath[len(destPath)-1]
	}

	if st.IsDir() {
		size, err := d.compileDir(ctx, srcPath, destPath)
		if err != nil {
			return nil, err
		}
		return &dirEntry{
			Name:     name,
			MimeType: graph.CinodeDirMimeType,
			IsDir:    true,
			Size:     int64(size),
		}, nil
	}

	if st.Mode().IsRegular() {
		mime, err := d.compileFile(ctx, srcPath, destPath)
		if err != nil {
			return nil, err
		}
		return &dirEntry{
			Name:     name,
			MimeType: mime,
			IsDir:    false,
			Size:     st.Size(),
		}, nil
	}

	d.log.ErrorContext(ctx, "path is neither dir nor a regular file", "path", srcPath)
	return nil, fmt.Errorf("neither dir nor a regular file: %v", srcPath)
}

func (d *dirCompiler) compileFile(ctx context.Context, srcPath string, dstPath []string) (string, error) {
	d.log.InfoContext(ctx, "compiling file", "path", srcPath)
	fl, err := d.fsys.Open(srcPath)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to open file", "path", srcPath, "err", err)
		return "", fmt.Errorf("couldn't open file %v: %w", srcPath, err)
	}
	defer fl.Close()

	ep, err := d.cfs.SetEntryFile(ctx, dstPath, fl)
	if err != nil {
		return "", fmt.Errorf("failed to upload file %v: %w", srcPath, err)
	}

	return ep.MimeType(), nil
}

func (d *dirCompiler) compileDir(ctx context.Context, srcPath string, dstPath []string) (int, error) {
	fileList, err := fs.ReadDir(d.fsys, srcPath)
	if err != nil {
		d.log.ErrorContext(ctx, "couldn't read contents of dir", "path", srcPath, "err", err)
		return 0, fmt.Errorf("couldn't read contents of dir %v: %w", srcPath, err)
	}

	// TODO: Reset directory content
	// TODO: Build index file

	entries := make([]*dirEntry, 0, len(fileList))
	hasIndex := false

	for _, e := range fileList {
		entry, err := d.compilePath(
			ctx,
			path.Join(srcPath, e.Name()),
			append(dstPath, e.Name()),
		)
		if err != nil {
			return 0, err
		}

		if entry.Name == d.indexFileName {
			hasIndex = true
		} else {
			entries = append(entries, entry)
		}
	}

	if d.createIndexFile && !hasIndex {
		buf := bytes.NewBuffer(nil)
		err = dirIndexTemplate.Execute(buf, map[string]any{
			"entries":   entries,
			"indexName": d.indexFileName,
		})
		if err != nil {
			return 0, err
		}

		_, err = d.cfs.SetEntryFile(ctx,
			append(dstPath, d.indexFileName),
			bytes.NewReader(buf.Bytes()),
		)
		if err != nil {
			return 0, err
		}
	}

	return len(fileList), nil
}

//go:embed templates/dir.html
var _dirIndexTemplateStr string
var dirIndexTemplate = golang.Must(
	template.New("dir").Parse(_dirIndexTemplateStr),
)
