package cpu

import (
	"bufio"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const (
	osDarwin  = "darwin"
	osLinux   = "linux"
	osWindows = "windows"
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
		LogicalCores:  runtime.NumCPU(),
		PhysicalCores: runtime.NumCPU(),
	}

	switch runtime.GOOS {
	case osDarwin:
		// Try to detect Apple Silicon heterogeneous architecture first.
		if detectAppleSiliconTopology(topology) {
			return topology
		}
		if logical, ok := sysctlInt("hw.logicalcpu"); ok && logical > 0 {
			topology.LogicalCores = logical
		}
		if physical, ok := sysctlInt("hw.physicalcpu"); ok && physical > 0 {
			topology.PhysicalCores = physical
		}
	case osLinux:
		if physical, ok := detectLinuxPhysicalCores(); ok && physical > 0 {
			topology.PhysicalCores = physical
		}
	case osWindows:
		if logical, physical, ok := detectWindowsCoreCounts(); ok {
			if logical > 0 {
				topology.LogicalCores = logical
			}
			if physical > 0 {
				topology.PhysicalCores = physical
			}
		}
	}

	// Conservative fallback: if we can't detect physical cores reliably, assume
	// physical == logical instead of guessing HT/SMT and misreporting topology.
	if topology.PhysicalCores < 1 || topology.PhysicalCores > topology.LogicalCores {
		topology.PhysicalCores = topology.LogicalCores
	}

	topology.HasHyperthreading = topology.LogicalCores > topology.PhysicalCores

	return topology
}

func detectLinuxPhysicalCores() (int, bool) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		if cores := parseLinuxCPUInfoPhysicalCores(string(data)); cores > 0 {
			return cores, true
		}
	}

	if output, err := exec.Command("lscpu").Output(); err == nil {
		if cores := parseLSCPUSummaryPhysicalCores(string(output)); cores > 0 {
			return cores, true
		}
	}

	return 0, false
}

func parseLinuxCPUInfoPhysicalCores(cpuinfo string) int {
	blocks := parseCPUInfoBlocks(cpuinfo)
	if len(blocks) == 0 {
		return 0
	}

	if cores := countUniqueCores(blocks); cores > 0 {
		return cores
	}

	return countCoresBySocket(blocks)
}

func parseCPUInfoBlocks(cpuinfo string) []map[string]string {
	scanner := bufio.NewScanner(strings.NewReader(cpuinfo))
	current := make(map[string]string)
	var blocks []map[string]string

	flush := func() {
		if len(current) == 0 {
			return
		}
		copied := make(map[string]string, len(current))
		maps.Copy(copied, current)
		blocks = append(blocks, copied)
		current = make(map[string]string)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		current[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	flush()

	return blocks
}

func countUniqueCores(blocks []map[string]string) int {
	uniqueCorePairs := make(map[string]struct{})
	for _, b := range blocks {
		physID, okPhys := b["physical id"]
		coreID, okCore := b["core id"]
		if okPhys && okCore {
			uniqueCorePairs[physID+":"+coreID] = struct{}{}
		}
	}
	return len(uniqueCorePairs)
}

// countCoresBySocket is a fallback for systems that expose `cpu cores` but not per-core IDs.
func countCoresBySocket(blocks []map[string]string) int {
	socketCores := make(map[string]int)
	var singleSocketCores int
	for _, b := range blocks {
		cpuCoresStr, ok := b["cpu cores"]
		if !ok {
			continue
		}
		cpuCores, err := strconv.Atoi(strings.TrimSpace(cpuCoresStr))
		if err != nil || cpuCores <= 0 {
			continue
		}
		if physID, ok := b["physical id"]; ok {
			if _, exists := socketCores[physID]; !exists {
				socketCores[physID] = cpuCores
			}
			continue
		}
		if singleSocketCores == 0 {
			singleSocketCores = cpuCores
		}
	}

	total := 0
	for _, n := range socketCores {
		total += n
	}
	if total > 0 {
		return total
	}

	return singleSocketCores
}

func parseLSCPUSummaryPhysicalCores(summary string) int {
	var sockets, coresPerSocket int
	scanner := bufio.NewScanner(strings.NewReader(summary))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Socket(s):"):
			sockets = parseTrailingIntAfterColon(line)
		case strings.HasPrefix(line, "Core(s) per socket:"):
			coresPerSocket = parseTrailingIntAfterColon(line)
		}
	}

	if sockets > 0 && coresPerSocket > 0 {
		return sockets * coresPerSocket
	}
	if coresPerSocket > 0 {
		return coresPerSocket
	}

	return 0
}

func parseTrailingIntAfterColon(line string) int {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0
	}
	return n
}

func detectWindowsCoreCounts() (logical, physical int, ok bool) {
	if out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_Processor | Measure-Object -Property NumberOfCores -Sum).Sum;"+
			"(Get-CimInstance Win32_Processor | Measure-Object -Property NumberOfLogicalProcessors -Sum).Sum").Output(); err == nil {
		if l, p, parsed := parseWindowsPowerShellCoreCounts(string(out)); parsed {
			return l, p, true
		}
	}

	if out, err := exec.Command("wmic", "cpu", "get", "NumberOfCores,NumberOfLogicalProcessors", "/value").Output(); err == nil {
		if l, p, parsed := parseWindowsWMICCoreCounts(string(out)); parsed {
			return l, p, true
		}
	}

	return 0, 0, false
}

func parseWindowsPowerShellCoreCounts(output string) (logical, physical int, ok bool) {
	lines := strings.Fields(output)
	if len(lines) < 2 {
		return 0, 0, false
	}

	physical, err1 := strconv.Atoi(lines[0])
	logical, err2 := strconv.Atoi(lines[1])
	if err1 != nil || err2 != nil || physical <= 0 || logical <= 0 {
		return 0, 0, false
	}

	return logical, physical, true
}

func parseWindowsWMICCoreCounts(output string) (logical, physical int, ok bool) {
	var logicalTotal, physicalTotal int
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			continue
		}

		switch key {
		case "NumberOfLogicalProcessors":
			logicalTotal += n
		case "NumberOfCores":
			physicalTotal += n
		}
	}

	if logicalTotal <= 0 || physicalTotal <= 0 {
		return 0, 0, false
	}

	return logicalTotal, physicalTotal, true
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
