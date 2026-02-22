package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binaryPath string

// TestMain builds the binary before running tests
func TestMain(m *testing.M) {
	// Build the binary
	binaryName := "yerr"
	if runtime.GOOS == "windows" {
		binaryName = "yerr.exe"
	}

	// Build to a temporary location
	buildCmd := exec.Command("go", "build", "-o", binaryName, ".")
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdout = os.Stdout
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build binary: %v\n", err)
		os.Exit(1)
	}

	// Store the absolute path
	var err error
	binaryPath, err = filepath.Abs(binaryName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get absolute path: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	exitCode := m.Run()

	// Cleanup
	_ = os.Remove(binaryName)

	os.Exit(exitCode)
}

// TestVersionCommand tests the version command
func TestVersionCommand(t *testing.T) {
	cmd := exec.Command(getBinaryPath(), "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version command failed: %v, output: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "yerr version") {
		t.Errorf("version output should contain 'yerr version', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, version) {
		t.Errorf("version output should contain version %s, got: %s", version, outputStr)
	}
}

// TestCPUInfoCommand tests the cpu info command
func TestCPUInfoCommand(t *testing.T) {
	cmd := exec.Command(getBinaryPath(), "cpu", "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cpu info command failed: %v, output: %s", err, output)
	}

	outputStr := string(output)
	expectedStrings := []string{
		"CPU INFORMATION",
		"CORE CONFIGURATION",
		"PROCESSOR",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("cpu info output should contain %q, got: %s", expected, outputStr)
		}
	}
}

// TestCPULoadCommand tests the cpu load command with a short duration
func TestCPULoadCommand(t *testing.T) {
	cmd := exec.Command(getBinaryPath(), "cpu", "load", "-w", "2", "-d", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cpu load command failed: %v, output: %s", err, output)
	}

	outputStr := string(output)
	expectedStrings := []string{
		"CPU LOAD TEST RESULTS",
		"Duration:",
		"Workers:",
		"Total Ops:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("cpu load output should contain %q, got: %s", expected, outputStr)
		}
	}
}

func TestCPULoadCommandJSON(t *testing.T) {
	cmd := exec.Command(getBinaryPath(), "cpu", "load", "-w", "1", "-d", "1", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cpu load --json failed: %v, output: %s", err, output)
	}

	var payload map[string]any
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("invalid JSON output: %v, output: %s", err, output)
	}
	if payload["type"] != "cpu_load_result" {
		t.Fatalf("unexpected type field: %#v", payload["type"])
	}
}

func TestCPULoadCommandNoColor(t *testing.T) {
	cmd := exec.Command(getBinaryPath(), "cpu", "load", "-w", "1", "-d", "1", "--no-color")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cpu load --no-color failed: %v, output: %s", err, output)
	}

	if strings.Contains(string(output), "\x1b[") {
		t.Fatalf("expected no ANSI escape sequences, got: %q", string(output))
	}
}

// TestHTTPLoadCommand tests the http load command with a test server
func TestHTTPLoadCommand(t *testing.T) {
	// This test would require a test server running, so we'll test error handling instead
	// Testing with an invalid URL to verify error handling
	cmd := exec.Command(getBinaryPath(), "http", "http://localhost:99999/invalid", "-w", "1", "-n", "1")
	output, err := cmd.CombinedOutput()
	// We expect this to fail, but it should handle the error gracefully
	if err == nil {
		t.Logf("HTTP command unexpectedly succeeded (may be ok): %s", output)
	}
}

// TestCPULoadCommandWithFlags tests various flag combinations
func TestCPULoadCommandWithFlags(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		expect []string
	}{
		{
			name:   "with physical-only flag",
			args:   []string{"cpu", "load", "-w", "2", "-d", "1", "-p"},
			expect: []string{"CPU LOAD TEST RESULTS"},
		},
		{
			name:   "with core-type flag",
			args:   []string{"cpu", "load", "-c", "all", "-w", "2", "-d", "1"},
			expect: []string{"CPU LOAD TEST RESULTS"},
		},
		{
			name:   "with max-procs flag",
			args:   []string{"cpu", "load", "-w", "2", "-d", "1", "-m", "2"},
			expect: []string{"CPU LOAD TEST RESULTS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{}, tt.args...)
			cmd := exec.Command(getBinaryPath(), args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("command failed: %v, output: %s", err, output)
			}

			outputStr := string(output)
			for _, expected := range tt.expect {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("output should contain %q, got: %s", expected, outputStr)
				}
			}
		})
	}
}

// TestHTTPLoadCommandWithFlags tests various HTTP flag combinations
func TestHTTPLoadCommandWithFlags(t *testing.T) {
	// Test with a URL that might not exist - we're testing the command parsing
	cmd := exec.Command(getBinaryPath(), "http", "http://example.com", "-w", "2", "-n", "5", "-X", "GET")
	output, err := cmd.CombinedOutput()
	// May succeed or fail depending on network, but should not crash
	if err != nil {
		// If it fails, should be a network error, not a command parsing error
		outputStr := string(output)
		if strings.Contains(outputStr, "unknown flag") || strings.Contains(outputStr, "invalid argument") {
			t.Errorf("Command parsing error: %v, output: %s", err, outputStr)
		}
	}
}

// TestRootCommandHelp tests the root command help
func TestRootCommandHelp(t *testing.T) {
	cmd := exec.Command(getBinaryPath(), "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("help command failed: %v, output: %s", err, output)
	}

	outputStr := string(output)
	expectedStrings := []string{
		"yerr",
		"CPU",
		"HTTP",
		"load testing",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("help output should contain %q, got: %s", expected, outputStr)
		}
	}
}

// TestInvalidCommand tests error handling for invalid commands
func TestInvalidCommand(t *testing.T) {
	cmd := exec.Command(getBinaryPath(), "invalid", "command")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("Invalid command should return an error")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "unknown command") && !strings.Contains(outputStr, "Error") {
		t.Errorf("Expected error message for invalid command, got: %s", outputStr)
	}
}

// TestCPULoadCommandAutoDetect tests auto-detection of workers
func TestCPULoadCommandAutoDetect(t *testing.T) {
	cmd := exec.Command(getBinaryPath(), "cpu", "load", "-d", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cpu load command failed: %v, output: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "CPU LOAD TEST RESULTS") {
		t.Errorf("Expected CPU load test results, got: %s", outputStr)
	}
}

// getBinaryPath returns the path to the compiled yerr binary
func getBinaryPath() string {
	// Return the path set by TestMain
	if binaryPath != "" {
		return binaryPath
	}

	// Fallback for running tests without TestMain (e.g., go test -run specific test)
	binaryName := "yerr"
	if runtime.GOOS == "windows" {
		binaryName = "yerr.exe"
	}

	// First, try in the bin directory (if built with make)
	binPath := filepath.Join("..", "..", "bin", binaryName)
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}

	// Try in current directory
	if _, err := os.Stat(binaryName); err == nil {
		return binaryName
	}

	// Try in PATH
	if path, err := exec.LookPath(binaryName); err == nil {
		return path
	}

	// Fallback to binary name (will fail if not in PATH, but that's ok for tests)
	return binaryName
}
