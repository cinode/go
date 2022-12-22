package datastore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cinode/go/pkg/common"
	"github.com/stretchr/testify/require"
)

// import (
// 	"bytes"
// 	"io"
// 	"mime/multipart"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"
// )

func testServer(t *testing.T) string {
	// Test web interface and web connector
	server := httptest.NewServer(WebInterface(InMemory()))
	t.Cleanup(func() { server.Close() })
	return server.URL + "/"
}

func testHTTPResponse(t *testing.T, method string, path string, data io.Reader, code int) {
	url := testServer(t)
	testHTTPResponseOwnServer(t, method, url+path, data, code)
}

func testHTTPResponseOwnServerContentType(t *testing.T, method string, url string, data io.Reader, contentType string, code int) {
	req, err := http.NewRequest(method, url, data)
	require.NoError(t, err)

	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, code, resp.StatusCode)
}

func testHTTPResponseOwnServer(t *testing.T, method string, url string, data io.Reader, code int) {
	testHTTPResponseOwnServerContentType(t, method, url, data, "application/octet-stream", code)
}

func TestWebInterfaceInvalidMethod(t *testing.T) {
	testHTTPResponse(t, http.MethodOptions, "", nil, http.StatusMethodNotAllowed)
}

func TestWebInterfaceGetQueryString(t *testing.T) {
	url := testServer(t)
	testHTTPResponseOwnServer(t, http.MethodPut, url+emptyBlobName.String(), bytes.NewBuffer(nil), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodGet, url+emptyBlobName.String()+"?param=value", nil, http.StatusBadRequest)
}

func TestWebInterfacePutQueryString(t *testing.T) {
	testHTTPResponse(t, http.MethodPut, emptyBlobName.String()+"?param=value", bytes.NewBuffer(nil), http.StatusBadRequest)
	testHTTPResponse(t, http.MethodPut, emptyBlobName.String(), bytes.NewBuffer(nil), http.StatusOK)
}

func TestWebInterfaceHeadQueryString(t *testing.T) {
	url := testServer(t)
	testHTTPResponseOwnServer(t, http.MethodPut, url+emptyBlobName.String(), bytes.NewBuffer(nil), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodHead, url+emptyBlobName.String()+"?param=value", nil, http.StatusBadRequest)
	testHTTPResponseOwnServer(t, http.MethodHead, url+emptyBlobName.String(), nil, http.StatusOK)
}

func TestWebInterfaceDeleteQueryString(t *testing.T) {
	url := testServer(t)
	testHTTPResponseOwnServer(t, http.MethodPut, url+emptyBlobName.String(), bytes.NewBuffer(nil), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodDelete, url+emptyBlobName.String()+"?param=value", nil, http.StatusBadRequest)
	testHTTPResponseOwnServer(t, http.MethodDelete, url+emptyBlobName.String(), nil, http.StatusOK)
}

func TestWebIntefaceExistsFailure(t *testing.T) {
	server := httptest.NewServer(WebInterface(&datastore{
		s: &mockStore{
			fExists: func(ctx context.Context, name common.BlobName) (bool, error) { return false, errors.New("fail") },
		},
	}))
	defer server.Close()

	testHTTPResponseOwnServer(t, http.MethodHead, server.URL+"/"+emptyBlobName.String(), nil, http.StatusInternalServerError)
}

func TestWebInterfaceMultipartSave(t *testing.T) {
	url := testServer(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_, err := writer.CreateFormFile("file", "file")
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	testHTTPResponseOwnServerContentType(t, http.MethodPut, url+emptyBlobName.String(), body, writer.FormDataContentType(), http.StatusOK)
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

	testHTTPResponseOwnServerContentType(t, http.MethodPut, url+emptyBlobName.String(), body, writer.FormDataContentType(), http.StatusBadRequest)
}
