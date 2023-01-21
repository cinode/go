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

package datastore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/cinode/go/pkg/internal/utilities/validatingreader"
)

var (
	ErrWebConnectionError = errors.New("connection error")
)

type webConnector struct {
	baseURL          string
	client           *http.Client
	customizeRequest func(*http.Request) error
}

var _ DS = (*webConnector)(nil)

type webConnectorOption func(*webConnector)

func WebOptionHttpClient(client *http.Client) webConnectorOption {
	return func(wc *webConnector) { wc.client = client }
}

func WebOptionCustomizeRequest(f func(*http.Request) error) webConnectorOption {
	return func(wc *webConnector) { wc.customizeRequest = f }
}

// FromWeb returns Datastore implementation that connects to external url
func FromWeb(baseURL string, options ...webConnectorOption) (DS, error) {
	_, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	ret := &webConnector{
		baseURL:          baseURL,
		client:           http.DefaultClient,
		customizeRequest: func(r *http.Request) error { return nil },
	}

	for _, o := range options {
		o(ret)
	}

	return ret, nil
}

func (w *webConnector) Kind() string {
	return "Web"
}

func (w *webConnector) Open(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	switch name.Type() {
	case blobtypes.Static:
		return w.openStatic(ctx, name)
	case blobtypes.DynamicLink:
		return w.openDynamicLink(ctx, name)
	default:
		return nil, blobtypes.ErrUnknownBlobType
	}
}

func (w *webConnector) openStatic(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		w.baseURL+name.String(),
		nil,
	)
	if err != nil {
		return nil, err
	}

	res, err := w.do(req)
	if err != nil {
		return nil, err
	}

	err = w.errCheck(res)
	if err != nil {
		res.Body.Close()
		return nil, err
	}

	return validatingreader.NewHashValidation(res.Body, sha256.New(), name.Hash(), blobtypes.ErrValidationFailed), nil
}

func (w *webConnector) openDynamicLink(ctx context.Context, name common.BlobName) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		w.baseURL+name.String(),
		nil,
	)
	if err != nil {
		return nil, err
	}

	res, err := w.do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	err = w.errCheck(res)
	if err != nil {
		return nil, err
	}

	buff := bytes.NewBuffer(nil)
	_, err = dynamiclink.FromReader(name, io.TeeReader(res.Body, buff))
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(buff.Bytes())), nil
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
	res, err := w.do(req)
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
	res, err := w.do(req)
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

	res, err := w.do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return w.errCheck(res)
}

func (w *webConnector) do(req *http.Request) (*http.Response, error) {
	err := w.customizeRequest(req)
	if err != nil {
		return nil, err
	}

	return w.client.Do(req)
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
