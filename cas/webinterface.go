package cas

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
)

var (
	errNoData = errors.New("No upload data")
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
	i.checkErr(err, w, r)
}

type partReader struct {
	p *multipart.Part
	b io.Closer
}

func (r *partReader) Read(b []byte) (int, error) {
	return r.p.Read(b)
}

func (r *partReader) Close() error {
	err1 := r.p.Close()
	err2 := r.b.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (i *webInterface) getUploadReader(r *http.Request) (io.ReadCloser, error) {

	mpr, err := r.MultipartReader()
	if err == http.ErrNotMultipart {
		// Not multipart, read raw body data
		return r.Body, nil
	}
	if err != nil {
		return nil, err
	}

	for {
		// Get next part of the upload
		part, err := mpr.NextPart()
		if err == io.EOF {
			return nil, errNoData
		}
		if err != nil {
			return nil, err
		}

		// Search for first file input
		fn := part.FileName()
		if fn != "" {
			return &partReader{
				p: part,
				b: r.Body,
			}, nil
		}
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

	reader, err := i.getUploadReader(r)
	if !i.checkErr(err, w, r) {
		return
	}

	name, err := i.cas.SaveAutoNamed(reader)
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

	reader, err := i.getUploadReader(r)
	if !i.checkErr(err, w, r) {
		return
	}

	err = i.cas.Save(name, reader)
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

	exists, err := i.cas.Exists(name)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.NotFound(w, r)
	}
}
