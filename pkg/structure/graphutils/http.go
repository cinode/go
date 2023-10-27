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

package graphutils

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cinode/go/pkg/structure/graph"
	"golang.org/x/exp/slog"
)

type HTTPHandler struct {
	FS        graph.CinodeFS
	IndexFile string
	Log       *slog.Logger
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := h.Log.With(
		slog.String("RemoteAddr", r.RemoteAddr),
		slog.String("URL", r.URL.String()),
		slog.String("Method", r.Method),
	)

	if r.Method != "GET" {
		log.Error("Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	if strings.HasSuffix(path, "/") {
		path += h.IndexFile
	}

	pathList := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i := range pathList {
		p, err := url.PathUnescape(pathList[i])
		if err != nil {
			log.WarnCtx(r.Context(),
				"Incorrect request path",
				"err", err,
			)
			http.Error(w,
				fmt.Sprintf("Could not unescape URL path segment: %s", err.Error()),
				http.StatusBadRequest,
			)
			return
		}
		pathList[i] = p
	}

	fileEP, err := h.FS.FindEntry(r.Context(), pathList)
	switch {
	case errors.Is(err, graph.ErrEntryNotFound):
		log.Warn("Not found")
		http.NotFound(w, r)
		return
	case err != nil:
		log.Error("Error serving request", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if fileEP.IsDir() {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusPermanentRedirect)
		return
	}

	w.Header().Set("Content-Type", fileEP.MimeType())
	rc, err := h.FS.OpenEntrypointData(r.Context(), fileEP)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("%s: %v", http.StatusText(http.StatusInternalServerError), err),
			http.StatusInternalServerError,
		)
		h.Log.Error("Error opening file", "err", err)
		return
	}
	defer rc.Close()

	_, err = io.Copy(w, rc)
	if err != nil {
		h.Log.Error("Error sending file", "err", err)
	}
}
