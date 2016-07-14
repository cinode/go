package cas

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	webInterfaceTestBlobName = "ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4"
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

func testHTTPResponseOwnServer(t *testing.T, method string, url string, data io.Reader, code int) {

	req, err := http.NewRequest(method, url, data)
	errPanic(err)
	resp, err := http.DefaultClient.Do(req)
	errPanic(err)
	defer resp.Body.Close()
	if resp.StatusCode != code {
		t.Fatalf("Incorrect status code %d (%s)", resp.StatusCode, resp.Status)
	}
}

func TestWebInterfaceInvalidMethod(t *testing.T) {
	testHTTPResponse(t, http.MethodOptions, "", nil, http.StatusMethodNotAllowed)
}

func TestWebInterfaceGetQueryString(t *testing.T) {
	url, d := testServer()
	defer d()
	testHTTPResponseOwnServer(t, http.MethodPut, url+webInterfaceTestBlobName, testData(), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodGet, url+webInterfaceTestBlobName+"?param=value", nil, http.StatusNotFound)
}

func TestWebInterfacePostQueryString(t *testing.T) {
	testHTTPResponse(t, http.MethodPost, "?param=value", testData(), http.StatusNotFound)
	testHTTPResponse(t, http.MethodPost, "", testData(), http.StatusOK)
}

func TestWebInterfacePutQueryString(t *testing.T) {
	testHTTPResponse(t, http.MethodPut, webInterfaceTestBlobName+"?param=value", testData(), http.StatusNotFound)
	testHTTPResponse(t, http.MethodPut, webInterfaceTestBlobName, testData(), http.StatusOK)
}

func TestWebInterfaceHeadQueryString(t *testing.T) {
	url, d := testServer()
	defer d()
	testHTTPResponseOwnServer(t, http.MethodPut, url+webInterfaceTestBlobName, testData(), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodHead, url+webInterfaceTestBlobName+"?param=value", nil, http.StatusNotFound)
	testHTTPResponseOwnServer(t, http.MethodHead, url+webInterfaceTestBlobName, nil, http.StatusOK)
}

func TestWebInterfaceDeleteQueryString(t *testing.T) {
	url, d := testServer()
	defer d()
	testHTTPResponseOwnServer(t, http.MethodPut, url+webInterfaceTestBlobName, testData(), http.StatusOK)
	testHTTPResponseOwnServer(t, http.MethodDelete, url+webInterfaceTestBlobName+"?param=value", nil, http.StatusNotFound)
	testHTTPResponseOwnServer(t, http.MethodDelete, url+webInterfaceTestBlobName, nil, http.StatusOK)
}

func TestWebInterfacePostNonRoot(t *testing.T) {
	testHTTPResponse(t, http.MethodPost, webInterfaceTestBlobName, testData(), http.StatusNotFound)
}

type errorOnExists struct {
	memory
}

func (a *errorOnExists) Exists(name string) (bool, error) {
	return false, errors.New("Error")
}

func TestWebIntefaceExistsFailure(t *testing.T) {
	server := httptest.NewServer(WebInterface(&errorOnExists{}))
	defer server.Close()
	testHTTPResponseOwnServer(t, http.MethodHead, server.URL+"/"+webInterfaceTestBlobName, nil, http.StatusInternalServerError)
}
