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
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/structure"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/chacha20"
	"google.golang.org/protobuf/proto"
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

			wi, err := compileFS(srcDir, dstDir, useStaticBlobs, rootWriterInfo)
			if err != nil {
				log.Fatal(err)
			}
			if len(wi) != 0 {
				log.Printf("Generated new root dynamic link, writer info: %X", wi)
			}
			log.Println("")
			fmt.Println("DONE")
		},
	}

	cmd.Flags().StringVarP(&srcDir, "source", "s", "", "Source directory with content to compile")
	cmd.Flags().StringVarP(&dstDir, "destination", "d", "", "Destination directory for blobs")
	cmd.Flags().BoolVarP(&useStaticBlobs, "static", "t", false, "If set to true, compile static dataset and entrypoint.txt file with static dataset")
	cmd.Flags().BytesBase64VarP(&rootWriterInfo, "writer-info", "w", nil, "Writer info for the rood dynamic link, if not specified, a random writer info will be generated and printed out")

	return cmd
}

func appendBytes(parts ...[]byte) []byte {
	out := []byte{}

	for _, p := range parts {
		out = append(out, p...)
	}

	return out
}

func compileFS(srcDir, dstDir string, static bool, writerInfo []byte) ([]byte, error) {
	var wi []byte

	be := blenc.FromDatastore(datastore.InFileSystem(dstDir))

	// Compile static files first
	name, key, err := compileOneLevel(srcDir, be)
	if err != nil {
		return nil, err
	}

	if !static {
		// Build dynamic link to the root

		if len(key) != chacha20.KeySize {
			panic("Invalid key length")
		}

		linkData := bytes.NewBuffer(nil)
		linkData.WriteByte(0) // reserved byte
		linkData.Write(key)
		linkData.Write(name)

		if len(writerInfo) == 0 {
			// Creating new link
			name, key, wi, err = be.Create(context.Background(), blobtypes.DynamicLink, bytes.NewReader(linkData.Bytes()))
			if err != nil {
				log.Fatal(err)
			}

			// Writer info needs to be extended to contain name and key
			wi = appendBytes(
				[]byte{byte(len(name))},
				name,
				[]byte{byte(len(key))},
				key,
				wi,
			)

		} else {
			// Update existing link

			// extract name and key first, pure writerInfo does not explicitly expose those values
			bnLen := writerInfo[0]
			name = common.BlobName(writerInfo[1 : bnLen+1])
			writerInfo = writerInfo[bnLen+1:]

			keyLen := writerInfo[0]
			key = writerInfo[1 : keyLen+1]
			writerInfo = writerInfo[keyLen+1:]

			err = be.Update(context.Background(), name, writerInfo, key, bytes.NewReader(linkData.Bytes()))
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	fl, err := os.Create(path.Join(dstDir, "entrypoint.txt"))
	if err != nil {
		return nil, err
	}
	_, err = fmt.Fprintf(
		fl,
		"%s\n%s\n",
		name,
		hex.EncodeToString(key),
	)
	if err != nil {
		fl.Close()
		return nil, err
	}

	err = fl.Close()
	if err != nil {
		return nil, err
	}

	return wi, nil
}

func compileOneLevel(path string, be blenc.BE) (common.BlobName, []byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't check path: %w", err)
	}

	if st.IsDir() {
		return compileDir(path, be)
	}

	if st.Mode().IsRegular() {
		return compileFile(path, be)
	}

	return nil, nil, fmt.Errorf("neither dir nor a regular file: %v", path)
}

func compileFile(path string, be blenc.BE) (common.BlobName, []byte, error) {
	fmt.Println(" *", path)
	fl, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't read file %v: %w", path, err)
	}
	defer fl.Close()
	bn, ki, _, err := be.Create(context.Background(), blobtypes.Static, fl)
	return bn, ki, err
}

func compileDir(p string, be blenc.BE) (common.BlobName, []byte, error) {
	fileList, err := os.ReadDir(p)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't read contents of dir %v: %w", p, err)
	}
	dirStruct := structure.Directory{
		Entries: make(map[string]*structure.Directory_Entry),
	}
	for _, e := range fileList {
		subPath := path.Join(p, e.Name())
		name, key, err := compileOneLevel(subPath, be)
		if err != nil {
			return nil, nil, err
		}
		contentType := "application/cinode-dir"
		if !e.IsDir() {
			contentType = mime.TypeByExtension(filepath.Ext(e.Name()))
			if contentType == "" {
				file, err := os.Open(subPath)
				if err != nil {
					return nil, nil, fmt.Errorf("can not detect content type for %v: %w", subPath, err)
				}
				buffer := make([]byte, 512)
				n, err := io.ReadFull(file, buffer)
				file.Close()
				if err != nil && err != io.ErrUnexpectedEOF {
					return nil, nil, fmt.Errorf("can not detect content type for %v: %w", subPath, err)
				}
				contentType = http.DetectContentType(buffer[:n])
			}
		}
		dirStruct.Entries[e.Name()] = &structure.Directory_Entry{
			Bid: name,
			KeyInfo: &structure.KeyInfo{
				Key: key,
			},
			MimeType: contentType,
		}
	}

	data, err := proto.Marshal(&dirStruct)
	if err != nil {
		return nil, nil, fmt.Errorf("can not serialize directory %v: %w", p, err)
	}

	bn, ki, _, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(data))
	return bn, ki, err
}
