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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	entrypoint, err := getEntrypoint()
	if err != nil {
		log.Fatalf("Failed to initialize entrypoint: %v", err)
	}

	mainDS, err := getMainDS()
	if err != nil {
		log.Fatalf("Could not create main datastore: %v", err)
	}

	additionalDSs, err := getAdditionalDSs()
	if err != nil {
		log.Fatalf("Could not create additional datastores: %v", err)
	}

	handler := setupCinodeProxy(mainDS, additionalDSs, entrypoint)

	log.Println("Listening on http://localhost:8080")
	err = http.ListenAndServe(":8080", handler)
	if err != nil {
		log.Fatal(err)
	}
}

func getEntrypoint() (*protobuf.Entrypoint, error) {
	entrypointB58, found := os.LookupEnv("CINODE_ENTRYPOINT")
	if !found {
		entrypointFile, found := os.LookupEnv("CINODE_ENTRYPOINT_FILE")
		if !found {
			return nil, errors.New("missing CINODE_ENTRYPOINT or CINODE_ENTRYPOINT_FILE env var")
		}
		entrypointFileData, err := os.ReadFile(entrypointFile)
		if err != nil {
			return nil, fmt.Errorf("could not read entrypoint file at '%s': %w", entrypointFile, err)
		}
		entrypointB58 = string(bytes.TrimSpace(entrypointFileData))
	}
	entrypointRaw := base58.Decode(entrypointB58)
	if len(entrypointRaw) == 0 {
		return nil, errors.New("could not decode base58 entrypoint data")
	}
	entrypoint, err := protobuf.EntryPointFromBytes(entrypointRaw)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal entrypoint data: %w", err)
	}

	return entrypoint, nil
}

func getMainDS() (datastore.DS, error) {
	location := os.Getenv("CINODE_MAIN_DATASTORE")
	if location == "" {
		return datastore.InMemory(), nil
	}
	return datastore.FromLocation(location)
}

func getAdditionalDSs() ([]datastore.DS, error) {
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
		location := os.Getenv(envName)
		ds, err := datastore.FromLocation(location)
		if err != nil {
			return nil, fmt.Errorf("invalid datastore location '%s' from env var '%s': %w", location, envName, err)
		}
		additionalDSs = append(additionalDSs, ds)
	}

	return additionalDSs, nil
}

func setupCinodeProxy(
	mainDS datastore.DS,
	additionalDSs []datastore.DS,
	entrypoint *protobuf.Entrypoint,
) http.Handler {
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

	return handler
}
