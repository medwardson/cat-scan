package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePumaStats_SingleProcess(t *testing.T) {
	data := []byte(`{
		"started_at": "2026-05-01T00:00:00Z",
		"backlog": 2,
		"running": 3,
		"pool_capacity": 2,
		"max_threads": 5,
		"requests_count": 1234
	}`)

	stats, err := parsePumaStats(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.WorkerCount != 1 {
		t.Errorf("WorkerCount = %d, want 1", stats.WorkerCount)
	}
	if !stats.Booted {
		t.Error("Booted = false, want true")
	}
	if stats.TotalBacklog != 2 {
		t.Errorf("TotalBacklog = %d, want 2", stats.TotalBacklog)
	}
	if stats.TotalRunning != 3 {
		t.Errorf("TotalRunning = %d, want 3", stats.TotalRunning)
	}
	if stats.TotalPoolCapacity != 2 {
		t.Errorf("TotalPoolCapacity = %d, want 2", stats.TotalPoolCapacity)
	}
	if stats.TotalMaxThreads != 5 {
		t.Errorf("TotalMaxThreads = %d, want 5", stats.TotalMaxThreads)
	}
	if len(stats.Workers) != 1 {
		t.Fatalf("len(Workers) = %d, want 1", len(stats.Workers))
	}
	if stats.Workers[0].Index != 0 {
		t.Errorf("Workers[0].Index = %d, want 0", stats.Workers[0].Index)
	}
	if stats.Workers[0].Backlog != 2 {
		t.Errorf("Workers[0].Backlog = %d, want 2", stats.Workers[0].Backlog)
	}
}

func TestParsePumaStats_Clustered(t *testing.T) {
	data := []byte(`{
		"started_at": "2026-05-01T00:00:00Z",
		"workers": 2,
		"phase": 0,
		"booted_workers": 2,
		"old_workers": 0,
		"worker_status": [
			{
				"started_at": "2026-05-01T00:00:00Z",
				"pid": 100,
				"index": 0,
				"phase": 0,
				"booted": true,
				"last_checkin": "2026-05-01T00:00:10Z",
				"last_status": {
					"backlog": 1,
					"running": 2,
					"pool_capacity": 1,
					"max_threads": 3,
					"requests_count": 500
				}
			},
			{
				"started_at": "2026-05-01T00:00:00Z",
				"pid": 101,
				"index": 1,
				"phase": 0,
				"booted": true,
				"last_checkin": "2026-05-01T00:00:10Z",
				"last_status": {
					"backlog": 3,
					"running": 3,
					"pool_capacity": 0,
					"max_threads": 3,
					"requests_count": 700
				}
			}
		]
	}`)

	stats, err := parsePumaStats(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.WorkerCount != 2 {
		t.Errorf("WorkerCount = %d, want 2", stats.WorkerCount)
	}
	if !stats.Booted {
		t.Error("Booted = false, want true")
	}
	if stats.TotalBacklog != 4 {
		t.Errorf("TotalBacklog = %d, want 4", stats.TotalBacklog)
	}
	if stats.TotalRunning != 5 {
		t.Errorf("TotalRunning = %d, want 5", stats.TotalRunning)
	}
	if stats.TotalPoolCapacity != 1 {
		t.Errorf("TotalPoolCapacity = %d, want 1", stats.TotalPoolCapacity)
	}
	if stats.TotalMaxThreads != 6 {
		t.Errorf("TotalMaxThreads = %d, want 6", stats.TotalMaxThreads)
	}
	if len(stats.Workers) != 2 {
		t.Fatalf("len(Workers) = %d, want 2", len(stats.Workers))
	}
	if stats.Workers[0].Backlog != 1 {
		t.Errorf("Workers[0].Backlog = %d, want 1", stats.Workers[0].Backlog)
	}
	if stats.Workers[1].Backlog != 3 {
		t.Errorf("Workers[1].Backlog = %d, want 3", stats.Workers[1].Backlog)
	}
}

func TestParsePumaStats_ClusteredPartialBoot(t *testing.T) {
	data := []byte(`{
		"workers": 2,
		"booted_workers": 1,
		"worker_status": [
			{
				"index": 0,
				"booted": true,
				"last_status": {
					"backlog": 0,
					"running": 1,
					"pool_capacity": 2,
					"max_threads": 3
				}
			},
			{
				"index": 1,
				"booted": false,
				"last_status": {
					"backlog": 0,
					"running": 0,
					"pool_capacity": 0,
					"max_threads": 3
				}
			}
		]
	}`)

	stats, err := parsePumaStats(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Booted {
		t.Error("Booted = true, want false (one worker not booted)")
	}
	if stats.WorkerCount != 2 {
		t.Errorf("WorkerCount = %d, want 2", stats.WorkerCount)
	}
}

func TestParsePumaStats_ZeroBacklog(t *testing.T) {
	data := []byte(`{
		"backlog": 0,
		"running": 0,
		"pool_capacity": 5,
		"max_threads": 5
	}`)

	stats, err := parsePumaStats(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.TotalBacklog != 0 {
		t.Errorf("TotalBacklog = %d, want 0", stats.TotalBacklog)
	}
	if stats.TotalPoolCapacity != 5 {
		t.Errorf("TotalPoolCapacity = %d, want 5", stats.TotalPoolCapacity)
	}
}

func TestParsePumaStats_InvalidJSON(t *testing.T) {
	_, err := parsePumaStats([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestFetchStats_HTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"backlog":1,"running":2,"pool_capacity":3,"max_threads":5}`))
	}))
	defer srv.Close()

	client := NewPumaClient(srv.URL)
	stats, err := client.FetchStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.TotalBacklog != 1 {
		t.Errorf("TotalBacklog = %d, want 1", stats.TotalBacklog)
	}
	if stats.TotalRunning != 2 {
		t.Errorf("TotalRunning = %d, want 2", stats.TotalRunning)
	}
}

func TestFetchStats_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewPumaClient(srv.URL)
	_, err := client.FetchStats()
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestFetchStats_ConnectionRefused(t *testing.T) {
	client := NewPumaClient("http://127.0.0.1:1")
	_, err := client.FetchStats()
	if err == nil {
		t.Error("expected error for connection refused, got nil")
	}
}
