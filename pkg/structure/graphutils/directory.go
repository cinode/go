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
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"

	_ "embed"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/structure/graph"
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

	err := c.compilePath(ctx, ".", c.basePath)
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

type dirCompiler struct {
	ctx      context.Context
	fsys     fs.FS
	cfs      graph.CinodeFS
	log      *slog.Logger
	basePath []string
}

func (d *dirCompiler) compilePath(
	ctx context.Context,
	srcPath string,
	destPath []string,
) error {
	st, err := fs.Stat(d.fsys, srcPath)
	if err != nil {
		d.log.ErrorCtx(ctx, "failed to stat path", "path", srcPath, "err", err)
		return fmt.Errorf("couldn't check path: %w", err)
	}

	if st.IsDir() {
		return d.compileDir(ctx, srcPath, destPath)
	}

	if st.Mode().IsRegular() {
		return d.compileFile(ctx, srcPath, destPath)
	}

	d.log.ErrorContext(ctx, "path is neither dir nor a regular file", "path", srcPath)
	return fmt.Errorf("neither dir nor a regular file: %v", srcPath)
}

func (d *dirCompiler) compileFile(ctx context.Context, srcPath string, dstPath []string) error {
	d.log.InfoContext(ctx, "compiling file", "path", srcPath)
	fl, err := d.fsys.Open(srcPath)
	if err != nil {
		d.log.ErrorContext(ctx, "failed to open file", "path", srcPath, "err", err)
		return fmt.Errorf("couldn't open file %v: %w", srcPath, err)
	}
	defer fl.Close()

	_, err = d.cfs.SetEntryFile(ctx, dstPath, fl)
	if err != nil {
		return fmt.Errorf("failed to upload file %v: %w", srcPath, err)
	}

	return nil
}

func (d *dirCompiler) compileDir(ctx context.Context, srcPath string, dstPath []string) error {
	fileList, err := fs.ReadDir(d.fsys, srcPath)
	if err != nil {
		d.log.ErrorContext(ctx, "couldn't read contents of dir", "path", srcPath, "err", err)
		return fmt.Errorf("couldn't read contents of dir %v: %w", srcPath, err)
	}

	// TODO: Reset directory content
	// TODO: Build index file

	for _, e := range fileList {
		err := d.compilePath(
			ctx,
			path.Join(srcPath, e.Name()),
			append(dstPath, e.Name()),
		)
		if err != nil {
			return err
		}
	}

	return nil
}
