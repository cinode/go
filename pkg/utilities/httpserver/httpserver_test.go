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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCancelWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
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
		t.Context(),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ListenAddr(":0"),
	)
	require.NoError(t, err)
	require.Less(t, time.Since(start), time.Second)
}

func TestEnsureHandlerIsCalled(t *testing.T) {
	handlerCalled := false

	server, listener, err := startGracefully(cfg{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		}),
		listenAddr: ":0",
		log:        slog.Default(),
	})
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
		t.Context(),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ListenAddr("not-a-listen-address"),
	)
	require.IsType(t, &net.OpError{}, err)
}

func TestOptions(t *testing.T) {
	t.Run("ListenPort", func(t *testing.T) {
		config := cfg{}
		ListenPort(54321)(&config)
		require.Equal(t, ":54321", config.listenAddr)
	})
	t.Run("ListenAddr", func(t *testing.T) {
		config := cfg{}
		ListenAddr(":12345")(&config)
		require.Equal(t, ":12345", config.listenAddr)
	})
	t.Run("Logger", func(t *testing.T) {
		log := slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil), nil))
		config := cfg{}
		Logger(log)(&config)
		require.Equal(t, log, config.log)
	})
}

func TestFailResponseOnError(t *testing.T) {
	var triggeredError error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FailResponseOnError(w, triggeredError)
	}))

	resp, err := http.Get(server.URL)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	triggeredError = errors.New("error")

	resp, err = http.Get(server.URL)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
