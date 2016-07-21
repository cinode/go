package cas

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer() (string, func()) {
	// Test web interface and web connector
	server := httptest.NewServer(WebInterface(InMemory()))
	return server.URL + "/", func() { server.Close() }
}

func testHTTPResponse(t *testing.T, method string, path string, data io.Reader, code int) {
	url, d := testServer()
	defer d()

	testHTTPResponseOwnServer(t, method, url+path, data, code)
}

func testHTTPResponseOwnServerContentType(t *testing.T, method string, url string,
	data io.Reader, contentType string, code int) {

	req, err := http.NewRequest(method, url, data)
	errPanic(err)
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	errPanic(err)
	defer resp.Body.Close()
	if resp.StatusCode != code {
		t.Fatalf("Incorrect status code %d (%s)", resp.StatusCode, resp.Status)
	}
}

func testHTTPResponseOwnServer(t *testing.T, method string, url string, data io.Reader, code int) {
	testHTTPResponseOwnServerContentType(t, method, url, data, "application/octet-stream", code)
}

func TestWebInterfaceInvalidMethod(t *testing.T) {
	testHTTPResponse(t, http.MethodOptions, "", nil, http.StatusMethodNotAllowed)
}

func TestWebInterfaceGetQueryString(t *testing.T) {
	url, d := testServer()
	defer d()
	testHTTPResponseOwnServer(t, http.MethodPut, url+emptyBlobName, emptyBlobReader(), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodGet, url+emptyBlobName+"?param=value", nil, http.StatusNotFound)
}

func TestWebInterfacePostQueryString(t *testing.T) {
	testHTTPResponse(t, http.MethodPost, "?param=value", emptyBlobReader(), http.StatusNotFound)
	testHTTPResponse(t, http.MethodPost, "", emptyBlobReader(), http.StatusOK)
}

func TestWebInterfacePutQueryString(t *testing.T) {
	testHTTPResponse(t, http.MethodPut, emptyBlobName+"?param=value", emptyBlobReader(), http.StatusNotFound)
	testHTTPResponse(t, http.MethodPut, emptyBlobName, emptyBlobReader(), http.StatusOK)
}

func TestWebInterfaceHeadQueryString(t *testing.T) {
	url, d := testServer()
	defer d()
	testHTTPResponseOwnServer(t, http.MethodPut, url+emptyBlobName, emptyBlobReader(), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodHead, url+emptyBlobName+"?param=value", nil, http.StatusNotFound)
	testHTTPResponseOwnServer(t, http.MethodHead, url+emptyBlobName, nil, http.StatusOK)
}

func TestWebInterfaceDeleteQueryString(t *testing.T) {
	url, d := testServer()
	defer d()
	testHTTPResponseOwnServer(t, http.MethodPut, url+emptyBlobName, emptyBlobReader(), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodDelete, url+emptyBlobName+"?param=value", nil, http.StatusNotFound)
	testHTTPResponseOwnServer(t, http.MethodDelete, url+emptyBlobName, nil, http.StatusOK)
}

func TestWebInterfacePostNonRoot(t *testing.T) {
	testHTTPResponse(t, http.MethodPost, emptyBlobName, emptyBlobReader(), http.StatusNotFound)
}

func TestWebIntefaceExistsFailure(t *testing.T) {
	server := httptest.NewServer(WebInterface(&errorOnExists{}))
	defer server.Close()
	testHTTPResponseOwnServer(t, http.MethodHead, server.URL+"/"+emptyBlobName, nil, http.StatusInternalServerError)
}

func TestWebInterfaceMultipartSave(t *testing.T) {
	url, d := testServer()
	defer d()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_, err := writer.CreateFormFile("file", "file")
	errPanic(err)
	writer.Close()

	testHTTPResponseOwnServerContentType(t, http.MethodPost, url, body, writer.FormDataContentType(), http.StatusOK)
}

func TestWebInterfaceMultipartNoDataSave(t *testing.T) {
	url, d := testServer()
	defer d()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	field, err := writer.CreateFormField("test")
	errPanic(err)
	field.Write([]byte("test"))
	writer.Close()

	testHTTPResponseOwnServerContentType(t, http.MethodPost, url, body, writer.FormDataContentType(), http.StatusBadRequest)
}
