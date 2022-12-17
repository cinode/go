package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/cinode/go/blenc"
	"github.com/cinode/go/common"
	"github.com/cinode/go/datastore"
	"github.com/cinode/go/structure"
	"github.com/spf13/cobra"
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

func getEntrypoint(datastoreDir string) ([]byte, blenc.KeyInfo) {
	ep, err := os.Open(path.Join(datastoreDir, "entrypoint.txt"))
	if err != nil {
		log.Fatalf("Can't open entrypoint file from %s", datastoreDir)
	}
	defer ep.Close()

	scanner := bufio.NewScanner(ep)
	if !scanner.Scan() {
		log.Fatalf("Malformed entrypoint file - missing bid")
	}
	bid, err := common.BlobNameFromString(scanner.Text())
	if err != nil {
		log.Fatalf("Malformed entrypoint file - could not get blob name: %v", err.Error())
	}

	if !scanner.Scan() {
		log.Fatalf("Malformed entrypoint file - missing key info")
	}

	keyInfoText := strings.Split(scanner.Text(), ":")
	if len(keyInfoText) != 3 {
		log.Fatalf("Malformed entrypoint file - invalid key info, must be 3 segments split by ':'")
	}
	keyType, err := hex.DecodeString(keyInfoText[0])
	if err != nil {
		log.Fatalf("Malformed entrypoint file - invalid key info, key type segment can not be hex-decoded: %v", err.Error())
	}
	if len(keyType) != 1 {
		log.Fatalf("Malformed entrypoint file - invalid key info, key type segment must be one byte")
	}

	keyKey, err := hex.DecodeString(keyInfoText[1])
	if err != nil {
		log.Fatalf("Malformed entrypoint file - invalid key info, key segment can not be hex-decoded: %v", err.Error())
	}

	keyIV, err := hex.DecodeString(keyInfoText[2])
	if err != nil {
		log.Fatalf("Malformed entrypoint file - invalid key info, IV can not be hex-decoded: %v", err.Error())
	}
	return bid, blenc.NewStaticKeyInfo(
		keyType[0],
		keyKey,
		keyIV,
	)
}

func handleDir(
	ctx context.Context,
	be blenc.BE,
	bid []byte,
	ki blenc.KeyInfo,
	w http.ResponseWriter,
	r *http.Request,
	subPath string,
) {

	if subPath == "" {
		subPath = "index.html"
	}

	pathParts := strings.SplitN(subPath, "/", 2)

	dirBytes := bytes.NewBuffer(nil)
	err := be.Read(ctx, common.BlobName(bid), ki, dirBytes)
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
			blenc.NewStaticKeyInfo(
				byte(entry.GetKeyInfo().GetType()),
				entry.GetKeyInfo().GetKey(),
				entry.GetKeyInfo().GetIv(),
			),
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
		blenc.NewStaticKeyInfo(
			byte(entry.GetKeyInfo().GetType()),
			entry.GetKeyInfo().GetKey(),
			entry.GetKeyInfo().GetIv(),
		),
		w,
	)
	if err != nil {
		// TODO: Log this, can't send an error back, it's too late
	}
}

func server(datastoreDir string) {

	fmt.Println("Serving files from", datastoreDir)

	epBID, epKI := getEntrypoint(datastoreDir)

	be := blenc.FromDatastore(datastore.InFileSystem(datastoreDir))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = r.URL.Path[1:]
		}

		handleDir(r.Context(), be, epBID, epKI, w, r, r.URL.Path)
	})

	fmt.Println("Listening on http://localhost:8080/")
	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		log.Fatalln(err)
	}
}
