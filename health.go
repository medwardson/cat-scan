package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var pidFile = "/run/ailurophile.pid"

func writePIDFile() error {
	return os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func removePIDFile() {
	os.Remove(pidFile)
}

func checkHealth() int {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unhealthy: %v\n", err)
		return 1
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "unhealthy: invalid pid: %v\n", err)
		return 1
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unhealthy: %v\n", err)
		return 1
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		fmt.Fprintf(os.Stderr, "unhealthy: process %d not running\n", pid)
		return 1
	}

	fmt.Printf("healthy (pid %d)\n", pid)
	return 0
}
