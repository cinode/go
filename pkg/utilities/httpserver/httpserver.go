/*
Copyright © 2025 Bartłomiej Święcki (byo)

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

package httpserver

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

type cfg struct {
	handler    http.Handler
	listenAddr string
	log        *slog.Logger
}

type Option func(c *cfg)

func ListenPort(port int) Option          { return func(c *cfg) { c.listenAddr = ":" + strconv.Itoa(port) } }
func ListenAddr(listenAddr string) Option { return func(c *cfg) { c.listenAddr = listenAddr } }
func Logger(log *slog.Logger) Option      { return func(c *cfg) { c.log = log } }

func RunGracefully(ctx context.Context, handler http.Handler, opt ...Option) error {
	cfg := cfg{
		handler:    handler,
		listenAddr: ":http",
		log:        slog.Default(),
	}

	for _, o := range opt {
		o(&cfg)
	}

	server, _, err := startGracefully(ctx, cfg)
	if err != nil {
		return err
	}

	return endGracefully(ctx, server, cfg)
}

func startGracefully(ctx context.Context, cfg cfg) (*http.Server, net.Listener, error) {
	cfg.log.Info("Starting http server", "listenAddr", cfg.listenAddr)
	listener, err := net.Listen("tcp", cfg.listenAddr)
	if err != nil {
		return nil, nil, err
	}

	server := &http.Server{
		Addr: cfg.listenAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg.log.Info(
				"http request",
				slog.Group("req",
					slog.String("remoteAddr", r.RemoteAddr),
					slog.String("method", r.Method),
					slog.String("url", r.URL.String()),
				),
			)
			cfg.handler.ServeHTTP(w, r)
		}),
	}

	go server.Serve(listener)

	return server, listener, nil
}

func endGracefully(ctx context.Context, server *http.Server, cfg cfg) error {
	ctx, _ = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	<-ctx.Done()
	cfg.log.Info("Shutting down")

	// TODO: More graceful way?
	return server.Close()
}

func FailResponseOnError(w http.ResponseWriter, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
