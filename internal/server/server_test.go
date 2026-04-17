// ABOUTME: Unit tests for domain parsing and route handling.
// ABOUTME: Verifies banner text splitting and HTTP response codes.

package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseDomain(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		wantFull  string
		wantFirst string
		wantRest  string
	}{
		{
			name:      "subdomain",
			baseURL:   "https://meet.lobb.ie",
			wantFull:  "meet.lobb.ie",
			wantFirst: "meet",
			wantRest:  ".lobb.ie",
		},
		{
			name:      "two_part_domain",
			baseURL:   "https://example.com",
			wantFull:  "example.com",
			wantFirst: "example",
			wantRest:  ".com",
		},
		{
			name:      "bare_host",
			baseURL:   "https://localhost",
			wantFull:  "localhost",
			wantFirst: "localhost",
			wantRest:  "",
		},
		{
			name:      "with_port",
			baseURL:   "https://meet.lobb.ie:8080",
			wantFull:  "meet.lobb.ie",
			wantFirst: "meet",
			wantRest:  ".lobb.ie",
		},
		{
			name:      "with_path",
			baseURL:   "https://meet.lobb.ie/something",
			wantFull:  "meet.lobb.ie",
			wantFirst: "meet",
			wantRest:  ".lobb.ie",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			full, first, rest := parseDomain(tt.baseURL)
			if full != tt.wantFull {
				t.Errorf("full = %q, want %q", full, tt.wantFull)
			}
			if first != tt.wantFirst {
				t.Errorf("first = %q, want %q", first, tt.wantFirst)
			}
			if rest != tt.wantRest {
				t.Errorf("rest = %q, want %q", rest, tt.wantRest)
			}
		})
	}
}

func newTestServer() *Server {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return New(Config{
		Addr:    "127.0.0.1:0",
		BaseURL: "https://meet.lobb.ie",
		VpaasID: "vpaas-magic-cookie-test",
		Logger:  logger,
	})
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRootReturns400(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("root status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRoomPageRendered(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/my-meeting", nil)
	w := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("room status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	if !strings.Contains(body, "my-meeting") {
		t.Error("room page does not contain room name")
	}
	if !strings.Contains(body, "vpaas-magic-cookie-test/my-meeting") {
		t.Error("room page does not contain full JaaS room path")
	}
	if !strings.Contains(body, `<span class="domain-highlight">meet</span>`) {
		t.Error("room page does not contain highlighted domain part")
	}
	if !strings.Contains(body, `<span class="domain-dim">.lobb.ie</span>`) {
		t.Error("room page does not contain dim domain part")
	}
}

func TestNestedPathRejected(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/some/nested/path", nil)
	w := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("nested path status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFontServed(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/static/SpecialElite-Regular.woff2", nil)
	w := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("font status = %d, want %d", w.Code, http.StatusOK)
	}
}
