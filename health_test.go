package main

import (
	"os"
	"path/filepath"
	"testing"
)

func useTempPIDFile(t *testing.T) {
	t.Helper()
	orig := pidFile
	pidFile = filepath.Join(t.TempDir(), "ailurophile.pid")
	t.Cleanup(func() { pidFile = orig })
}

func TestCheckHealth_Healthy(t *testing.T) {
	useTempPIDFile(t)

	if err := writePIDFile(); err != nil {
		t.Fatalf("writePIDFile() error: %v", err)
	}

	if code := checkHealth(); code != 0 {
		t.Errorf("checkHealth() = %d, want 0", code)
	}
}

func TestCheckHealth_NoPIDFile(t *testing.T) {
	useTempPIDFile(t)

	if code := checkHealth(); code != 1 {
		t.Errorf("checkHealth() = %d, want 1", code)
	}
}

func TestCheckHealth_StaleProcess(t *testing.T) {
	useTempPIDFile(t)

	os.WriteFile(pidFile, []byte("999999999"), 0644)

	if code := checkHealth(); code != 1 {
		t.Errorf("checkHealth() = %d, want 1", code)
	}
}
