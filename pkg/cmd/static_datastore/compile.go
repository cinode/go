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

package static_datastore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/cinodefs/uploader"
	"github.com/cinode/go/pkg/datastore"
	"github.com/spf13/cobra"
)

func compileCmd() *cobra.Command {
	var o compileFSOptions
	var rootWriterInfoStr string
	var rootWriterInfoFile string
	var useRawFilesystem bool

	cmd := &cobra.Command{
		Use:   "compile --source <src_dir> --destination <dst_location>",
		Short: "Compile datastore from static files",
		Long: strings.Join([]string{
			"The compile command can be used to create an encrypted datastore from",
			"a content with static files that can then be used to serve through a",
			"simple http server.",
		}, "\n"),
		Run: func(cmd *cobra.Command, args []string) {
			if o.srcDir == "" || o.dstLocation == "" {
				cmd.Help()
				return
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")

			fatalResult := func(format string, args ...interface{}) {
				msg := fmt.Sprintf(format, args...)

				enc.Encode(map[string]string{
					"result": "ERROR",
					"msg":    msg,
				})

				log.Fatalf(msg)
			}

			if len(rootWriterInfoFile) > 0 {
				data, err := os.ReadFile(rootWriterInfoFile)
				if err != nil {
					fatalResult("Couldn't read data from the writer info file at '%s': %v", rootWriterInfoFile, err)
				}
				if len(data) == 0 {
					fatalResult("Writer info file at '%s' is empty", rootWriterInfoFile)
				}
				rootWriterInfoStr = string(data)
			}
			if len(rootWriterInfoStr) > 0 {
				wi, err := cinodefs.WriterInfoFromString(rootWriterInfoStr)
				if err != nil {
					fatalResult("Couldn't parse writer info: %v", err)
				}
				o.writerInfo = wi
			}

			if useRawFilesystem {
				// For backwards compatibility
				o.dstLocation = "file-raw://" + o.dstLocation
			}

			ep, wi, err := compileFS(cmd.Context(), o)
			if err != nil {
				fatalResult("%s", err)
			}

			result := map[string]string{
				"result":     "OK",
				"entrypoint": ep.String(),
			}
			if wi != nil {
				result["writer-info"] = wi.String()
			}
			enc.Encode(result)

			log.Println("DONE")
		},
	}

	cmd.Flags().StringVarP(
		&o.srcDir, "source", "s", "",
		"Source directory with content to compile",
	)
	cmd.Flags().StringVarP(
		&o.dstLocation, "destination", "d", "",
		"location of destination datastore for blobs, can be a directory "+
			"or an url prefixed with file://, file-raw://, http://, https://",
	)
	cmd.Flags().BoolVarP(
		&o.static, "static", "t", false,
		"if set to true, compile only the static dataset, do not create or update dynamic link",
	)
	cmd.Flags().BoolVarP(
		&useRawFilesystem, "raw-filesystem", "r", false,
		"if set to true, use raw filesystem instead of the optimized one, "+
			"can be used to create dataset for a standard http server",
	)
	cmd.Flags().MarkDeprecated(
		"raw-filesystem",
		"use file-raw:// destination prefix instead",
	)
	cmd.Flags().StringVarP(
		&rootWriterInfoStr, "writer-info", "w", "",
		"writer info for the root dynamic link, if neither writer info nor writer info file is specified, "+
			"a random writer info will be generated and printed out",
	)
	cmd.Flags().StringVarP(
		&rootWriterInfoFile, "writer-info-file", "f", "",
		"name of the file containing writer info for the root dynamic link, "+
			"if neither writer info nor writer info file is specified, "+
			"a random writer info will be generated and printed out",
	)
	cmd.Flags().StringVar(
		&o.indexFile, "index-file", "index.html",
		"name of the index file",
	)
	cmd.Flags().BoolVar(
		&o.generateIndexFiles, "generate-index-files", false,
		"automatically generate index html files with directory listing if index file is not present",
	)
	cmd.Flags().BoolVar(
		&o.append, "append", false,
		"append file in existing datastore leaving existing unchanged files as is",
	)

	return cmd
}

type compileFSOptions struct {
	srcDir             string
	dstLocation        string
	static             bool
	writerInfo         *cinodefs.WriterInfo
	generateIndexFiles bool
	indexFile          string
	append             bool
}

func compileFS(
	ctx context.Context,
	o compileFSOptions,
) (
	*cinodefs.Entrypoint,
	*cinodefs.WriterInfo,
	error,
) {
	ds, err := datastore.FromLocation(o.dstLocation)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open datastore: %w", err)
	}

	opts := []cinodefs.Option{}
	if o.static {
		opts = append(opts, cinodefs.NewRootStaticDirectory())
	} else if o.writerInfo == nil {
		opts = append(opts, cinodefs.NewRootDynamicLink())
	} else {
		opts = append(opts, cinodefs.RootWriterInfo(o.writerInfo))
	}

	fs, err := cinodefs.New(
		ctx,
		blenc.FromDatastore(ds),
		opts...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create cinode filesystem instance: %w", err)
	}

	if !o.append {
		err = fs.ResetDir(ctx, []string{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to reset the root directory: %w", err)
		}
	}

	var genOpts []uploader.Option
	if o.generateIndexFiles {
		genOpts = append(genOpts, uploader.CreateIndexFile(o.indexFile))
	}

	err = uploader.UploadStaticDirectory(
		ctx,
		os.DirFS(o.srcDir),
		fs,
		genOpts...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't upload directory content: %w", err)
	}

	ep, err := fs.RootEntrypoint()
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't get root entrypoint from cinodefs instance: %w", err)
	}

	wi, err := fs.RootWriterInfo(ctx)
	if errors.Is(err, cinodefs.ErrNotALink) {
		return ep, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't get root writer info from cinodefs instance: %w", err)
	}

	return ep, wi, nil
}
