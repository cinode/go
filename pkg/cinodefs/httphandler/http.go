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

package httphandler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cinode/go/pkg/cinodefs"
	"golang.org/x/exp/slog"
)

type Handler struct {
	FS        cinodefs.FS
	IndexFile string
	Log       *slog.Logger
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := h.Log.With(
		slog.String("RemoteAddr", r.RemoteAddr),
		slog.String("URL", r.URL.String()),
		slog.String("Method", r.Method),
	)

	switch r.Method {
	case "GET":
		h.serveGet(w, r, log)
		return
	default:
		log.Error("Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (h *Handler) serveGet(w http.ResponseWriter, r *http.Request, log *slog.Logger) {
	path := r.URL.Path
	if strings.HasSuffix(path, "/") {
		path += h.IndexFile
	}

	pathList := strings.Split(strings.TrimPrefix(path, "/"), "/")
	fileEP, err := h.FS.FindEntry(r.Context(), pathList)
	switch {
	case errors.Is(err, cinodefs.ErrEntryNotFound),
		errors.Is(err, cinodefs.ErrNotADirectory):
		log.Warn("Not found")
		http.NotFound(w, r)
		return
	case errors.Is(err, cinodefs.ErrModifiedDirectory):
		// Can't get the entrypoint, but since it's a directory
		// (only with unsaved changes), redirect to the directory itself
		// that will in the end load the index file if present.
		http.Redirect(w, r, r.URL.Path+"/", http.StatusTemporaryRedirect)
		return
	case h.handleHttpError(err, w, log, "Error finding entrypoint"):
		return
	}

	if fileEP.IsDir() {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusTemporaryRedirect)
		return
	}

	rc, err := h.FS.OpenEntrypointData(r.Context(), fileEP)
	if h.handleHttpError(err, w, log, "Error opening file") {
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", fileEP.MimeType())
	_, err = io.Copy(w, rc)
	h.handleHttpError(err, w, log, "Error sending file")
}

func (h *Handler) handleHttpError(err error, w http.ResponseWriter, log *slog.Logger, logMsg string) bool {
	if err != nil {
		log.Error(logMsg, "err", err)
		http.Error(w,
			fmt.Sprintf("%s: %v", http.StatusText(http.StatusInternalServerError), err),
			http.StatusInternalServerError,
		)
		return true
	}
	return false
}
