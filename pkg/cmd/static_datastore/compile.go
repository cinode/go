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

package static_datastore

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/protobuf"
	"github.com/cinode/go/pkg/structure"
	"github.com/spf13/cobra"
)

func compileCmd() *cobra.Command {

	var srcDir, dstDir string
	var useStaticBlobs bool
	var rootWriterInfo []byte

	cmd := &cobra.Command{
		Use:   "compile --source <src_dir> --destination <dst_dir>",
		Short: "Compile datastore from static files",
		Long: `
The compile command can be used to create an encrypted datastore from
a content with static files that can then be used to serve through a
simple http server.

Files stored on disk are encrypted through the blenc layer. However
this tool should not be considered secure since the encryption key
for the root node is stored in plaintext in an 'entrypoint.txt' file.
`,
		Run: func(cmd *cobra.Command, args []string) {
			if srcDir == "" || dstDir == "" {
				cmd.Help()
				return
			}

			var wi *protobuf.WriterInfo
			if len(rootWriterInfo) > 0 {
				_wi, err := protobuf.WriterInfoFromBytes(rootWriterInfo)
				if err != nil {
					log.Fatalf("Couldn't parse writer info: %v", err)
				}
				wi = _wi
			}

			wi, err := compileFS(srcDir, dstDir, useStaticBlobs, wi)
			if err != nil {
				log.Fatal(err)
			}
			if wi != nil {
				wiBytes, err := wi.ToBytes()
				if err != nil {
					log.Fatalf("Couldn't serialize writer info: %v", err)
				}
				log.Printf("Generated new root dynamic link, writer info: %X", wiBytes)
			}
			log.Println()
			log.Println("DONE")
		},
	}

	cmd.Flags().StringVarP(&srcDir, "source", "s", "", "Source directory with content to compile")
	cmd.Flags().StringVarP(&dstDir, "destination", "d", "", "Destination directory for blobs")
	cmd.Flags().BoolVarP(&useStaticBlobs, "static", "t", false, "If set to true, compile static dataset and entrypoint.txt file with static dataset")
	cmd.Flags().BytesHexVarP(&rootWriterInfo, "writer-info", "w", nil, "Writer info for the rood dynamic link, if not specified, a random writer info will be generated and printed out")

	return cmd
}

func compileFS(srcDir, dstDir string, static bool, writerInfo *protobuf.WriterInfo) (*protobuf.WriterInfo, error) {
	var retWi *protobuf.WriterInfo

	be := blenc.FromDatastore(datastore.InFileSystem(dstDir))

	ep, err := structure.UploadStaticDirectory(context.Background(), os.DirFS(srcDir), be)
	if err != nil {
		return nil, fmt.Errorf("couldn't upload directory content: %w", err)
	}

	if !static {
		if writerInfo == nil {
			ep, retWi, err = structure.CreateLink(context.Background(), be, ep)
			if err != nil {
				return nil, fmt.Errorf("failed to update root link: %w", err)
			}
		} else {
			ep, err = structure.UpdateLink(context.Background(), be, writerInfo, ep)
			if err != nil {
				return nil, fmt.Errorf("failed to update root link: %w", err)
			}
		}
	}

	epBytes, err := ep.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize entrypoint data: %w", err)
	}

	err = os.WriteFile(
		path.Join(dstDir, "entrypoint.txt"),
		[]byte(hex.EncodeToString(epBytes)),
		0666,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to write entrypoint data: %w", err)
	}

	return retWi, nil
}
