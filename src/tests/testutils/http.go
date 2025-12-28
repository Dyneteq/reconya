package testutils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type TestServer struct {
	*httptest.Server
	t *testing.T
}

func NewTestServer(t *testing.T, handler http.Handler) *TestServer {
	server := httptest.NewServer(handler)
	return &TestServer{
		Server: server,
		t:      t,
	}
}

func (ts *TestServer) GET(path string) *http.Response {
	resp, err := http.Get(ts.URL + path)
	require.NoError(ts.t, err)
	return resp
}

func (ts *TestServer) POST(path string, body interface{}) *http.Response {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(ts.t, err)
		bodyReader = bytes.NewReader(jsonBody)
	}

	resp, err := http.Post(ts.URL+path, "application/json", bodyReader)
	require.NoError(ts.t, err)
	return resp
}

func (ts *TestServer) PUT(path string, body interface{}) *http.Response {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(ts.t, err)
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest("PUT", ts.URL+path, bodyReader)
	require.NoError(ts.t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(ts.t, err)
	return resp
}

func (ts *TestServer) DELETE(path string) *http.Response {
	req, err := http.NewRequest("DELETE", ts.URL+path, nil)
	require.NoError(ts.t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(ts.t, err)
	return resp
}

func AssertJSONResponse(t *testing.T, resp *http.Response, expectedStatus int, target interface{}) {
	require.Equal(t, expectedStatus, resp.StatusCode)

	if target != nil {
		defer resp.Body.Close()
		err := json.NewDecoder(resp.Body).Decode(target)
		require.NoError(t, err)
	}
}

func AssertErrorResponse(t *testing.T, resp *http.Response, expectedStatus int, expectedMessage string) {
	require.Equal(t, expectedStatus, resp.StatusCode)

	defer resp.Body.Close()
	var errorResp map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&errorResp)
	require.NoError(t, err)

	if expectedMessage != "" {
		require.Contains(t, errorResp["error"], expectedMessage)
	}
}
