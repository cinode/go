package datastore

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
)

var (
	errNoData = errors.New("no upload data")
)

// WebInterface provides simple web interface for given Datastore
type webInterface struct {
	ds DS
}

// WebInterface returns http handler representing web interface to given
// Datastore instance
func WebInterface(ds DS) http.Handler {
	return &webInterface{
		ds: ds,
	}
}

func (i *webInterface) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		i.serveGet(w, r)
	case http.MethodPut:
		i.servePut(w, r)
	case http.MethodDelete:
		i.serveDelete(w, r)
	case http.MethodHead:
		i.serveHead(w, r)
	default:
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
	}
}

func (i *webInterface) getName(w http.ResponseWriter, r *http.Request) (BlobName, error) {
	// Don't allow url queries and require path to start with '/'
	if r.URL.Path[0] != '/' || r.URL.RawQuery != "" {
		return nil, ErrInvalidBlobName
	}

	bn, err := BlobNameFromString(r.URL.Path[1:])
	if err != nil {
		return nil, err
	}

	return bn, nil
}

func (i *webInterface) sendName(name BlobName, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(&webNameResponse{
		Name: name.String(),
	})
}

func (i *webInterface) sendError(w http.ResponseWriter, httpCode int, code string, message string) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(&webErrResponse{
		Code:    code,
		Message: message,
	})
}
func (i *webInterface) checkErr(err error, w http.ResponseWriter, r *http.Request) bool {
	if err == nil {
		return true
	}

	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return false
	}

	code := webErrToCode(err)
	if code != "" {
		i.sendError(w, http.StatusBadRequest, code, err.Error())
		return false
	}

	http.Error(w, "Internal server error", http.StatusInternalServerError)
	return false
}

func (i *webInterface) serveGet(w http.ResponseWriter, r *http.Request) {
	name, err := i.getName(w, r)
	if !i.checkErr(err, w, r) {
		return
	}

	err = i.ds.Read(name, w)
	if !i.checkErr(err, w, r) {
		return
	}
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

func (i *webInterface) servePut(w http.ResponseWriter, r *http.Request) {
	name, err := i.getName(w, r)
	if !i.checkErr(err, w, r) {
		return
	}

	reader, err := i.getUploadReader(r)
	if !i.checkErr(err, w, r) {
		return
	}
	defer reader.Close()

	err = i.ds.Update(name, reader)
	if !i.checkErr(err, w, r) {
		return
	}

	i.sendName(name, w, r)
}

func (i *webInterface) serveDelete(w http.ResponseWriter, r *http.Request) {

	name, err := i.getName(w, r)
	if !i.checkErr(err, w, r) {
		return
	}

	err = i.ds.Delete(name)
	if !i.checkErr(err, w, r) {
		return
	}

	i.sendName(name, w, r)
}

func (i *webInterface) serveHead(w http.ResponseWriter, r *http.Request) {
	name, err := i.getName(w, r)
	if !i.checkErr(err, w, r) {
		return
	}

	exists, err := i.ds.Exists(name)
	if !i.checkErr(err, w, r) {
		return
	}

	if !exists {
		http.NotFound(w, r)
	}
}
