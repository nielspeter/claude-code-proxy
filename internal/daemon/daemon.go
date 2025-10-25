package daemon

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"syscall"
)

const (
	pidFile   = "/tmp/claude-code-proxy.pid"
	healthURL = "http://localhost:8082/health"
)

// IsRunning checks if the proxy daemon is running
func IsRunning() bool {
	// Try health check first
	resp, err := http.Get(healthURL)
	if err == nil {
		resp.Body.Close()
		return resp.StatusCode == 200
	}

	// Fallback: check PID file
	return isProcessRunning()
}

// Start daemonizes the current process
func Start() error {
	// Already running check
	if IsRunning() {
		return fmt.Errorf("proxy is already running")
	}

	// Clean up stale PID file
	cleanupPID()

	// Write PID file
	if err := writePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	fmt.Println("🚀 Starting Claude Code Proxy daemon...")
	return nil
}

// Stop stops the running daemon
func Stop() {
	if !IsRunning() {
		fmt.Println("Proxy is not running")
		return
	}

	pid, err := readPID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading PID: %v\n", err)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding process: %v\n", err)
		return
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping process: %v\n", err)
		return
	}

	cleanupPID()
	fmt.Println("✅ Proxy stopped")
}

// Status prints the current daemon status
func Status() {
	if IsRunning() {
		pid, _ := readPID()
		fmt.Printf("✅ Proxy is running (PID: %d)\n", pid)
		fmt.Printf("   Health endpoint: %s\n", healthURL)
	} else {
		fmt.Println("❌ Proxy is not running")
	}
}

// Helper functions

func writePID() error {
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func readPID() (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func cleanupPID() {
	os.Remove(pidFile)
}

func isProcessRunning() bool {
	pid, err := readPID()
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Cleanup should be called on shutdown
func Cleanup() {
	cleanupPID()
}
