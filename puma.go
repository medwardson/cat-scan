package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WorkerStats holds metrics for a single Puma worker.
type WorkerStats struct {
	Index         int
	Backlog       int
	Running       int
	PoolCapacity  int
	MaxThreads    int
}

// PumaStats holds normalized metrics aggregated from all Puma workers.
type PumaStats struct {
	Workers           []WorkerStats
	TotalBacklog      int
	TotalRunning      int
	TotalPoolCapacity int
	TotalMaxThreads   int
	WorkerCount       int
	Booted            bool
}

// pumaRawResponse represents the top-level JSON returned by Puma's /stats endpoint.
// Fields are pointers or omitempty to handle both single and clustered shapes.
type pumaRawResponse struct {
	// Clustered mode fields
	Workers       *int               `json:"workers,omitempty"`
	BootedWorkers *int               `json:"booted_workers,omitempty"`
	WorkerStatus  []pumaWorkerStatus `json:"worker_status,omitempty"`

	// Single mode fields (also appear in last_status for clustered)
	Backlog       *int `json:"backlog,omitempty"`
	Running       *int `json:"running,omitempty"`
	PoolCapacity  *int `json:"pool_capacity,omitempty"`
	MaxThreads    *int `json:"max_threads,omitempty"`
	RequestsCount *int `json:"requests_count,omitempty"`
}

type pumaWorkerStatus struct {
	Index      int             `json:"index"`
	Booted     bool            `json:"booted"`
	LastStatus pumaLastStatus  `json:"last_status"`
}

type pumaLastStatus struct {
	Backlog       int `json:"backlog"`
	Running       int `json:"running"`
	PoolCapacity  int `json:"pool_capacity"`
	MaxThreads    int `json:"max_threads"`
	RequestsCount int `json:"requests_count"`
}

// PumaClient fetches and parses stats from a Puma control socket.
type PumaClient struct {
	controlURL string
	httpClient *http.Client
}

// NewPumaClient creates a new PumaClient targeting the given control URL.
func NewPumaClient(controlURL string) *PumaClient {
	return &PumaClient{
		controlURL: controlURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// FetchStats fetches Puma stats and returns normalized PumaStats.
func (c *PumaClient) FetchStats() (*PumaStats, error) {
	resp, err := c.httpClient.Get(c.controlURL + "/stats")
	if err != nil {
		return nil, fmt.Errorf("fetching puma stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("puma stats returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading puma stats body: %w", err)
	}

	return parsePumaStats(body)
}

// parsePumaStats parses raw JSON into normalized PumaStats.
func parsePumaStats(data []byte) (*PumaStats, error) {
	var raw pumaRawResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing puma stats JSON: %w", err)
	}

	stats := &PumaStats{}

	if raw.Workers != nil && len(raw.WorkerStatus) > 0 {
		// Clustered mode
		stats.WorkerCount = *raw.Workers
		allBooted := true
		for _, ws := range raw.WorkerStatus {
			if !ws.Booted {
				allBooted = false
			}
			w := WorkerStats{
				Index:        ws.Index,
				Backlog:      ws.LastStatus.Backlog,
				Running:      ws.LastStatus.Running,
				PoolCapacity: ws.LastStatus.PoolCapacity,
				MaxThreads:   ws.LastStatus.MaxThreads,
			}
			stats.Workers = append(stats.Workers, w)
			stats.TotalBacklog += w.Backlog
			stats.TotalRunning += w.Running
			stats.TotalPoolCapacity += w.PoolCapacity
			stats.TotalMaxThreads += w.MaxThreads
		}
		stats.Booted = allBooted
	} else {
		// Single process mode — treat as 1 worker at index 0
		stats.WorkerCount = 1
		stats.Booted = true
		w := WorkerStats{
			Index:        0,
			Backlog:      intPtrVal(raw.Backlog),
			Running:      intPtrVal(raw.Running),
			PoolCapacity: intPtrVal(raw.PoolCapacity),
			MaxThreads:   intPtrVal(raw.MaxThreads),
		}
		stats.Workers = append(stats.Workers, w)
		stats.TotalBacklog = w.Backlog
		stats.TotalRunning = w.Running
		stats.TotalPoolCapacity = w.PoolCapacity
		stats.TotalMaxThreads = w.MaxThreads
	}

	return stats, nil
}

func intPtrVal(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
