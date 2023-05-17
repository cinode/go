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
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/utilities/httpserver"
	"github.com/cinode/go/pkg/protobuf"
	"github.com/cinode/go/pkg/structure"
	"github.com/jbenet/go-base58"
	"golang.org/x/exp/slog"
)

func Execute(ctx context.Context) error {
	cfg, err := getConfig()
	if err != nil {
		return err
	}
	return executeWithConfig(ctx, cfg)
}

func executeWithConfig(ctx context.Context, cfg *config) error {
	mainDS, err := datastore.FromLocation(cfg.mainDSLocation)
	if err != nil {
		return fmt.Errorf("could not create main datastore: %w", err)
	}

	additionalDSs := []datastore.DS{}
	for _, loc := range cfg.additionalDSLocations {
		ds, err := datastore.FromLocation(loc)
		if err != nil {
			return fmt.Errorf("could not create additional datastores: %w", err)
		}
		additionalDSs = append(additionalDSs, ds)
	}

	entrypointRaw := base58.Decode(cfg.entrypoint)
	if len(entrypointRaw) == 0 {
		return errors.New("could not decode base58 entrypoint data")
	}
	entrypoint, err := protobuf.EntryPointFromBytes(entrypointRaw)
	if err != nil {
		return fmt.Errorf("could not unmarshal entrypoint data: %w", err)
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

	handler := setupCinodeProxy(mainDS, additionalDSs, entrypoint)
	return httpserver.RunGracefully(ctx,
		handler,
		httpserver.ListenPort(cfg.port),
		httpserver.Logger(log),
	)
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

	return &structure.HTTPHandler{
		FS: &fs,
	}
}

type config struct {
	entrypoint            string
	mainDSLocation        string
	additionalDSLocations []string
	port                  int
}

func getConfig() (*config, error) {
	cfg := config{}

	entrypoint, found := os.LookupEnv("CINODE_ENTRYPOINT")
	if !found {
		entrypointFile, found := os.LookupEnv("CINODE_ENTRYPOINT_FILE")
		if !found {
			return nil, errors.New("missing CINODE_ENTRYPOINT or CINODE_ENTRYPOINT_FILE env var")
		}
		entrypointFileData, err := os.ReadFile(entrypointFile)
		if err != nil {
			return nil, fmt.Errorf("could not read entrypoint file at '%s': %w", entrypointFile, err)
		}
		entrypoint = string(bytes.TrimSpace(entrypointFileData))
	}
	cfg.entrypoint = entrypoint

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

	return &cfg, nil
}
