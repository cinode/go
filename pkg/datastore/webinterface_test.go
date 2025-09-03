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

package datastore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/datastore/testutils"
	"github.com/stretchr/testify/require"
)

func testServer(t *testing.T) string {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Test web interface and web connector
	server := httptest.NewServer(WebInterface(
		InMemory(),
		WebInterfaceOptionLogger(log),
	))
	t.Cleanup(func() { server.Close() })
	return server.URL + "/"
}

func testHTTPResponse(
	t *testing.T,
	method string,
	path string,
	data io.Reader,
	code int,
) {
	url := testServer(t)
	testHTTPResponseOwnServer(t, method, url+path, data, code)
}

func testHTTPResponseOwnServerContentType(
	t *testing.T,
	method string,
	url string,
	data io.Reader,
	contentType string,
	code int,
) {
	req, err := http.NewRequest(method, url, data)
	require.NoError(t, err)

	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, code, resp.StatusCode)
}

func testHTTPResponseOwnServer(
	t *testing.T,
	method string,
	url string,
	data io.Reader,
	code int,
) {
	testHTTPResponseOwnServerContentType(
		t,
		method,
		url,
		data,
		"application/octet-stream",
		code,
	)
}

func TestWebInterfaceInvalidMethod(t *testing.T) {
	testHTTPResponse(
		t,
		http.MethodOptions,
		"",
		nil,
		http.StatusMethodNotAllowed,
	)
}

func TestWebInterfaceGetQueryString(t *testing.T) {
	url := testServer(t)
	testHTTPResponseOwnServer(
		t,
		http.MethodPut,
		url+testutils.EmptyBlobNameStatic.String(),
		bytes.NewBuffer(nil),
		http.StatusOK,
	)
	testHTTPResponseOwnServer(
		t,
		http.MethodGet,
		url+testutils.EmptyBlobNameStatic.String()+"?param=value",
		nil,
		http.StatusBadRequest,
	)
}

func TestWebInterfacePutQueryString(t *testing.T) {
	testHTTPResponse(
		t,
		http.MethodPut,
		testutils.EmptyBlobNameStatic.String()+"?param=value",
		bytes.NewBuffer(nil),
		http.StatusBadRequest,
	)
	testHTTPResponse(
		t,
		http.MethodPut,
		testutils.EmptyBlobNameStatic.String(),
		bytes.NewBuffer(nil),
		http.StatusOK,
	)
}

func TestWebInterfaceHeadQueryString(t *testing.T) {
	url := testServer(t)
	testHTTPResponseOwnServer(
		t,
		http.MethodPut,
		url+testutils.EmptyBlobNameStatic.String(),
		bytes.NewBuffer(nil),
		http.StatusOK,
	)
	testHTTPResponseOwnServer(
		t,
		http.MethodHead,
		url+testutils.EmptyBlobNameStatic.String()+"?param=value",
		nil,
		http.StatusBadRequest,
	)
	testHTTPResponseOwnServer(
		t,
		http.MethodHead,
		url+testutils.EmptyBlobNameStatic.String(),
		nil,
		http.StatusOK,
	)
}

func TestWebInterfaceDeleteQueryString(t *testing.T) {
	url := testServer(t)
	testHTTPResponseOwnServer(
		t,
		http.MethodPut,
		url+testutils.EmptyBlobNameStatic.String(),
		bytes.NewBuffer(nil),
		http.StatusOK,
	)
	testHTTPResponseOwnServer(
		t,
		http.MethodDelete,
		url+testutils.EmptyBlobNameStatic.String()+"?param=value",
		nil,
		http.StatusBadRequest,
	)
	testHTTPResponseOwnServer(
		t,
		http.MethodDelete,
		url+testutils.EmptyBlobNameStatic.String(),
		nil,
		http.StatusOK,
	)
}

func TestWebIntefaceExistsFailure(t *testing.T) {
	server := httptest.NewServer(WebInterface(&datastore{
		s: &mockStore{
			fExists: func(ctx context.Context, name *common.BlobName) (bool, error) { return false, errors.New("fail") },
		},
	}))
	defer server.Close()

	testHTTPResponseOwnServer(
		t,
		http.MethodHead,
		server.URL+"/"+testutils.EmptyBlobNameStatic.String(),
		nil,
		http.StatusInternalServerError,
	)
}

func TestWebInterfaceMultipartSave(t *testing.T) {
	url := testServer(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_, err := writer.CreateFormFile("file", "file")
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	testHTTPResponseOwnServerContentType(
		t,
		http.MethodPut,
		url+testutils.EmptyBlobNameStatic.String(),
		body,
		writer.FormDataContentType(),
		http.StatusOK,
	)
}

func TestWebInterfaceMultipartNoDataSave(t *testing.T) {
	url := testServer(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	field, err := writer.CreateFormField("test")
	require.NoError(t, err)
	_, err = field.Write([]byte("test"))
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	testHTTPResponseOwnServerContentType(
		t,
		http.MethodPut,
		url+testutils.EmptyBlobNameStatic.String(),
		body,
		writer.FormDataContentType(),
		http.StatusBadRequest,
	)
}
