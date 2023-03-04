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

package httpserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slog"
)

func TestCancelWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := RunGracefully(
		ctx,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ListenAddr(":0"),
	)
	require.NoError(t, err)
	require.Less(t, time.Since(start), time.Second)
}

func TestCancelWithSignal(t *testing.T) {
	signalFunc := getSignalFunc(t) // Some quirks needed since signals are not possible on Windows :facepalm:
	go func() {
		time.Sleep(10 * time.Millisecond)
		signalFunc()
	}()
	start := time.Now()
	err := RunGracefully(
		context.Background(),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ListenAddr(":0"),
	)
	require.NoError(t, err)
	require.Less(t, time.Since(start), time.Second)
}

func TestEnsureHandlerIsCalled(t *testing.T) {
	handlerCalled := false

	server, listener, err := startGracefully(
		context.Background(),
		cfg{
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
			}),
			listenAddr: ":0",
			log:        slog.Default(),
		},
	)
	require.NoError(t, err)

	resp, err := http.Get(
		fmt.Sprintf("http://localhost:%d/", listener.Addr().(*net.TCPAddr).Port),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	err = server.Close()
	require.NoError(t, err)

	require.True(t, handlerCalled)
}

func TestFailOnInvalidListenAddr(t *testing.T) {
	err := RunGracefully(
		context.Background(),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ListenAddr("not-a-listen-address"),
	)
	require.IsType(t, &net.OpError{}, err)
}

func TestOptions(t *testing.T) {
	t.Run("ListenPort", func(t *testing.T) {
		cfg := cfg{}
		ListenPort(54321)(&cfg)
		require.Equal(t, ":54321", cfg.listenAddr)
	})
	t.Run("ListenAddr", func(t *testing.T) {
		cfg := cfg{}
		ListenAddr(":12345")(&cfg)
		require.Equal(t, ":12345", cfg.listenAddr)
	})
	t.Run("Logger", func(t *testing.T) {
		log := slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil)))
		cfg := cfg{}
		Logger(log)(&cfg)
		require.Equal(t, log, cfg.log)
	})
}
