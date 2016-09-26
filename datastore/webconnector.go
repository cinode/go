package datastore

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

func errServerConnection(err error) error {
	return errors.New("Connection error: " + err.Error())
}

type webConnectorStreamWrapper struct {
	source io.ReadCloser
}

func (w *webConnectorStreamWrapper) Read(b []byte) (int, error) {
	n, err := w.source.Read(b)
	if err == io.EOF {
		err2 := w.source.Close()
		if err2 != nil {
			return 0, err2
		}
	}
	return n, err
}

func (w *webConnectorStreamWrapper) Close() error {
	return nil
}

type webConnector struct {
	baseURL string
	client  *http.Client
}

// FromWeb returns Datastore implementation that connects to external url
func FromWeb(baseURL string, client *http.Client) DS {

	return &webConnector{
		baseURL: baseURL,
		client:  client,
	}

}

func (w *webConnector) Kind() string {
	return "Web"
}

func (w *webConnector) Open(name string) (io.ReadCloser, error) {
	res, err := w.client.Get(w.baseURL + name)
	if err != nil {
		return nil, err
	}
	err = w.errCheck(res)
	if err != nil {
		res.Body.Close()
		return nil, err
	}
	return hashValidatingReader(res.Body, name), nil
}

func (w *webConnector) SaveAutoNamed(r io.ReadCloser) (string, error) {
	res, err := w.client.Post(
		w.baseURL,
		"application/octet-stream",
		&webConnectorStreamWrapper{source: r},
	)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	name, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	err = w.errCheck(res)
	if err != nil {
		return "", err
	}
	return string(name), nil
}

func (w *webConnector) Save(name string, r io.ReadCloser) error {
	req, err := http.NewRequest(
		http.MethodPut,
		w.baseURL+name,
		&webConnectorStreamWrapper{source: r},
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()
	return w.errCheck(res)
}

func (w *webConnector) Exists(name string) (bool, error) {
	res, err := http.Head(w.baseURL + name)
	if err != nil {
		return false, err
	}
	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()
	err = w.errCheck(res)
	if err == ErrNotFound {
		return false, nil
	}
	if err == nil {
		return true, nil
	}
	return false, err
}

func (w *webConnector) Delete(name string) error {
	req, err := http.NewRequest(http.MethodDelete, w.baseURL+name, nil)
	if err != nil {
		return err
	}
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()
	return w.errCheck(res)
}

func (w *webConnector) errCheck(res *http.Response) error {
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode == http.StatusBadRequest {
		return ErrNameMismatch
	}
	if res.StatusCode >= 400 {
		return errServerConnection(fmt.Errorf(
			"Response status code: %v (%v)", res.StatusCode, res.Status))
	}
	return nil
}
