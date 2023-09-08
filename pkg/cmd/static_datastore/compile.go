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
	"github.com/cinode/go/pkg/protobuf"
	"github.com/cinode/go/pkg/structure"
	"github.com/jbenet/go-base58"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
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

			var wi *protobuf.WriterInfo
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
				_wi, err := protobuf.WriterInfoFromBytes(base58.Decode(rootWriterInfoStr))
				if err != nil {
					fatalResult("Couldn't parse writer info: %v", err)
				}
				wi = _wi
			}

			ep, wi, err := compileFS(srcDir, dstDir, useStaticBlobs, wi, useRawFilesystem)
			if err != nil {
				fatalResult("%s", err)
			}

			epBytes, err := ep.ToBytes()
			if err != nil {
				fatalResult("Couldn't serialize entrypoint: %v", err)
			}

			result := map[string]string{
				"result":     "OK",
				"entrypoint": base58.Encode(epBytes),
			}
			if wi != nil {
				wiBytes, err := wi.ToBytes()
				if err != nil {
					fatalResult("Couldn't serialize writer info: %v", err)
				}

				result["writer-info"] = base58.Encode(wiBytes)
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
	srcDir, dstDir string,
	static bool,
	writerInfo *protobuf.WriterInfo,
	useRawFS bool,
) (
	*protobuf.Entrypoint,
	*protobuf.WriterInfo,
	error,
) {
	var retWi *protobuf.WriterInfo

	ds, err := func() (datastore.DS, error) {
		if useRawFS {
			return datastore.InRawFileSystem(dstDir)
		}
		return datastore.InFileSystem(dstDir)
	}()
	if err != nil {
		return nil, nil, fmt.Errorf("could not open datastore: %w", err)
	}

	be := blenc.FromDatastore(ds)

	ep, err := structure.UploadStaticDirectory(
		context.Background(),
		slog.Default(),
		os.DirFS(srcDir),
		be,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't upload directory content: %w", err)
	}

	if !static {
		if writerInfo == nil {
			ep, retWi, err = structure.CreateLink(context.Background(), be, ep)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to update root link: %w", err)
			}
		} else {
			ep, err = structure.UpdateLink(context.Background(), be, writerInfo, ep)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to update root link: %w", err)
			}
		}
	}

	return ep, retWi, nil
}
