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
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/structure"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/chacha20"
	"google.golang.org/protobuf/proto"
)

func serverCmd() *cobra.Command {

	var dataStoreDir string

	cmd := &cobra.Command{
		Use:   "server --datastore <datastore_dir>",
		Short: "Serve files from datastore on given folder",
		Long: `
Serve files from static datastore from a directory.
`,
		Run: func(cmd *cobra.Command, args []string) {
			if dataStoreDir == "" {
				cmd.Help()
				return
			}
			server(dataStoreDir)
		},
	}

	cmd.Flags().StringVarP(&dataStoreDir, "datastore", "d", "", "Datastore directory containing blobs")

	return cmd
}

func getEntrypoint(datastoreDir string) ([]byte, []byte, error) {
	ep, err := os.Open(path.Join(datastoreDir, "entrypoint.txt"))
	if err != nil {
		return nil, nil, fmt.Errorf("can't open entrypoint file from %s", datastoreDir)
	}
	defer ep.Close()

	scanner := bufio.NewScanner(ep)
	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("malformed entrypoint file - missing bid")
	}
	bid, err := common.BlobNameFromString(scanner.Text())
	if err != nil {
		return nil, nil, fmt.Errorf("malformed entrypoint file - could not get blob name: %w", err)
	}

	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("malformed entrypoint file - missing key")
	}

	key, err := hex.DecodeString(scanner.Text())
	if err != nil {
		return nil, nil, fmt.Errorf("malformed entrypoint file - invalid key: %w", err)
	}

	return bid, key, nil
}

func handleDir(
	ctx context.Context,
	be blenc.BE,
	bid []byte,
	key []byte,
	w http.ResponseWriter,
	r *http.Request,
	subPath string,
) {

	if subPath == "" {
		subPath = "index.html"
	}

	pathParts := strings.SplitN(subPath, "/", 2)

	dirBytes := bytes.NewBuffer(nil)
	err := readWithLinkDereference(ctx, be, common.BlobName(bid), key, dirBytes)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	dir := structure.Directory{}
	if err := proto.Unmarshal(dirBytes.Bytes(), &dir); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	entry, exists := dir.GetEntries()[pathParts[0]]
	if !exists {
		http.NotFound(w, r)
		return
	}

	if entry.GetMimeType() == "application/cinode-dir" {
		if len(pathParts) == 0 {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusPermanentRedirect)
			return
		}
		handleDir(
			ctx,
			be,
			entry.GetBid(),
			entry.KeyInfo.GetKey(),
			w, r,
			pathParts[1],
		)
		return
	}

	if len(pathParts) > 1 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", entry.GetMimeType())
	err = be.Read(
		ctx,
		common.BlobName(entry.Bid),
		entry.KeyInfo.GetKey(),
		w,
	)
	if err != nil {
		// TODO: Log this, can't send an error back, it's too late
	}
}

func readWithLinkDereference(ctx context.Context, be blenc.BE, name common.BlobName, key []byte, w io.Writer) error {
	for redirectLevel := 0; redirectLevel < 10; redirectLevel++ {
		switch name.Type() {

		case blobtypes.Static:
			return be.Read(ctx, name, key, w)

		case blobtypes.DynamicLink:
			buff := bytes.NewBuffer(nil)
			err := be.Read(ctx, name, key, buff)
			if err != nil {
				return err
			}

			b := buff.Bytes()
			if len(b) < 1+chacha20.KeySize+1 {
				return fmt.Errorf("invalid dynamic link content")
			}

			if b[0] != 0 {
				return fmt.Errorf("invalid dynamic link content: non-zero reserved byte")
			}

			key = b[1 : 1+chacha20.KeySize]
			name = common.BlobName(b[1+chacha20.KeySize:])

		default:
			return blobtypes.ErrUnknownBlobType
		}
	}

	return fmt.Errorf("Too many dynamic link redirects")
}

func serverHandler(datastoreDir string) (http.Handler, error) {
	epBID, epKI, err := getEntrypoint(datastoreDir)
	if err != nil {
		return nil, err
	}

	be := blenc.FromDatastore(datastore.InFileSystem(datastoreDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = r.URL.Path[1:]
		}

		handleDir(r.Context(), be, epBID, epKI, w, r, r.URL.Path)
	}), nil
}

func server(datastoreDir string) {

	fmt.Println("Serving files from", datastoreDir)

	hnd, err := serverHandler(datastoreDir)
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", hnd)

	fmt.Println("Listening on http://localhost:8080/")
	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		log.Fatalln(err)
	}
}
