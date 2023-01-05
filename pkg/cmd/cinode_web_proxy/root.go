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

package cinode_web_proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/protobuf"
	"github.com/cinode/go/pkg/structure"
	"github.com/jbenet/go-base58"
)

const (
	filePrefix     = "file://"
	rawFilePrefix  = "file-raw://"
	webPrefixHttp  = "http://"
	webPrefixHttps = "https://"
	memoryPrefix   = "memory://"
)

func buildDS(name string) (datastore.DS, error) {
	switch {
	case name == "":
		return datastore.InMemory(), nil

	case strings.HasPrefix(name, filePrefix):
		return datastore.InFileSystem(name[len(filePrefix):]), nil

	case strings.HasPrefix(name, rawFilePrefix):
		return datastore.InRawFileSystem(name[len(rawFilePrefix):]), nil

	case strings.HasPrefix(name, webPrefixHttp),
		strings.HasPrefix(name, webPrefixHttps):
		return datastore.FromWeb(name), nil

	case strings.HasPrefix(name, memoryPrefix):
		if name != memoryPrefix {
			return nil, fmt.Errorf("memory datastream must not use any parameters, use `%s`", memoryPrefix)
		}
		return datastore.InMemory(), nil

	default:
		return datastore.InFileSystem(name), nil
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	entrypointB58, found := os.LookupEnv("CINODE_ENTRYPOINT")
	if !found {
		entrypointFile, found := os.LookupEnv("CINODE_ENTRYPOINT_FILE")
		if !found {
			log.Fatal("Missing CINODE_ENTRYPOINT or CINODE_ENTRYPOINT_FILE env var")
		}
		entrypointFileData, err := os.ReadFile(entrypointFile)
		if err != nil {
			log.Fatalf("Could not read entrypoint file at '%s': %v", entrypointFile, err)
		}
		entrypointB58 = string(bytes.TrimSpace(entrypointFileData))
	}
	entrypointRaw := base58.Decode(entrypointB58)
	if len(entrypointRaw) == 0 {
		log.Fatalf("Could not decode hex entrypoint data")
	}
	entrypoint, err := protobuf.EntryPointFromBytes(entrypointRaw)
	if err != nil {
		log.Fatalf("Could not unmarshal entrypoint data: %v", err)
	}

	mainDS, err := buildDS(os.Getenv("CINODE_MAIN_DATASTORE"))
	if err != nil {
		log.Fatalf("Could not create main datastore: %v", err)
	}

	additionalDSs := []datastore.DS{}
	additionalDSEnvNames := []string{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CINODE_ADDITIONAL_DATASTORE_") {
			split := strings.SplitN(e, "=", 2)
			additionalDSEnvNames = append(additionalDSEnvNames, split[0])
		}
	}
	sort.Strings(additionalDSEnvNames)

	for _, envName := range additionalDSEnvNames {
		ds, err := buildDS(os.Getenv(envName))
		if err != nil {
			log.Fatalf("Could not create additional datastore from env var %s: %v", envName, err)
		}
		additionalDSs = append(additionalDSs, ds)
	}

	fs := structure.CinodeFS{
		BE: blenc.FromDatastore(
			datastore.NewMultiSource(mainDS, time.Hour, additionalDSs...),
		),
		RootEntrypoint:   entrypoint,
		MaxLinkRedirects: 10,
		IndexFile:        "index.html",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")

		fileEP, err := fs.FindEntrypoint(r.Context(), path)
		switch {
		case errors.Is(err, structure.ErrNotFound):
			http.NotFound(w, r)
			return
		case err != nil:
			log.Println("Error serving request:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if fileEP.MimeType == structure.CinodeDirMimeType {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusPermanentRedirect)
			return
		}

		w.Header().Set("Content-Type", fileEP.GetMimeType())
		rc, err := fs.OpenContent(r.Context(), fileEP)
		if err != nil {
			log.Printf("Error sending file: %v", err)
		}
		defer rc.Close()

		_, err = io.Copy(w, rc)
		if err != nil {
			log.Printf("Error sending file: %v", err)
		}

	})

	log.Println("Listening on http://localhost:8080")
	err = http.ListenAndServe(":8080", handler)
	if err != nil {
		log.Fatal(err)
	}
}
