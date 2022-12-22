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
	"google.golang.org/protobuf/proto"
)

func compileCmd() *cobra.Command {

	var srcDir, dstDir string

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
			err := compile(srcDir, dstDir)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("DONE")
		},
	}

	cmd.Flags().StringVarP(&srcDir, "source", "s", "", "Source directory with content to compile")
	cmd.Flags().StringVarP(&dstDir, "destination", "d", "", "Destination directory for blobs")

	return cmd
}

func compile(srcDir, dstDir string) error {

	be := blenc.FromDatastore(datastore.InFileSystem(dstDir))

	name, key, err := compileOneLevel(srcDir, be)
	if err != nil {
		return err
	}

	fl, err := os.Create(path.Join(dstDir, "entrypoint.txt"))
	if err != nil {
		return err
	}
	keyType, keyKey, keyIV, err := key.GetSymmetricKey()
	if err != nil {
		log.Fatal(err)
	}
	_, err = fmt.Fprintf(
		fl,
		"%s\n%s:%s:%s\n",
		name,
		hex.EncodeToString([]byte{keyType}),
		hex.EncodeToString(keyKey),
		hex.EncodeToString(keyIV),
	)
	if err != nil {
		fl.Close()
		return err
	}

	err = fl.Close()
	if err != nil {
		return err
	}

	return nil
}

func compileOneLevel(path string, be blenc.BE) (common.BlobName, blenc.KeyInfo, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("Couldn't check path: %w", err)
	}

	if st.IsDir() {
		return compileDir(path, be)
	}

	if st.Mode().IsRegular() {
		return compileFile(path, be)
	}

	return nil, nil, fmt.Errorf("Neither dir nor a regular file: %v", path)
}

func compileFile(path string, be blenc.BE) (common.BlobName, blenc.KeyInfo, error) {
	fmt.Println(" *", path)
	fl, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("Couldn't read file %v: %w", path, err)
	}
	defer fl.Close()
	bn, ki, _, err := be.Create(context.Background(), blobtypes.Static, fl)
	return bn, ki, err
}

func compileDir(p string, be blenc.BE) (common.BlobName, blenc.KeyInfo, error) {
	fileList, err := os.ReadDir(p)
	if err != nil {
		return nil, nil, fmt.Errorf("Couldn't read contents of dir %v: %w", p, err)
	}
	dirStruct := structure.Directory{
		Entries: make(map[string]*structure.Directory_Entry),
	}
	for _, e := range fileList {
		subPath := path.Join(p, e.Name())
		name, ki, err := compileOneLevel(subPath, be)
		if err != nil {
			return nil, nil, err
		}
		contentType := "application/cinode-dir"
		if !e.IsDir() {
			contentType = mime.TypeByExtension(filepath.Ext(e.Name()))
			if contentType == "" {
				file, err := os.Open(subPath)
				if err != nil {
					return nil, nil, fmt.Errorf("Can not detect content type for %v: %w", subPath, err)
				}
				buffer := make([]byte, 512)
				n, err := io.ReadFull(file, buffer)
				file.Close()
				if err != nil && err != io.ErrUnexpectedEOF {
					return nil, nil, fmt.Errorf("Can not detect content type for %v: %w", subPath, err)
				}
				contentType = http.DetectContentType(buffer[:n])
			}
		}
		keyType, keyKey, keyIV, err := ki.GetSymmetricKey()
		if err != nil {
			return nil, nil, fmt.Errorf("Can not fetch key ifo for %v: %w", subPath, err)

		}
		dirStruct.Entries[e.Name()] = &structure.Directory_Entry{
			Bid: name,
			KeyInfo: &structure.KeyInfo{
				Type: uint32(keyType),
				Key:  keyKey,
				Iv:   keyIV,
			},
			MimeType: contentType,
		}
	}

	data, err := proto.Marshal(&dirStruct)
	if err != nil {
		return nil, nil, fmt.Errorf("Can not serialize directory %v: %w", p, err)
	}

	bn, ki, _, err := be.Create(context.Background(), blobtypes.Static, bytes.NewReader(data))
	return bn, ki, err
}
