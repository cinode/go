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
	"fmt"
	"log"
	"os"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/structure/graph"
	"github.com/cinode/go/pkg/structure/graphutils"
	"github.com/spf13/cobra"
)

func compileCmd() *cobra.Command {

	var srcDir, dstDir string
	var useStaticBlobs bool
	var useRawFilesystem bool
	var rootWriterInfoStr string
	var rootWriterInfoFile string

	cmd := &cobra.Command{
		Use:   "compile --source <src_dir> --destination <dst_dir>",
		Short: "Compile datastore from static files",
		Long: `
The compile command can be used to create an encrypted datastore from
a content with static files that can then be used to serve through a
simple http server.
`,
		Run: func(cmd *cobra.Command, args []string) {
			if srcDir == "" || dstDir == "" {
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

			var wi *graph.WriterInfo
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
				_wi, err := graph.WriterInfoFromString(rootWriterInfoStr)
				if err != nil {
					fatalResult("Couldn't parse writer info: %v", err)
				}
				wi = &_wi
			}

			ep, wi, err := compileFS(
				cmd.Context(),
				srcDir,
				dstDir,
				useStaticBlobs,
				wi,
				useRawFilesystem,
			)
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

	cmd.Flags().StringVarP(&srcDir, "source", "s", "", "Source directory with content to compile")
	cmd.Flags().StringVarP(&dstDir, "destination", "d", "", "Destination directory for blobs")
	cmd.Flags().BoolVarP(&useStaticBlobs, "static", "t", false, "If set to true, compile only the static dataset, do not create or update dynamic link")
	cmd.Flags().BoolVarP(&useRawFilesystem, "raw-filesystem", "r", false, "If set to true, use raw filesystem instead of the optimized one, can be used to create dataset for a standard http server")
	cmd.Flags().StringVarP(&rootWriterInfoStr, "writer-info", "w", "", "Writer info for the root dynamic link, if neither writer info nor writer info file is specified, a random writer info will be generated and printed out")
	cmd.Flags().StringVarP(&rootWriterInfoFile, "writer-info-file", "f", "", "Name of the file containing writer info for the root dynamic link, if neither writer info nor writer info file is specified, a random writer info will be generated and printed out")

	return cmd
}

func compileFS(
	ctx context.Context,
	srcDir, dstDir string,
	static bool,
	writerInfo *graph.WriterInfo,
	useRawFS bool,
) (
	*graph.Entrypoint,
	*graph.WriterInfo,
	error,
) {
	ds, err := func() (datastore.DS, error) {
		if useRawFS {
			return datastore.InRawFileSystem(dstDir)
		}
		return datastore.InFileSystem(dstDir)
	}()
	if err != nil {
		return nil, nil, fmt.Errorf("could not open datastore: %w", err)
	}

	opts := []graph.CinodeFSOption{}
	if static {
		opts = append(opts, graph.NewRootStaticDirectory())
	} else if writerInfo == nil {
		opts = append(opts, graph.NewRootDynamicLink())
	} else {
		opts = append(opts, graph.RootWriterInfo(*writerInfo))
	}

	fs, err := graph.NewCinodeFS(
		ctx,
		blenc.FromDatastore(ds),
		opts...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create cinode filesystem instance: %w", err)
	}

	err = graphutils.UploadStaticDirectory(ctx, os.DirFS(srcDir), fs)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't upload directory content: %w", err)
	}

	ep, err := fs.RootEntrypoint()
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't get root entrypoint from cinodefs instance: %w", err)
	}

	wi, err := fs.RootWriterInfo(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't get root writer info from cinodefs instance: %w", err)
	}

	return ep, &wi, nil
}
