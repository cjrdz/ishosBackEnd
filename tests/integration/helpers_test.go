package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func doJSONRequest(t *testing.T, client *http.Client, method, url string, body any, token string) (*http.Response, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	return resp, respBody
}

func requireStatus(t *testing.T, resp *http.Response, expected int, body []byte) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Fatalf("expected status %d, got %d, body=%s", expected, resp.StatusCode, string(body))
	}
}
