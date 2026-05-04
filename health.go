package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

// HealthServer provides a simple health check HTTP endpoint.
type HealthServer struct {
	mu      sync.RWMutex
	healthy bool
	lastErr string
	logger  *slog.Logger
}

// NewHealthServer creates a new HealthServer.
func NewHealthServer(logger *slog.Logger) *HealthServer {
	return &HealthServer{
		logger: logger,
	}
}

// SetHealthy marks the health status as OK after a successful poll.
func (h *HealthServer) SetHealthy() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthy = true
	h.lastErr = ""
}

// SetUnhealthy marks the health status as failed with the given error message.
func (h *HealthServer) SetUnhealthy(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthy = false
	h.lastErr = msg
}

// ListenAndServe starts the health check HTTP server on the given port.
func (h *HealthServer) ListenAndServe(port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealthz)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		h.logger.Info("health server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error("health server error", "error", err)
		}
	}()

	return srv
}

func (h *HealthServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	healthy := h.healthy
	lastErr := h.lastErr
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if healthy {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	msg := "last poll failed or never ran"
	if lastErr != "" {
		msg = lastErr
	}

	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "error",
		"message": msg,
	})
}
