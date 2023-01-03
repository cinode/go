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
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/protobuf"
	"github.com/cinode/go/pkg/structure"
	"github.com/spf13/cobra"
)

func serverCmd() *cobra.Command {

	var dataStoreDir string

	cmd := &cobra.Command{
		Use:   "server --datastore <datastore_dir>",
		Short: "Serve files from datastore on given folder",
		Long: `
Serve files from datastore files from a directory.
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

func getEntrypoint(datastoreDir string) (*protobuf.Entrypoint, error) {

	data, err := os.ReadFile(path.Join(datastoreDir, "entrypoint.txt"))
	if err != nil {
		return nil, fmt.Errorf("can't read entrypoint data from %s", datastoreDir)
	}

	data, err = hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid entrypoint data file in %s - not a hexadecimal string", datastoreDir)
	}

	return protobuf.EntryPointFromBytes(data)
}

func handleDir(
	ctx context.Context,
	be blenc.BE,
	ep *protobuf.Entrypoint,
	w http.ResponseWriter,
	r *http.Request,
	subPath string,
) {

}

func serverHandler(datastoreDir string) (http.Handler, error) {
	ep, err := getEntrypoint(datastoreDir)
	if err != nil {
		return nil, err
	}

	dh := structure.CinodeFS{
		BE:               blenc.FromDatastore(datastore.InFileSystem(datastoreDir)),
		RootEntrypoint:   ep,
		MaxLinkRedirects: 10,
		IndexFile:        "index.html",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")

		fileEP, err := dh.FindEntrypoint(r.Context(), path)
		switch {
		case errors.Is(err, structure.ErrNotFound):
			http.NotFound(w, r)
			return
		case err != nil:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if fileEP.MimeType == structure.CinodeDirMimeType {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusPermanentRedirect)
			return
		}

		w.Header().Set("Content-Type", fileEP.GetMimeType())
		rc, err := dh.OpenContent(r.Context(), fileEP)
		if err != nil {
			log.Printf("Error sending file: %v", err)
		}
		defer rc.Close()

		_, err = io.Copy(w, rc)
		if err != nil {
			log.Printf("Error sending file: %v", err)
		}

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
