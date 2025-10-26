package daemon

import (
	"os"
	"testing"
)

// TestWriteAndReadPID tests PID file write and read operations
func TestWriteAndReadPID(t *testing.T) {
	// Clean up any existing PID file first
	defer os.Remove(pidFile)

	// Write PID
	expectedPID := os.Getpid()
	err := writePID()
	if err != nil {
		t.Fatalf("writePID failed: %v", err)
	}

	// Read PID
	readPID, err := readPID()
	if err != nil {
		t.Fatalf("readPID failed: %v", err)
	}

	if readPID != expectedPID {
		t.Errorf("Expected PID %d, got %d", expectedPID, readPID)
	}
}

// TestPIDFileCreation tests that PID file is created with correct permissions
func TestPIDFileCreation(t *testing.T) {
	defer os.Remove(pidFile)

	err := writePID()
	if err != nil {
		t.Fatalf("writePID failed: %v", err)
	}

	// Check if file exists
	info, err := os.Stat(pidFile)
	if err != nil {
		t.Fatalf("PID file not created: %v", err)
	}

	// Check file is readable
	if info.Mode()&0400 == 0 {
		t.Errorf("PID file not readable by owner")
	}
}

// TestCleanupPID tests that cleanup removes the PID file
func TestCleanupPID(t *testing.T) {
	// Write PID
	err := writePID()
	if err != nil {
		t.Fatalf("writePID failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("PID file not created")
	}

	// Cleanup
	cleanupPID()

	// Verify file is removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Errorf("PID file not removed by cleanup")
	}
}

// TestReadPIDFileNotFound tests error handling when PID file doesn't exist
func TestReadPIDFileNotFound(t *testing.T) {
	// Ensure PID file doesn't exist
	os.Remove(pidFile)

	_, err := readPID()
	if err == nil {
		t.Errorf("Expected error when PID file doesn't exist")
	}
}

// TestReadPIDInvalidContent tests error handling with invalid PID file content
func TestReadPIDInvalidContent(t *testing.T) {
	defer os.Remove(pidFile)

	// Write invalid content
	err := os.WriteFile(pidFile, []byte("not-a-number"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Try to read
	_, err = readPID()
	if err == nil {
		t.Errorf("Expected error when PID file contains invalid content")
	}
}

// TestStart tests the Start function
func TestStart(t *testing.T) {
	defer os.Remove(pidFile)

	// Clean up any existing PID file
	cleanupPID()

	// Start should succeed if proxy is not running
	err := Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify PID file was created
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("PID file not created after Start: %v", err)
	}
}

// TestStartAlreadyRunning tests Start when process is already running
func TestStartAlreadyRunning(t *testing.T) {
	defer os.Remove(pidFile)

	// Write current PID to simulate running process
	err := writePID()
	if err != nil {
		t.Fatalf("writePID failed: %v", err)
	}

	// Start should fail since IsRunning will return true
	err = Start()
	if err == nil {
		t.Errorf("Start should fail when proxy is already running")
	}
}

// TestCleanupOnExit tests that Cleanup removes PID file
func TestCleanupOnExit(t *testing.T) {
	// Write PID
	err := writePID()
	if err != nil {
		t.Fatalf("writePID failed: %v", err)
	}

	// Cleanup should remove the file
	Cleanup()

	// Verify file is removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Errorf("PID file not removed by Cleanup")
	}
}

// TestIsProcessRunningWithCurrentPID tests that isProcessRunning detects current process
func TestIsProcessRunningWithCurrentPID(t *testing.T) {
	defer os.Remove(pidFile)

	// Write current PID
	err := writePID()
	if err != nil {
		t.Fatalf("writePID failed: %v", err)
	}

	// isProcessRunning should return true for current process
	if !isProcessRunning() {
		t.Errorf("isProcessRunning should return true for current process")
	}
}

// TestIsProcessRunningWithInvalidPID tests that isProcessRunning handles invalid PID
func TestIsProcessRunningWithInvalidPID(t *testing.T) {
	defer os.Remove(pidFile)

	// Write an invalid PID (very large number unlikely to be running)
	err := os.WriteFile(pidFile, []byte("999999999"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// isProcessRunning should return false
	if isProcessRunning() {
		t.Errorf("isProcessRunning should return false for non-existent PID")
	}
}

// TestIsProcessRunningNoPIDFile tests that isProcessRunning handles missing PID file
func TestIsProcessRunningNoPIDFile(t *testing.T) {
	// Ensure PID file doesn't exist
	os.Remove(pidFile)

	// isProcessRunning should return false when file doesn't exist
	if isProcessRunning() {
		t.Errorf("isProcessRunning should return false when PID file doesn't exist")
	}
}

// TestStatusOutput tests that Status function works without panicking
func TestStatusOutput(t *testing.T) {
	defer os.Remove(pidFile)

	// Write current PID
	err := writePID()
	if err != nil {
		t.Fatalf("writePID failed: %v", err)
	}

	// Status should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Status panicked: %v", r)
		}
	}()

	Status()
}

// TestStatusNotRunning tests Status when process is not running
func TestStatusNotRunning(t *testing.T) {
	// Ensure PID file doesn't exist
	os.Remove(pidFile)

	// Status should not panic even when process is not running
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Status panicked: %v", r)
		}
	}()

	Status()
}

// TestIsRunningHealthCheck tests IsRunning with health check
func TestIsRunningHealthCheck(t *testing.T) {
	defer os.Remove(pidFile)

	// Write current PID
	err := writePID()
	if err != nil {
		t.Fatalf("writePID failed: %v", err)
	}

	// IsRunning should use health check first, then fall back to PID check
	// Since no server is running, health check will fail and it will use PID check
	result := IsRunning()

	if !result {
		t.Errorf("IsRunning should return true when valid PID file exists")
	}
}

// BenchmarkWritePID benchmarks PID file writing
func BenchmarkWritePID(b *testing.B) {
	defer os.Remove(pidFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writePID()
	}
}

// BenchmarkReadPID benchmarks PID file reading
func BenchmarkReadPID(b *testing.B) {
	err := writePID()
	if err != nil {
		b.Fatalf("Failed to write PID: %v", err)
	}
	defer os.Remove(pidFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		readPID()
	}
}

// BenchmarkIsProcessRunning benchmarks process running check
func BenchmarkIsProcessRunning(b *testing.B) {
	err := writePID()
	if err != nil {
		b.Fatalf("Failed to write PID: %v", err)
	}
	defer os.Remove(pidFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isProcessRunning()
	}
}
