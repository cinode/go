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

package public_node

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/utilities/httpserver"
	"golang.org/x/exp/slog"
)

func Execute(ctx context.Context) error {
	return executeWithConfig(ctx, getConfig())
}

func executeWithConfig(ctx context.Context, cfg config) error {
	handler, err := buildHttpHandler(cfg)
	if err != nil {
		return err
	}

	log := slog.Default()

	log.Info("Server listening for connections",
		"address", fmt.Sprintf("http://localhost:%d", cfg.port),
	)

	log.Info("System info",
		"goos", runtime.GOOS,
		"goarch", runtime.GOARCH,
		"compiler", runtime.Compiler,
		"cpus", runtime.NumCPU(),
	)

	return httpserver.RunGracefully(ctx,
		handler,
		httpserver.ListenPort(cfg.port),
	)
}

func buildHttpHandler(cfg config) (http.Handler, error) {
	mainDS, err := datastore.FromLocation(cfg.mainDSLocation)
	if err != nil {
		return nil, fmt.Errorf("could not create main datastore: %w", err)
	}

	additionalDSs := []datastore.DS{}
	for _, loc := range cfg.additionalDSLocations {
		ds, err := datastore.FromLocation(loc)
		if err != nil {
			return nil, fmt.Errorf("could not create additional datastores: %w", err)
		}
		additionalDSs = append(additionalDSs, ds)
	}

	ds := datastore.NewMultiSource(mainDS, time.Hour, additionalDSs...)
	handler := datastore.WebInterface(ds)

	if cfg.uploadUsername != "" || cfg.uploadPassword != "" {
		origHandler := handler
		expectedUsernameHash := sha256.Sum256([]byte(cfg.uploadUsername))
		expectedPasswordHash := sha256.Sum256([]byte(cfg.uploadPassword))
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead:
				// Auth not required, continue without auth check
			default:
				// Every other method requires token, this is preventive
				// since not all methods will be uploads, but it comes from the
				// secure-by-default approach.
				//
				// Also we're comparing hashes instead of their values.
				// This, due to properties of a hashing function, reduces attacks
				// based on side-channel information, including the length of the
				// token. The subtle.ConstantTimeCompare is not really needed here
				// but it does not do any harm.
				username, password, ok := r.BasicAuth()

				var validAuth int = 0
				if ok {
					validAuth = 1
				}

				usernameHash := sha256.Sum256([]byte(username))
				validAuth &= subtle.ConstantTimeCompare(
					expectedUsernameHash[:],
					usernameHash[:],
				)

				passwordHash := sha256.Sum256([]byte(password))
				validAuth &= subtle.ConstantTimeCompare(
					expectedPasswordHash[:],
					passwordHash[:],
				)

				if validAuth != 1 {
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
			origHandler.ServeHTTP(w, r)
		})
	}

	return handler, nil
}

type config struct {
	mainDSLocation        string
	additionalDSLocations []string
	port                  int

	uploadUsername string
	uploadPassword string
}

func getConfig() config {
	cfg := config{}

	cfg.mainDSLocation = os.Getenv("CINODE_MAIN_DATASTORE")
	if cfg.mainDSLocation == "" {
		cfg.mainDSLocation = "memory://"
	}

	additionalDSEnvNames := []string{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CINODE_ADDITIONAL_DATASTORE") {
			split := strings.SplitN(e, "=", 2)
			additionalDSEnvNames = append(additionalDSEnvNames, split[0])
		}
	}
	sort.Strings(additionalDSEnvNames)

	for _, envName := range additionalDSEnvNames {
		location := os.Getenv(envName)
		cfg.additionalDSLocations = append(cfg.additionalDSLocations, location)
	}

	cfg.port = 8080
	cfg.uploadUsername = os.Getenv("CINODE_UPLOAD_USERNAME")
	cfg.uploadPassword = os.Getenv("CINODE_UPLOAD_PASSWORD")

	return cfg
}
