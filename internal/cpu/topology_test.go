package cpu

import (
	"runtime"
	"strings"
	"testing"
)

func TestDetectTopology(t *testing.T) {
	topology := DetectTopology()

	if topology == nil {
		t.Fatal("DetectTopology() returned nil")
	}

	if topology.LogicalCores <= 0 {
		t.Errorf("LogicalCores = %d, want > 0", topology.LogicalCores)
	}

	// Logical cores should match runtime.NumCPU()
	if topology.LogicalCores != runtime.NumCPU() {
		t.Errorf("LogicalCores = %d, want %d", topology.LogicalCores, runtime.NumCPU())
	}

	// Physical cores should be at least 1
	if topology.PhysicalCores < 1 {
		t.Errorf("PhysicalCores = %d, want >= 1", topology.PhysicalCores)
	}

	// Physical cores should not exceed logical cores
	if topology.PhysicalCores > topology.LogicalCores {
		t.Errorf("PhysicalCores (%d) > LogicalCores (%d)", topology.PhysicalCores, topology.LogicalCores)
	}

	// If heterogeneous, should have performance and efficiency cores
	if topology.HasHeterogeneous {
		if topology.PerformanceCores <= 0 {
			t.Error("PerformanceCores should be > 0 for heterogeneous CPUs")
		}
		if topology.EfficiencyCores <= 0 {
			t.Error("EfficiencyCores should be > 0 for heterogeneous CPUs")
		}
		if topology.PhysicalCores != topology.PerformanceCores+topology.EfficiencyCores {
			t.Errorf("PhysicalCores (%d) should equal PerformanceCores (%d) + EfficiencyCores (%d)",
				topology.PhysicalCores, topology.PerformanceCores, topology.EfficiencyCores)
		}
		// Heterogeneous CPUs typically don't use hyperthreading
		if topology.HasHyperthreading {
			t.Error("Heterogeneous CPUs should not have hyperthreading")
		}
	}

	// If hyperthreading, logical should be > physical
	if topology.HasHyperthreading {
		if topology.LogicalCores <= topology.PhysicalCores {
			t.Errorf("LogicalCores (%d) should be > PhysicalCores (%d) with hyperthreading",
				topology.LogicalCores, topology.PhysicalCores)
		}
	}
}

func TestTopology_String(t *testing.T) {
	tests := []struct {
		name     string
		topology *Topology
		contains []string
	}{
		{
			name: "heterogeneous",
			topology: &Topology{
				LogicalCores:      8,
				PhysicalCores:     8,
				PerformanceCores:  4,
				EfficiencyCores:   4,
				HasHeterogeneous:  true,
				HasHyperthreading: false,
			},
			contains: []string{"8", "4P+4E"},
		},
		{
			name: "hyperthreading",
			topology: &Topology{
				LogicalCores:      8,
				PhysicalCores:     4,
				HasHeterogeneous:  false,
				HasHyperthreading: true,
			},
			contains: []string{"8", "4", "SMT/HT"},
		},
		{
			name: "simple",
			topology: &Topology{
				LogicalCores:      4,
				PhysicalCores:     4,
				HasHeterogeneous:  false,
				HasHyperthreading: false,
			},
			contains: []string{"4", "cores"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.topology.String()
			if result == "" {
				t.Error("String() returned empty string")
			}

			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("String() = %q, should contain %q", result, substr)
				}
			}
		})
	}
}

func TestDetectCPUInfo(t *testing.T) {
	info := DetectCPUInfo()

	if info == nil {
		t.Fatal("DetectCPUInfo() returned nil")
	}

	if info.Topology == nil {
		t.Fatal("Info.Topology is nil")
	}

	// Architecture should be set
	if info.Architecture == "" {
		t.Error("Architecture should not be empty")
	}

	// Architecture should match runtime.GOARCH
	if info.Architecture != runtime.GOARCH {
		t.Errorf("Architecture = %q, want %q", info.Architecture, runtime.GOARCH)
	}

	// Topology should match what DetectTopology returns
	expectedTopology := DetectTopology()
	if info.Topology.LogicalCores != expectedTopology.LogicalCores {
		t.Errorf("Topology.LogicalCores = %d, want %d",
			info.Topology.LogicalCores, expectedTopology.LogicalCores)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    1024,
			expected: "1 KB",
		},
		{
			name:     "megabytes",
			bytes:    1024 * 1024,
			expected: "1.00 MB",
		},
		{
			name:     "gigabytes",
			bytes:    1024 * 1024 * 1024,
			expected: "1.00 GB",
		},
		{
			name:     "large megabytes",
			bytes:    5 * 1024 * 1024,
			expected: "5.00 MB",
		},
		{
			name:     "fractional kilobytes",
			bytes:    1536,
			expected: "2 KB", // 1536 / 1024 = 1.5 KB, rounded to 2 KB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestSysctlString(t *testing.T) {
	// Test with a valid sysctl key (should work on macOS)
	if runtime.GOOS == "darwin" {
		result := sysctlString("kern.ostype")
		if result == "" {
			t.Log("sysctlString returned empty for kern.ostype (may be acceptable)")
		} else if !strings.Contains(result, "Darwin") {
			t.Logf("sysctlString(kern.ostype) = %q, expected to contain 'Darwin'", result)
		}
	}

	// Test with invalid key (should return empty)
	result := sysctlString("invalid.key.that.does.not.exist.12345")
	if result != "" {
		t.Errorf("sysctlString with invalid key should return empty, got %q", result)
	}
}

func TestSysctlSize(t *testing.T) {
	// Test with invalid key (should return empty)
	result := sysctlSize("invalid.key.that.does.not.exist.12345")
	if result != "" {
		t.Errorf("sysctlSize with invalid key should return empty, got %q", result)
	}
}

func TestDetectVendor(t *testing.T) {
	tests := []struct {
		name    string
		brand   string
		machine string
		want    string
	}{
		{
			name:    "intel",
			brand:   "Intel Core i7-9700K",
			machine: "",
			want:    "Intel",
		},
		{
			name:    "amd",
			brand:   "AMD Ryzen 9 5900X",
			machine: "",
			want:    "AMD",
		},
		{
			name:    "apple by brand",
			brand:   "Apple M1",
			machine: "",
			want:    "Apple",
		},
		{
			name:    "apple by machine",
			brand:   "",
			machine: "arm64",
			want:    "Apple",
		},
		{
			name:    "unknown",
			brand:   "Unknown Processor",
			machine: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectVendor(tt.brand, tt.machine)
			if result != tt.want {
				t.Errorf("detectVendor(%q, %q) = %q, want %q", tt.brand, tt.machine, result, tt.want)
			}
		})
	}
}

func TestTopology_EdgeCases(t *testing.T) {
	t.Run("single core", func(t *testing.T) {
		topology := &Topology{
			LogicalCores:      1,
			PhysicalCores:     1,
			HasHeterogeneous:  false,
			HasHyperthreading: false,
		}

		result := topology.String()
		if !strings.Contains(result, "1") {
			t.Errorf("String() for single core should contain '1', got %q", result)
		}
	})

	t.Run("very large core count", func(t *testing.T) {
		topology := &Topology{
			LogicalCores:      64,
			PhysicalCores:     32,
			HasHeterogeneous:  false,
			HasHyperthreading: true,
		}

		result := topology.String()
		if !strings.Contains(result, "64") {
			t.Errorf("String() should contain core count, got %q", result)
		}
	})
}
