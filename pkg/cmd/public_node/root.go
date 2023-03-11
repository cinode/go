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
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/internal/utilities/httpserver"
)

func Execute(ctx context.Context) error {
	return executeWithConfig(ctx, getConfig())
}

func executeWithConfig(ctx context.Context, cfg config) error {
	handler, err := buildHttpHandler(cfg)
	if err != nil {
		return err
	}

	log.Printf("Listening on http://localhost:%d", cfg.port)
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
	return datastore.WebInterface(ds), nil
}

type config struct {
	mainDSLocation        string
	additionalDSLocations []string
	port                  int
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

	return cfg
}
