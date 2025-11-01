package cpu

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const (
	osDarwin = "darwin"
)

// Topology contains information about the CPU architecture
type Topology struct {
	LogicalCores      int
	PhysicalCores     int
	PerformanceCores  int
	EfficiencyCores   int
	HasHeterogeneous  bool
	HasHyperthreading bool
}

// Info contains comprehensive CPU information
type Info struct {
	Topology *Topology

	Vendor       string
	BrandString  string
	Architecture string
	Machine      string
	Model        string

	L1ICache      string // L1 instruction cache
	L1DCache      string // L1 data cache
	L2Cache       string
	L3Cache       string
	CacheLineSize string
	PageSize      string
	// Performance level specific cache info
	PerfL1ICache string
	PerfL1DCache string
	PerfL2Cache  string
	EffL1ICache  string
	EffL1DCache  string
	EffL2Cache   string

	Family   int
	ModelNum int
	Stepping int
}

// DetectTopology detects the CPU topology of the system
func DetectTopology() *Topology {
	topology := &Topology{
		LogicalCores: runtime.NumCPU(),
	}

	// Try to detect Apple Silicon heterogeneous architecture
	if runtime.GOOS == osDarwin {
		if detectAppleSiliconTopology(topology) {
			return topology
		}
	}

	// Fall back to simple detection
	// Assume hyperthreading if logical cores > physical cores
	topology.PhysicalCores = topology.LogicalCores / 2
	if topology.PhysicalCores < 1 {
		topology.PhysicalCores = topology.LogicalCores
	}

	topology.HasHyperthreading = topology.LogicalCores > topology.PhysicalCores

	return topology
}

// detectAppleSiliconTopology detects Apple Silicon P/E core configuration
func detectAppleSiliconTopology(topology *Topology) bool {
	// Try to get performance level info from sysctl
	cmd := exec.Command("sysctl", "-n", "hw.perflevel0.physicalcpu")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	perfCores, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || perfCores == 0 {
		return false
	}

	cmd = exec.Command("sysctl", "-n", "hw.perflevel1.physicalcpu")
	output, err = cmd.Output()
	if err != nil {
		return false
	}

	effCores, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || effCores == 0 {
		return false
	}

	// Successfully detected heterogeneous architecture
	topology.PerformanceCores = perfCores
	topology.EfficiencyCores = effCores
	topology.PhysicalCores = perfCores + effCores
	topology.HasHeterogeneous = true
	topology.HasHyperthreading = false // Apple Silicon doesn't use HT

	return true
}

// String returns a human-readable description of the topology
func (t *Topology) String() string {
	if t.HasHeterogeneous {
		return formatHeterogeneous(t)
	}

	if t.HasHyperthreading {
		return formatHyperthreading(t)
	}

	return formatSimple(t)
}

func formatHeterogeneous(t *Topology) string {
	return strings.Join([]string{
		strconv.Itoa(t.LogicalCores),
		" cores (",
		strconv.Itoa(t.PerformanceCores),
		"P+",
		strconv.Itoa(t.EfficiencyCores),
		"E)",
	}, "")
}

func formatHyperthreading(t *Topology) string {
	return strings.Join([]string{
		strconv.Itoa(t.LogicalCores),
		" logical cores (",
		strconv.Itoa(t.PhysicalCores),
		" physical + SMT/HT)",
	}, "")
}

func formatSimple(t *Topology) string {
	return strconv.Itoa(t.LogicalCores) + " cores"
}

// DetectCPUInfo detects comprehensive CPU information
func DetectCPUInfo() *Info {
	info := &Info{
		Topology:     DetectTopology(),
		Architecture: runtime.GOARCH,
	}

	if runtime.GOOS == osDarwin {
		detectDarwinCPUInfo(info)
	}

	return info
}

// detectDarwinCPUInfo detects CPU info on macOS
func detectDarwinCPUInfo(info *Info) {
	info.BrandString = sysctlString("machdep.cpu.brand_string")
	info.Machine = sysctlString("hw.machine")
	info.Model = sysctlString("hw.model")
	info.Vendor = detectVendor(info.BrandString, info.Machine)

	info.L1ICache = sysctlSize("hw.l1icachesize")
	info.L1DCache = sysctlSize("hw.l1dcachesize")
	info.L2Cache = sysctlSize("hw.l2cachesize")
	info.L3Cache = sysctlSize("hw.l3cachesize")
	info.CacheLineSize = sysctlSize("hw.cachelinesize")
	info.PageSize = sysctlSize("hw.pagesize")

	if !info.Topology.HasHeterogeneous {
		return
	}

	setHeterogeneousCaches(info)
}

// formatBytes formats bytes into human-readable format (KB, MB, GB)
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func sysctlString(key string) string {
	output, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func sysctlSize(key string) string {
	value, ok := sysctlInt(key)
	if !ok || value <= 0 {
		return ""
	}

	return formatBytes(int64(value))
}

func sysctlInt(key string) (int, bool) {
	output, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return 0, false
	}

	value, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, false
	}

	return value, true
}

func detectVendor(brand, machine string) string {
	switch {
	case strings.Contains(brand, "Intel"):
		return "Intel"
	case strings.Contains(brand, "AMD"):
		return "AMD"
	case strings.Contains(brand, "Apple"), machine == "arm64":
		return "Apple"
	default:
		return ""
	}
}

func setHeterogeneousCaches(info *Info) {
	perfCaches := map[string]*string{
		"hw.perflevel0.l1icachesize": &info.PerfL1ICache,
		"hw.perflevel0.l1dcachesize": &info.PerfL1DCache,
		"hw.perflevel0.l2cachesize":  &info.PerfL2Cache,
	}

	for key, target := range perfCaches {
		if formatted := sysctlSize(key); formatted != "" {
			*target = formatted
		}
	}

	effCaches := map[string]*string{
		"hw.perflevel1.l1icachesize": &info.EffL1ICache,
		"hw.perflevel1.l1dcachesize": &info.EffL1DCache,
		"hw.perflevel1.l2cachesize":  &info.EffL2Cache,
	}

	for key, target := range effCaches {
		if formatted := sysctlSize(key); formatted != "" {
			*target = formatted
		}
	}
}
