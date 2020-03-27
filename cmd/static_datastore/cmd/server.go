package cmd

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/cinode/go/blenc"
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

func getEntrypoint(datastoreDir string) (string, string) {
	ep, err := os.Open(path.Join(datastoreDir, "entrypoint.txt"))
	if err != nil {
		log.Fatalf("Can't open entrypoint file from %s\n", datastoreDir)
	}
	defer ep.Close()

	scanner := bufio.NewScanner(ep)
	scanner.Scan()
	bid := scanner.Text()
	scanner.Scan()
	key := scanner.Text()
	if err := scanner.Err(); err != nil {
		log.Fatalf("Cant find entrypoint: %v", err)
	}
	return bid, key
}

func handleDir(be blenc.BE, bid, key string, w http.ResponseWriter, r *http.Request, subPath string) {

	if subPath == "" {
		subPath = "index.html"
	}

	pathParts := strings.SplitN(subPath, "/", 2)

	dirData, err := be.Open(bid, key)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	dirBytes, err := ioutil.ReadAll(dirData)
	dirData.Close()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	dir := structure.Directory{}
	if err := proto.Unmarshal(dirBytes, &dir); err != nil {
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
		handleDir(be, entry.GetBid(), entry.GetKey(), w, r, pathParts[1])
		return
	}

	if len(pathParts) > 1 {
		http.NotFound(w, r)
		return
	}

	data, err := be.Open(entry.GetBid(), entry.GetKey())
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	defer data.Close()
	w.Header().Set("Content-Type", entry.GetMimeType())
	if _, err = io.Copy(w, data); err != nil {
		// TODO: Log this, can't send an error back, it's too late
	}
}

func server(datastoreDir string) {

	fmt.Println("Serving files from", datastoreDir)

	epBID, epKEY := getEntrypoint(datastoreDir)

	be := blenc.FromDatastore(datastore.InFileSystem(datastoreDir))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		handleDir(be, epBID, epKEY, w, r, r.URL.Path[1:])
	})

	fmt.Println("Listening on http://localhost:8080/")
	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		log.Fatalln(err)
	}
}
