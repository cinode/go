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
	"strconv"
	"strings"
	"time"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/cinodefs"
	"github.com/cinode/go/pkg/cinodefs/httphandler"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/cinode/go/pkg/utilities/httpserver"
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

	entrypoint, err := cinodefs.EntrypointFromString(cfg.entrypoint)
	if err != nil {
		return fmt.Errorf("could not parse entrypoint data: %w", err)
	}

	log := slog.Default()

	log.Info("Server listening for connections",
		"address", fmt.Sprintf("http://localhost:%d", cfg.port),
	)
	log.Info("Main datastore", "addr", cfg.mainDSLocation)
	log.Info("Additional datastores", "addrs", cfg.additionalDSLocations)

	log.Info("System info",
		"goos", runtime.GOOS,
		"goarch", runtime.GOARCH,
		"compiler", runtime.Compiler,
		"cpus", runtime.NumCPU(),
	)

	handler := setupCinodeProxy(ctx, mainDS, additionalDSs, entrypoint)

	return httpserver.RunGracefully(ctx,
		handler,
		httpserver.ListenPort(cfg.port),
		httpserver.Logger(log),
	)
}

func setupCinodeProxy(
	ctx context.Context,
	mainDS datastore.DS,
	additionalDSs []datastore.DS,
	entrypoint *cinodefs.Entrypoint,
) http.Handler {
	fs := golang.Must(cinodefs.New(
		ctx,
		blenc.FromDatastore(
			datastore.NewMultiSource(
				mainDS,
				time.Hour,
				additionalDSs...,
			),
		),
		cinodefs.RootEntrypoint(entrypoint),
		cinodefs.MaxLinkRedirects(10),
	))

	return &httphandler.Handler{
		FS:        fs,
		IndexFile: "index.html",
		Log:       slog.Default(),
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

	port := os.Getenv("CINODE_LISTEN_PORT")
	if port == "" {
		cfg.port = 8080
	} else {
		portNum, err := strconv.Atoi(port)
		if err == nil && (portNum < 0 || portNum > 65535) {
			err = fmt.Errorf("not in range 0..65535")
		}
		if err != nil {
			return nil, fmt.Errorf("invalid listen port %s: %w", port, err)
		}
		cfg.port = portNum
	}

	return &cfg, nil
}
