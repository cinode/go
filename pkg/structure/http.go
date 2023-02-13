/*
Copyright © 2022 Bartłomiej Święcki (byo)

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

package structure

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

type HTTPHandler struct {
	FS *CinodeFS
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")

	fileEP, err := h.FS.FindEntrypoint(r.Context(), path)
	switch {
	case errors.Is(err, ErrNotFound):
		http.NotFound(w, r)
		return
	case err != nil:
		log.Println("Error serving request:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if fileEP.MimeType == CinodeDirMimeType {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusPermanentRedirect)
		return
	}

	w.Header().Set("Content-Type", fileEP.GetMimeType())
	rc, err := h.FS.OpenContent(r.Context(), fileEP)
	if err != nil {
		log.Printf("Error sending file: %v", err)
	}
	defer rc.Close()

	_, err = io.Copy(w, rc)
	if err != nil {
		log.Printf("Error sending file: %v", err)
	}

}
