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

package httphandler

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/cinode/go/pkg/cinodefs"
)

type Handler struct {
	Log       *slog.Logger
	FS        cinodefs.FS
	IndexFile string
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

// sanitizeRedirectPath ensures redirect targets do not lead to open redirects.
func sanitizeRedirectPath(p string) string {
	if len(p) > 1 && p[0] == '/' && p[1] != '/' && p[1] != '\\' {
		return p
	}
	return "/"
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
		http.Redirect(w, r, sanitizeRedirectPath(r.URL.Path+"/"), http.StatusTemporaryRedirect)
		return
	case h.handleHTTPError(err, w, log, "Error finding entrypoint"):
		return
	}

	if fileEP.IsDir() {
		http.Redirect(w, r, sanitizeRedirectPath(r.URL.Path+"/"), http.StatusTemporaryRedirect)
		return
	}

	if h.handleEtag(w, r, fileEP, log) {
		// Client ETag matches, can optimize out the data
		return
	}

	rc, err := h.FS.OpenEntrypointData(r.Context(), fileEP)
	if h.handleHTTPError(err, w, log, "Error opening file") {
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", fileEP.MimeType())
	_, err = io.Copy(w, rc)
	h.handleHTTPError(err, w, log, "Error sending file")
}

func (h *Handler) handleHTTPError(err error, w http.ResponseWriter, log *slog.Logger, logMsg string) bool {
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

func (h *Handler) handleEtag(w http.ResponseWriter, r *http.Request, ep *cinodefs.Entrypoint, log *slog.Logger) bool {
	currentEtag := fmt.Sprintf("\"%X\"", sha256.Sum256(ep.Bytes()))

	if strings.Contains(r.Header.Get("If-None-Match"), currentEtag) {
		log.Debug("Valid ETag found, sending 304 Not Modified")
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	w.Header().Set("ETag", currentEtag)
	return false
}
