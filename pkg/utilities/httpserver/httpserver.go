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
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type cfg struct {
	log                     *slog.Logger
	handler                 http.Handler
	listenAddr              string
	gracefulShutdownTimeout time.Duration
}

type Option func(c *cfg)

func ListenPort(port int) Option          { return func(c *cfg) { c.listenAddr = ":" + strconv.Itoa(port) } }
func ListenAddr(listenAddr string) Option { return func(c *cfg) { c.listenAddr = listenAddr } }
func Logger(log *slog.Logger) Option      { return func(c *cfg) { c.log = log } }

func RunGracefully(ctx context.Context, handler http.Handler, opt ...Option) error {
	c := cfg{
		handler:                 handler,
		listenAddr:              ":http",
		log:                     slog.Default(),
		gracefulShutdownTimeout: 5 * time.Second,
	}

	for _, o := range opt {
		o(&c)
	}

	c.log.Info("Starting http server", "listenAddr", c.listenAddr)

	listener, err := net.Listen("tcp", c.listenAddr)
	if err != nil {
		return err
	}

	ctx, signalCtxCancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer signalCtxCancel()

	return runUntilContextNotDone(ctx, c, listener)
}

func runUntilContextNotDone(ctx context.Context, cfg cfg, listener net.Listener) error {
	server := &http.Server{
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
		ReadHeaderTimeout: 5 * time.Second, // Prevent Slowloris attacks
	}

	wg := sync.WaitGroup{}
	wg.Go(func() {
		<-ctx.Done()

		cfg.log.Info("Shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.gracefulShutdownTimeout)
		defer cancel()

		// TODO: More graceful way?
		if err := server.Shutdown(shutdownCtx); err != nil {
			cfg.log.Error("Failed to shutdown gracefully")
			server.Close()
		} else {
			cfg.log.Info("Shutdown complete")
		}
	})
	defer wg.Wait()

	err := server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}

	return err
}

func FailResponseOnError(w http.ResponseWriter, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
