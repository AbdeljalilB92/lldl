package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestHandlePostToken(t *testing.T) {
	var (
		mu          sync.Mutex
		capturedTok string
		called      bool
	)
	emit := func(token string) {
		mu.Lock()
		capturedTok = token
		called = true
		mu.Unlock()
	}

	ts := NewTokenServer(emit)

	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
		wantCalled bool
		wantToken  string
	}{
		{
			name:       "valid token",
			method:     http.MethodPost,
			body:       `{"token":"a1b2c3d4e5f6g7h8i9j0"}`,
			wantStatus: http.StatusNoContent,
			wantCalled: true,
			wantToken:  "a1b2c3d4e5f6g7h8i9j0",
		},
		{
			name:       "empty token",
			method:     http.MethodPost,
			body:       `{"token":""}`,
			wantStatus: http.StatusBadRequest,
			wantCalled: false,
		},
		{
			name:       "short token",
			method:     http.MethodPost,
			body:       `{"token":"abc"}`,
			wantStatus: http.StatusBadRequest,
			wantCalled: false,
		},
		{
			name:       "bad JSON",
			method:     http.MethodPost,
			body:       `{not json}`,
			wantStatus: http.StatusBadRequest,
			wantCalled: false,
		},
		{
			name:       "wrong method GET",
			method:     http.MethodGet,
			body:       `{"token":"a1b2c3d4e5f6g7h8i9j0"}`,
			wantStatus: http.StatusMethodNotAllowed,
			wantCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			capturedTok = ""

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequestWithContext(context.Background(), tt.method, "/token", body)
			w := httptest.NewRecorder()

			ts.server.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			mu.Lock()
			gotCalled := called
			gotToken := capturedTok
			mu.Unlock()

			if gotCalled != tt.wantCalled {
				t.Errorf("emitTokenEvent called = %v, want %v", gotCalled, tt.wantCalled)
			}
			if tt.wantCalled && gotToken != tt.wantToken {
				t.Errorf("captured token = %q, want %q", gotToken, tt.wantToken)
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	ts := NewTokenServer(func(string) {})

	tests := []struct {
		name   string
		method string
	}{
		{name: "GET", method: http.MethodGet},
		{name: "HEAD", method: http.MethodHead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), tt.method, "/health", nil)
			w := httptest.NewRecorder()
			ts.server.Handler.ServeHTTP(w, req)

			if w.Code != http.StatusNoContent {
				t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
			}
		})
	}
}

func TestStartAndStop(t *testing.T) {
	ts := NewTokenServer(func(string) {})

	ctx := context.Background()
	if err := ts.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	// Verify the server is reachable.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+tokenServerAddr+"/health", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("health status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	ts.Stop()

	// After Stop, the server should refuse connections.
	reqAfter, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+tokenServerAddr+"/health", nil)
	respAfter, err := http.DefaultClient.Do(reqAfter)
	if err == nil {
		respAfter.Body.Close()
		t.Error("expected connection error after Stop, got nil")
	}
}

func TestPostTokenIntegration(t *testing.T) {
	var (
		mu          sync.Mutex
		capturedTok string
	)
	ts := NewTokenServer(func(token string) {
		mu.Lock()
		capturedTok = token
		mu.Unlock()
	})

	ctx := context.Background()
	if err := ts.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer ts.Stop()

	body := strings.NewReader(`{"token":"a1b2c3d4e5f6g7h8i9j0k1"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+tokenServerAddr+"/token", body)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /token failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, http.StatusNoContent, respBody)
	}

	mu.Lock()
	got := capturedTok
	mu.Unlock()

	if got != "a1b2c3d4e5f6g7h8i9j0k1" {
		t.Errorf("captured token = %q, want %q", got, "a1b2c3d4e5f6g7h8i9j0k1")
	}
}

func TestNewTokenServerNilEmitDoesNotPanic(t *testing.T) {
	ts := NewTokenServer(nil)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"token":"short"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/token", body)
	ts.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTokenRequestJSONRoundTrip(t *testing.T) {
	original := tokenRequest{Token: "a1b2c3d4e5"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded tokenRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Token != original.Token {
		t.Errorf("token = %q, want %q", decoded.Token, original.Token)
	}
}
