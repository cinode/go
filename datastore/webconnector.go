package datastore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cinode/go/common"
	"github.com/cinode/go/internal/blobtypes/propagation"
)

var (
	ErrWebConnectionError = errors.New("connection error")
)

type webConnector struct {
	baseURL string
	client  *http.Client
}

var _ DS = (*webConnector)(nil)

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

func (w *webConnector) Read(ctx context.Context, name common.BlobName, output io.Writer) error {
	handler, err := propagation.HandlerForType(name.Type())
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		w.baseURL+name.String(),
		nil,
	)
	if err != nil {
		return err
	}

	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	err = w.errCheck(res)
	if err != nil {
		return err
	}

	return handler.Validate(
		name.Hash(),
		io.TeeReader(res.Body, output),
	)
}

func (w *webConnector) Update(ctx context.Context, name common.BlobName, r io.Reader) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		w.baseURL+name.String(),
		r,
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/json")
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return w.errCheck(res)
}

func (w *webConnector) Exists(ctx context.Context, name common.BlobName) (bool, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodHead,
		w.baseURL+name.String(),
		nil,
	)
	if err != nil {
		return false, err
	}
	res, err := w.client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	err = w.errCheck(res)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}

	if err == nil {
		return true, nil
	}
	return false, err
}

func (w *webConnector) Delete(ctx context.Context, name common.BlobName) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		w.baseURL+name.String(),
		nil,
	)
	if err != nil {
		return err
	}

	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return w.errCheck(res)
}

func (w *webConnector) errCheck(res *http.Response) error {
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode == http.StatusBadRequest {
		msg := webErrResponse{}
		err := json.NewDecoder(res.Body).Decode(&msg)
		if err == nil {
			err := webErrFromCode(msg.Code)
			if err != nil {
				return err
			}
			return fmt.Errorf(
				"%w: response status code: %v (%v), error code: %v, error message: %v",
				ErrWebConnectionError,
				res.StatusCode,
				res.Status,
				msg.Code,
				msg.Message,
			)
		}
		// Fallthrough to code below if can't decode json error
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf(
			"%w: response status code: %v (%v)",
			ErrWebConnectionError,
			res.StatusCode,
			res.Status,
		)
	}
	return nil
}
