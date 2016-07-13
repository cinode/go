package cas

import (
	"io"
	"net/http"
)

// WebInterface provides simple web interface for given CAS
type webInterface struct {
	cas CAS
}

// WebInterface returns http handler representing web interface to given CAS
// instance
func WebInterface(cas CAS) http.Handler {
	return &webInterface{
		cas: cas,
	}
}

func (i *webInterface) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		i.serveGet(w, r)
	case http.MethodPut:
		i.servePut(w, r)
	case http.MethodPost:
		i.servePost(w, r)
	case http.MethodDelete:
		i.serveDelete(w, r)
	case http.MethodHead:
		i.serveHead(w, r)
	default:
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
	}
}

func (i *webInterface) getName(w http.ResponseWriter, r *http.Request) (string, bool) {

	// Don't allow url queries and require path to start with '/'
	if r.URL.Path[0] != '/' || r.URL.RawQuery != "" {
		http.NotFound(w, r)
		return "", false
	}

	return r.URL.Path[1:], true
}

func (i *webInterface) checkErr(err error, w http.ResponseWriter, r *http.Request) bool {
	if err == nil {
		return true
	}

	if err == ErrNotFound {
		http.NotFound(w, r)
		return false
	}

	if err == ErrNameMismatch {
		http.Error(w, "Name mismatch", http.StatusBadRequest)
		return false
	}

	http.Error(w, "Internal server error", http.StatusInternalServerError)
	return false
}

func (i *webInterface) sendName(name string, w http.ResponseWriter, r *http.Request) {
	// TODO: Support multiple result encodings
	w.Write([]byte(name))
}

func (i *webInterface) serveGet(w http.ResponseWriter, r *http.Request) {

	name, ok := i.getName(w, r)
	if !ok {
		return
	}

	blob, err := i.cas.Open(name)
	if !i.checkErr(err, w, r) {
		return
	}
	defer blob.Close()

	_, err = io.Copy(w, blob)
	if !i.checkErr(err, w, r) {
		return
	}

}

func (i *webInterface) servePost(w http.ResponseWriter, r *http.Request) {

	path, ok := i.getName(w, r)
	if !ok {
		return
	}

	// Posting allowed onto root only
	if path != "" {
		http.NotFound(w, r)
		return
	}

	name, err := i.cas.SaveAutoNamed(r.Body)
	if !i.checkErr(err, w, r) {
		return
	}

	i.sendName(name, w, r)
}

func (i *webInterface) servePut(w http.ResponseWriter, r *http.Request) {
	name, ok := i.getName(w, r)
	if !ok {
		return
	}

	err := i.cas.Save(name, r.Body)
	if !i.checkErr(err, w, r) {
		return
	}

	i.sendName(name, w, r)
}

func (i *webInterface) serveDelete(w http.ResponseWriter, r *http.Request) {

	name, ok := i.getName(w, r)
	if !ok {
		return
	}

	err := i.cas.Delete(name)
	if !i.checkErr(err, w, r) {
		return
	}

	i.sendName(name, w, r)
}

func (i *webInterface) serveHead(w http.ResponseWriter, r *http.Request) {
	name, ok := i.getName(w, r)
	if !ok {
		return
	}

	err := i.cas.Exists(name)
	if err != nil {
		if err == ErrNotFound {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
