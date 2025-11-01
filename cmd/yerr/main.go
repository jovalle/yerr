package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/jovalle/yerr/internal/cpu"
	"github.com/jovalle/yerr/internal/http"
)

var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "yerr",
	Short: "yerr - CPU and HTTP load testing suite",
	Long: `yerr is a fast, lightweight load testing framework inspired by speed and endurance.

Like Hermes' winged boots meeting the streets of NYC, yerr delivers performance
testing with style and reliability.`,
	Version: version,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of yerr",
	Long:  `All software has versions. This is yerr's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("yerr version %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(cpuCmd)
	rootCmd.AddCommand(httpCmd)
	rootCmd.AddCommand(versionCmd)
	cpuCmd.AddCommand(loadCmd)
	cpuCmd.AddCommand(infoCmd)
}

var cpuCmd = &cobra.Command{
	Use:   "cpu",
	Short: "CPU load testing and information",
	Long:  `CPU load testing and information tools. Run 'yerr cpu --help' to see available subcommands.`,
}

var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Run CPU load tests",
	Long:  `Execute CPU-intensive workloads to stress test processor performance`,
	RunE:  runCPUTest,
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show comprehensive CPU information",
	Long:  `Display detailed information about the CPU including vendor, model, architecture, cores, cache, and specifications`,
	Run:   runInfo,
}

var httpCmd = &cobra.Command{
	Use:   "http [URL]",
	Short: "Run HTTP load tests",
	Long:  `Execute HTTP requests to test web service performance and reliability`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runHTTPTest,
}

var (
	cpuWorkers      int
	cpuDuration     int
	cpuPhysicalOnly bool
	cpuCoreType     string
	cpuMaxProcs     int
	httpWorkers     int
	httpRequests    int
	httpDuration    int
	httpMethod      string
)

func init() {
	loadCmd.Flags().IntVarP(&cpuWorkers, "workers", "w", 0, "Number of CPU workers (0 = auto-detect based on CPU cores)")
	loadCmd.Flags().IntVarP(&cpuDuration, "duration", "d", 0, "Test duration in seconds (0 = run forever)")
	loadCmd.Flags().BoolVarP(&cpuPhysicalOnly, "physical-only", "p", false, "Use only physical cores, ignore hyperthreading/SMT")
	loadCmd.Flags().StringVarP(&cpuCoreType, "core-type", "c", "all", "Core type: all, performance, efficiency (for heterogeneous CPUs)")
	loadCmd.Flags().IntVarP(&cpuMaxProcs, "max-procs", "m", 0, "Set GOMAXPROCS to limit OS threads/cores (0 = use Go default)")

	httpCmd.Flags().IntVarP(&httpWorkers, "workers", "w", 10, "Number of concurrent workers")
	httpCmd.Flags().IntVarP(&httpRequests, "requests", "n", 100, "Total number of requests")
	httpCmd.Flags().IntVarP(&httpDuration, "duration", "d", 0, "Test duration in seconds (0 = use request count)")
	httpCmd.Flags().StringVarP(&httpMethod, "method", "X", "GET", "HTTP method")
}

func runCPUTest(cmd *cobra.Command, args []string) error {
	cyan := color.New(color.FgCyan, color.Bold)
	yellow := color.New(color.FgYellow)
	blue := color.New(color.FgBlue)
	green := color.New(color.FgGreen)
	magenta := color.New(color.FgMagenta)

	// Set GOMAXPROCS if specified
	if cpuMaxProcs > 0 {
		oldMaxProcs := runtime.GOMAXPROCS(cpuMaxProcs)
		defer runtime.GOMAXPROCS(oldMaxProcs) // Restore on exit
		_, _ = blue.Printf("🔧 Set GOMAXPROCS to %d (was %d)\n", cpuMaxProcs, oldMaxProcs)
	}

	// Detect CPU topology
	topology := cpu.DetectTopology()

	// Auto-detect workers if not specified
	workers := cpuWorkers
	if workers == 0 {
		workers = calculateWorkers(topology, cpuCoreType, cpuPhysicalOnly)
	}

	// Show CPU topology info
	fmt.Println()
	_, _ = blue.Printf("💻 CPU Info: ")
	fmt.Print(topology.String())
	fmt.Println()

	// Show which cores are being used
	if cpuWorkers == 0 {
		printWorkerSelection(topology, cpuCoreType, cpuPhysicalOnly, workers, green, magenta)
	}

	_, _ = cyan.Print("\n🏃 Running CPU load test")
	if cpuDuration == 0 {
		fmt.Printf(" with %s workers\n", yellow.Sprint(workers))
		fmt.Println()
		_, _ = yellow.Println("⚠️  WARNING: This test will run indefinitely. Press CTRL+C to stop.")
	} else {
		fmt.Printf(" with %s workers for %s seconds...\n",
			yellow.Sprint(workers), yellow.Sprint(cpuDuration))
	}

	var duration time.Duration
	if cpuDuration > 0 {
		duration = time.Duration(cpuDuration) * time.Second
	} else {
		// Set to a very large duration for "infinite" runs
		duration = time.Duration(1<<63 - 1) // Max duration (~292 years)
	}

	config := cpu.TestConfig{
		Workers:  workers,
		Duration: duration,
	}

	result, err := cpu.Run(config)
	if err != nil {
		return fmt.Errorf("CPU test failed: %w", err)
	}

	cpu.PrintResult(result)
	return nil
}

func calculateWorkers(topology *cpu.Topology, coreType string, physicalOnly bool) int {
	// Handle heterogeneous architecture
	if topology.HasHeterogeneous {
		switch coreType {
		case "performance", "p":
			return topology.PerformanceCores
		case "efficiency", "e":
			return topology.EfficiencyCores
		default:
			return topology.LogicalCores
		}
	}

	// Handle hyperthreading
	if physicalOnly && topology.HasHyperthreading {
		return topology.PhysicalCores
	}

	return topology.LogicalCores
}

func printWorkerSelection(topology *cpu.Topology, coreType string, physicalOnly bool, workers int, green, magenta *color.Color) {
	if topology.HasHeterogeneous {
		switch coreType {
		case "performance", "p":
			_, _ = green.Printf("   Using: Performance cores only (%d workers)\n", workers)
			return
		case "efficiency", "e":
			_, _ = magenta.Printf("   Using: Efficiency cores only (%d workers)\n", workers)
			return
		default:
			fmt.Printf("   Using: All cores (%d workers)\n", workers)
			return
		}
	}

	if physicalOnly && topology.HasHyperthreading {
		fmt.Printf("   Using: Physical cores only (%d workers)\n", workers)
		return
	}

	fmt.Printf("   Using: All logical cores (%d workers)\n", workers)
}

func runHTTPTest(cmd *cobra.Command, args []string) error {
	url := args[0]

	cyan := color.New(color.FgCyan, color.Bold)
	yellow := color.New(color.FgYellow)
	blue := color.New(color.FgBlue)

	_, _ = cyan.Print("\n🏃 Running HTTP load test")
	fmt.Printf(" against %s\n", blue.Sprint(url))

	useDuration := httpDuration > 0
	if useDuration {
		fmt.Printf("   Workers: %s, Duration: %ss, Method: %s\n",
			yellow.Sprint(httpWorkers),
			yellow.Sprint(httpDuration),
			cyan.Sprint(httpMethod))
	} else {
		fmt.Printf("   Workers: %s, Requests: %s, Method: %s\n",
			yellow.Sprint(httpWorkers),
			yellow.Sprint(httpRequests),
			cyan.Sprint(httpMethod))
	}

	config := http.TestConfig{
		URL:         url,
		Method:      httpMethod,
		Workers:     httpWorkers,
		TotalReqs:   httpRequests,
		Duration:    time.Duration(httpDuration) * time.Second,
		UseDuration: useDuration,
	}

	result, err := http.Run(config)
	if err != nil {
		return fmt.Errorf("HTTP test failed: %w", err)
	}

	http.PrintResult(result)
	return nil
}

type infoPalette struct {
	cyan      *color.Color
	green     *color.Color
	yellow    *color.Color
	blue      *color.Color
	magenta   *color.Color
	white     *color.Color
	separator string
}

func newInfoPalette() infoPalette {
	palette := infoPalette{
		cyan:    color.New(color.FgCyan, color.Bold),
		green:   color.New(color.FgGreen, color.Bold),
		yellow:  color.New(color.FgYellow, color.Bold),
		blue:    color.New(color.FgBlue),
		magenta: color.New(color.FgMagenta),
		white:   color.New(color.FgWhite, color.Bold),
	}

	palette.separator = color.CyanString(strings.Repeat("━", 70))
	return palette
}

func runInfo(cmd *cobra.Command, args []string) {
	palette := newInfoPalette()
	info := cpu.DetectCPUInfo()

	fmt.Println()
	printInfoHeader(palette)
	printProcessorSection(info, palette)
	printCoreConfiguration(info.Topology, palette)
	printCacheHierarchy(info, palette)
	printMemoryConfiguration(info, palette)
	printWorkerRecommendations(info.Topology, palette)
	printUsageExamples(info.Topology, palette)
	printInfoFooter(palette)
}

func printInfoHeader(palette infoPalette) {
	fmt.Println(palette.separator)
	_, _ = palette.cyan.Println("💻 CPU INFORMATION")
	fmt.Println(palette.separator)
	fmt.Println()
}

func printProcessorSection(info *cpu.Info, palette infoPalette) {
	_, _ = palette.white.Println("🔧 PROCESSOR")
	if info.Vendor != "" {
		fmt.Printf("  Vendor:            %s\n", palette.cyan.Sprint(info.Vendor))
	}
	if info.BrandString != "" {
		fmt.Printf("  Model:             %s\n", palette.blue.Sprint(info.BrandString))
	}
	if info.Architecture != "" {
		fmt.Printf("  Architecture:      %s\n", palette.green.Sprint(info.Architecture))
	}
	if info.Model != "" {
		fmt.Printf("  Machine:           %s\n", palette.yellow.Sprint(info.Model))
	}

	fmt.Println()
}

func printCoreConfiguration(topology *cpu.Topology, palette infoPalette) {
	_, _ = palette.white.Println("📊 CORE CONFIGURATION")

	switch {
	case topology.HasHeterogeneous:
		fmt.Printf("  Type:              %s\n", palette.green.Sprint("Heterogeneous (Performance + Efficiency)"))
		fmt.Printf("  Total Cores:       %s\n", palette.cyan.Sprint(topology.LogicalCores))
		fmt.Printf("  Physical Cores:    %s\n", palette.blue.Sprint(topology.PhysicalCores))
		fmt.Printf("    • P-cores:       %s\n", palette.green.Sprint(topology.PerformanceCores))
		fmt.Printf("    • E-cores:       %s\n", palette.magenta.Sprint(topology.EfficiencyCores))
		fmt.Printf("  Hyperthreading:    %s\n", palette.yellow.Sprint("No"))
	case topology.HasHyperthreading:
		fmt.Printf("  Type:              %s\n", palette.blue.Sprint("Uniform with HT/SMT"))
		fmt.Printf("  Logical Cores:     %s\n", palette.cyan.Sprint(topology.LogicalCores))
		fmt.Printf("  Physical Cores:    %s\n", palette.green.Sprint(topology.PhysicalCores))
		fmt.Printf("  Hyperthreading:    %s (2 threads per core)\n", palette.yellow.Sprint("Yes"))
	default:
		fmt.Printf("  Type:              %s\n", palette.blue.Sprint("Uniform"))
		fmt.Printf("  Total Cores:       %s\n", palette.cyan.Sprint(topology.LogicalCores))
		fmt.Printf("  Hyperthreading:    %s\n", palette.yellow.Sprint("No"))
	}

	fmt.Println()
}

func printCacheHierarchy(info *cpu.Info, palette infoPalette) {
	_, _ = palette.white.Println("💾 CACHE HIERARCHY")

	if info.Topology.HasHeterogeneous && info.PerfL1ICache != "" {
		fmt.Printf("  Performance Cores:\n")
		fmt.Printf("    L1i Cache:       %s\n", palette.green.Sprint(info.PerfL1ICache))
		fmt.Printf("    L1d Cache:       %s\n", palette.green.Sprint(info.PerfL1DCache))
		fmt.Printf("    L2 Cache:        %s\n", palette.green.Sprint(info.PerfL2Cache))
		fmt.Println()

		fmt.Printf("  Efficiency Cores:\n")
		fmt.Printf("    L1i Cache:       %s\n", palette.magenta.Sprint(info.EffL1ICache))
		fmt.Printf("    L1d Cache:       %s\n", palette.magenta.Sprint(info.EffL1DCache))
		fmt.Printf("    L2 Cache:        %s\n", palette.magenta.Sprint(info.EffL2Cache))
	} else {
		printGeneralCaches(info, palette)
	}

	if info.CacheLineSize != "" {
		fmt.Printf("  Cache Line Size:   %s\n", palette.yellow.Sprint(info.CacheLineSize))
	}

	fmt.Println()
}

func printGeneralCaches(info *cpu.Info, palette infoPalette) {
	if info.L1ICache != "" {
		fmt.Printf("  L1i Cache:         %s\n", palette.cyan.Sprint(info.L1ICache))
	}
	if info.L1DCache != "" {
		fmt.Printf("  L1d Cache:         %s\n", palette.cyan.Sprint(info.L1DCache))
	}
	if info.L2Cache != "" {
		fmt.Printf("  L2 Cache:          %s\n", palette.blue.Sprint(info.L2Cache))
	}
	if info.L3Cache != "" {
		fmt.Printf("  L3 Cache:          %s\n", palette.green.Sprint(info.L3Cache))
	}
}

func printMemoryConfiguration(info *cpu.Info, palette infoPalette) {
	_, _ = palette.white.Println("⚙️  MEMORY CONFIGURATION")
	if info.PageSize != "" {
		fmt.Printf("  Page Size:         %s\n", palette.cyan.Sprint(info.PageSize))
	}

	fmt.Println()
}

func printWorkerRecommendations(topology *cpu.Topology, palette infoPalette) {
	_, _ = palette.white.Println("🎯 RECOMMENDED WORKER COUNTS")
	fmt.Printf("  Maximum Load:      %s workers (all cores)\n", palette.cyan.Sprint(topology.LogicalCores))

	switch {
	case topology.HasHeterogeneous:
		fmt.Printf("  P-cores Only:      %s workers (high performance)\n", palette.green.Sprint(topology.PerformanceCores))
		fmt.Printf("  E-cores Only:      %s workers (efficiency)\n", palette.magenta.Sprint(topology.EfficiencyCores))
	case topology.HasHyperthreading:
		fmt.Printf("  Physical Only:     %s workers (no HT/SMT)\n", palette.green.Sprint(topology.PhysicalCores))
	}

	fmt.Println()
}

func printUsageExamples(topology *cpu.Topology, palette infoPalette) {
	_, _ = palette.white.Println("📝 USAGE EXAMPLES")

	switch {
	case topology.HasHeterogeneous:
		fmt.Println("  # Test all cores")
		fmt.Printf("  %s\n", palette.blue.Sprint("yerr cpu load -d 10"))
		fmt.Println()
		fmt.Println("  # Test P-cores only")
		fmt.Printf("  %s\n", palette.green.Sprint("yerr cpu load -c performance -d 10"))
		fmt.Println()
		fmt.Println("  # Test E-cores only")
		fmt.Printf("  %s\n", palette.magenta.Sprint("yerr cpu load -c efficiency -d 10"))
	case topology.HasHyperthreading:
		fmt.Println("  # Test all logical cores")
		fmt.Printf("  %s\n", palette.cyan.Sprint("yerr cpu load -d 10"))
		fmt.Println()
		fmt.Println("  # Test physical cores only")
		fmt.Printf("  %s\n", palette.green.Sprint("yerr cpu load -p -d 10"))
	default:
		fmt.Println("  # Test all cores")
		fmt.Printf("  %s\n", palette.cyan.Sprint("yerr cpu load -d 10"))
	}

	fmt.Println()
}

func printInfoFooter(palette infoPalette) {
	fmt.Println(palette.separator)
	fmt.Println()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
