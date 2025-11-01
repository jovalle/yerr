package main

import (
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
