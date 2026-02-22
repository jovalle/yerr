package main

import (
	"os"
	"testing"

	"github.com/jovalle/yerr/internal/cpu"
)

func TestCalculateWorkers(t *testing.T) {
	tests := []struct {
		name         string
		topology     *cpu.Topology
		coreType     string
		physicalOnly bool
		want         int
	}{
		{
			name: "heterogeneous performance cores",
			topology: &cpu.Topology{
				LogicalCores:      12,
				PhysicalCores:     12,
				PerformanceCores:  6,
				EfficiencyCores:   6,
				HasHeterogeneous:  true,
				HasHyperthreading: false,
			},
			coreType:     "performance",
			physicalOnly: false,
			want:         6,
		},
		{
			name: "heterogeneous efficiency cores",
			topology: &cpu.Topology{
				LogicalCores:      12,
				PhysicalCores:     12,
				PerformanceCores:  6,
				EfficiencyCores:   6,
				HasHeterogeneous:  true,
				HasHyperthreading: false,
			},
			coreType:     "efficiency",
			physicalOnly: false,
			want:         6,
		},
		{
			name: "heterogeneous all cores",
			topology: &cpu.Topology{
				LogicalCores:      12,
				PhysicalCores:     12,
				PerformanceCores:  6,
				EfficiencyCores:   6,
				HasHeterogeneous:  true,
				HasHyperthreading: false,
			},
			coreType:     "all",
			physicalOnly: false,
			want:         12,
		},
		{
			name: "hyperthreading with physical only",
			topology: &cpu.Topology{
				LogicalCores:      8,
				PhysicalCores:     4,
				HasHeterogeneous:  false,
				HasHyperthreading: true,
			},
			coreType:     "all",
			physicalOnly: true,
			want:         4,
		},
		{
			name: "hyperthreading without physical only",
			topology: &cpu.Topology{
				LogicalCores:      8,
				PhysicalCores:     4,
				HasHeterogeneous:  false,
				HasHyperthreading: true,
			},
			coreType:     "all",
			physicalOnly: false,
			want:         8,
		},
		{
			name: "simple topology",
			topology: &cpu.Topology{
				LogicalCores:      4,
				PhysicalCores:     4,
				HasHeterogeneous:  false,
				HasHyperthreading: false,
			},
			coreType:     "all",
			physicalOnly: false,
			want:         4,
		},
		{
			name: "heterogeneous with short core type",
			topology: &cpu.Topology{
				LogicalCores:      12,
				PhysicalCores:     12,
				PerformanceCores:  6,
				EfficiencyCores:   6,
				HasHeterogeneous:  true,
				HasHyperthreading: false,
			},
			coreType:     "p",
			physicalOnly: false,
			want:         6,
		},
		{
			name: "heterogeneous efficiency with short",
			topology: &cpu.Topology{
				LogicalCores:      12,
				PhysicalCores:     12,
				PerformanceCores:  6,
				EfficiencyCores:   6,
				HasHeterogeneous:  true,
				HasHyperthreading: false,
			},
			coreType:     "e",
			physicalOnly: false,
			want:         6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateWorkers(tt.topology, tt.coreType, tt.physicalOnly)
			if got != tt.want {
				t.Errorf("calculateWorkers() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestValidateCPUTestArgs(t *testing.T) {
	tests := []struct {
		name      string
		workers   int
		duration  int
		maxProcs  int
		coreType  string
		wantCore  string
		wantError bool
	}{
		{
			name:     "valid defaults",
			workers:  0,
			duration: 0,
			maxProcs: 0,
			coreType: "all",
			wantCore: "all",
		},
		{
			name:     "normalizes whitespace and case",
			workers:  1,
			duration: 10,
			maxProcs: 1,
			coreType: " Performance ",
			wantCore: "performance",
		},
		{
			name:      "negative workers",
			workers:   -1,
			duration:  1,
			maxProcs:  0,
			coreType:  "all",
			wantError: true,
		},
		{
			name:      "invalid core type",
			workers:   1,
			duration:  1,
			maxProcs:  0,
			coreType:  "weird",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCore, err := validateCPUTestArgs(tt.workers, tt.duration, tt.maxProcs, tt.coreType)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateCPUTestArgs() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError {
				return
			}
			if gotCore != tt.wantCore {
				t.Fatalf("validateCPUTestArgs() coreType = %q, want %q", gotCore, tt.wantCore)
			}
		})
	}
}

func TestValidateHTTPTestArgs(t *testing.T) {
	tests := []struct {
		name      string
		workers   int
		requests  int
		duration  int
		method    string
		want      string
		wantError bool
	}{
		{
			name:     "request mode valid",
			workers:  1,
			requests: 10,
			duration: 0,
			method:   "get",
			want:     "GET",
		},
		{
			name:     "duration mode valid",
			workers:  2,
			requests: 0,
			duration: 5,
			method:   " post ",
			want:     "POST",
		},
		{
			name:      "zero workers",
			workers:   0,
			requests:  1,
			duration:  0,
			method:    "GET",
			wantError: true,
		},
		{
			name:      "zero requests in request mode",
			workers:   1,
			requests:  0,
			duration:  0,
			method:    "GET",
			wantError: true,
		},
		{
			name:      "empty method",
			workers:   1,
			requests:  1,
			duration:  0,
			method:    " ",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateHTTPTestArgs(tt.workers, tt.requests, tt.duration, tt.method)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateHTTPTestArgs() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError {
				return
			}
			if got != tt.want {
				t.Fatalf("validateHTTPTestArgs() method = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseHTTPHeaders(t *testing.T) {
	headers, err := parseHTTPHeaders([]string{"X-Test: yes", "Accept: application/json", "X-Test: second"})
	if err != nil {
		t.Fatalf("parseHTTPHeaders() error = %v", err)
	}
	if len(headers["X-Test"]) != 2 {
		t.Fatalf("X-Test header values = %d, want 2", len(headers["X-Test"]))
	}
	if headers["Accept"][0] != "application/json" {
		t.Fatalf("Accept header = %q, want application/json", headers["Accept"][0])
	}
}

func TestParseHTTPHeaders_Invalid(t *testing.T) {
	if _, err := parseHTTPHeaders([]string{"invalid"}); err == nil {
		t.Fatal("parseHTTPHeaders() expected error for invalid header")
	}
}

func TestResolveHTTPBody(t *testing.T) {
	t.Run("inline body", func(t *testing.T) {
		body, err := resolveHTTPBody("hello", "")
		if err != nil {
			t.Fatalf("resolveHTTPBody() error = %v", err)
		}
		if string(body) != "hello" {
			t.Fatalf("body = %q, want hello", string(body))
		}
	})

	t.Run("file body", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "body-*.txt")
		if err != nil {
			t.Fatalf("CreateTemp() error = %v", err)
		}
		if _, err := f.WriteString("from-file"); err != nil {
			t.Fatalf("WriteString() error = %v", err)
		}
		_ = f.Close()

		body, err := resolveHTTPBody("", f.Name())
		if err != nil {
			t.Fatalf("resolveHTTPBody() error = %v", err)
		}
		if string(body) != "from-file" {
			t.Fatalf("body = %q, want from-file", string(body))
		}
	})

	t.Run("mutually exclusive", func(t *testing.T) {
		if _, err := resolveHTTPBody("one", "/tmp/two"); err == nil {
			t.Fatal("resolveHTTPBody() expected error when body and body-file are both set")
		}
	})
}

func TestParseHTTPBasicAuth(t *testing.T) {
	auth, err := parseHTTPBasicAuth("jay:secret")
	if err != nil {
		t.Fatalf("parseHTTPBasicAuth() error = %v", err)
	}
	if auth == nil {
		t.Fatal("parseHTTPBasicAuth() returned nil auth")
	}
	if auth.Username != "jay" || auth.Password != "secret" {
		t.Fatalf("auth = %#v, want jay/secret", auth)
	}

	if _, err := parseHTTPBasicAuth("invalid"); err == nil {
		t.Fatal("parseHTTPBasicAuth() expected error for invalid input")
	}
}
