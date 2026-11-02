package api

import (
	"context"
	"encoding/json"
	"errors"
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

func TestReadyz(t *testing.T) {
	var err error
	h := NewRouter(Info{StartedAt: time.Now(), Ready: func(context.Context) error { return err }})
	get := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		return rec.Code
	}
	if code := get(); code != http.StatusOK {
		t.Errorf("ready: %d", code)
	}
	err = errors.New("db down")
	if code := get(); code != http.StatusServiceUnavailable {
		t.Errorf("not ready: %d", code)
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
