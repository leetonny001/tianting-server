package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	h := NewRouter(Info{Version: "test", StartedAt: time.Now().Add(-3 * time.Second)})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var got healthResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Status != "ok" || got.Version != "test" {
		t.Errorf("body = %+v", got)
	}
	if got.UptimeSeconds < 3 {
		t.Errorf("uptimeSeconds = %d, want >= 3", got.UptimeSeconds)
	}
}

func TestHealthzRejectsPost(t *testing.T) {
	h := NewRouter(Info{Version: "test", StartedAt: time.Now()})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
