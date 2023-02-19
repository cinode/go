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
	"context"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunGracefully(t *testing.T) {
	t.Run("cancel with context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		err := RunGracefully(ctx, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), ":0")
		require.NoError(t, err)
		require.Less(t, time.Since(start), time.Second)
	})

	t.Run("cancel with signal", func(t *testing.T) {
		go func() {
			time.Sleep(10 * time.Millisecond)
			syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		}()
		start := time.Now()
		err := RunGracefully(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), ":0")
		require.NoError(t, err)
		require.Less(t, time.Since(start), time.Second)
	})
}
