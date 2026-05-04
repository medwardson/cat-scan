package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthServer_InitiallyUnhealthy(t *testing.T) {
	h := NewHealthServer(slog.Default())

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	h.handleHealthz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "error" {
		t.Errorf("status = %q, want error", body["status"])
	}
}

func TestHealthServer_Healthy(t *testing.T) {
	h := NewHealthServer(slog.Default())
	h.SetHealthy()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	h.handleHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestHealthServer_UnhealthyWithMessage(t *testing.T) {
	h := NewHealthServer(slog.Default())
	h.SetUnhealthy("connection refused")

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	h.handleHealthz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["message"] != "connection refused" {
		t.Errorf("message = %q, want 'connection refused'", body["message"])
	}
}

func TestHealthServer_HealthyThenUnhealthy(t *testing.T) {
	h := NewHealthServer(slog.Default())

	h.SetHealthy()
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	h.handleHealthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("after SetHealthy: status = %d, want 200", rec.Code)
	}

	h.SetUnhealthy("poll failed")
	req = httptest.NewRequest("GET", "/healthz", nil)
	rec = httptest.NewRecorder()
	h.handleHealthz(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("after SetUnhealthy: status = %d, want 503", rec.Code)
	}
}
