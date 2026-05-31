package internal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWaitForHealthcheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := WaitForHealthcheck(DeployHealthcheck{
		URL:             server.URL,
		ExpectedStatus:  http.StatusOK,
		Attempts:        1,
		IntervalSeconds: 1,
		TimeoutSeconds:  1,
	})
	if err != nil {
		t.Fatalf("WaitForHealthcheck returned error: %v", err)
	}
}

func TestWaitForHealthcheck_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	err := WaitForHealthcheck(DeployHealthcheck{
		URL:             server.URL,
		ExpectedStatus:  http.StatusOK,
		Attempts:        2,
		IntervalSeconds: 1,
		TimeoutSeconds:  1,
	})
	if err == nil {
		t.Fatal("WaitForHealthcheck should fail when the endpoint never becomes healthy")
	}
	if !strings.Contains(err.Error(), server.URL) || !strings.Contains(err.Error(), "attempts=2") || !strings.Contains(err.Error(), "status=503") {
		t.Fatalf("WaitForHealthcheck error should contain url/attempts/status, got: %v", err)
	}
}
