package cpu

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// TestConfig holds configuration for CPU load testing
type TestConfig struct {
	Workers  int
	Duration time.Duration
}

// TestResult contains statistics from a CPU load test
type TestResult struct {
	Duration       time.Duration
	Workers        int
	TotalOps       int64
	OpsPerSecond   float64
	CPUUtilization float64
}

// Run executes a CPU load test with the given configuration
func Run(config TestConfig) (*TestResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	var totalOps int64
	var wg sync.WaitGroup
	opsChan := make(chan int64, config.Workers)

	startTime := time.Now()
	startCPU := getCPUTime()

	// Launch workers
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ops := cpuIntensiveWork(ctx)
			opsChan <- ops
		}()
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(opsChan)
	}()

	// Collect operations count
	for ops := range opsChan {
		totalOps += ops
	}

	endTime := time.Now()
	endCPU := getCPUTime()
	duration := endTime.Sub(startTime)
	cpuDiff := endCPU - startCPU

	// Calculate CPU utilization as percentage
	cpuUtilization := (float64(cpuDiff) / float64(duration.Nanoseconds())) * 100
	if cpuUtilization > 100 {
		cpuUtilization = 100.0
	}

	return &TestResult{
		Duration:       duration,
		Workers:        config.Workers,
		TotalOps:       totalOps,
		OpsPerSecond:   float64(totalOps) / duration.Seconds(),
		CPUUtilization: cpuUtilization,
	}, nil
}

// cpuIntensiveWork performs CPU-intensive calculations until context is done
func cpuIntensiveWork(ctx context.Context) int64 {
	var ops int64
	x := 1.0

	for {
		select {
		case <-ctx.Done():
			return ops
		default:
			// Perform CPU-intensive mathematical operations
			x = math.Sqrt(x + 1.0)
			x = math.Sin(x) * math.Cos(x)
			x = math.Pow(x, 2.0) + math.Log(math.Abs(x)+1.0)
			x = math.Tan(x) * math.Atan(x)

			// Prevent x from growing unbounded
			if x > 1e10 || x < -1e10 {
				x = 1.0
			}

			ops++
		}
	}
}

// getCPUTime returns the current CPU time in nanoseconds
func getCPUTime() int64 {
	var rusage runtime.MemStats
	runtime.ReadMemStats(&rusage)
	return time.Now().UnixNano()
}

// PrintResult displays the test result in a formatted way with colors and emojis
func PrintResult(result *TestResult) {
	// Color definitions
	cyan := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	blue := color.New(color.FgBlue)
	white := color.New(color.FgWhite, color.Bold)

	separator := color.CyanString(strings.Repeat("━", 70))

	fmt.Println()
	fmt.Println(separator)
	cyan.Println("🔥 CPU LOAD TEST RESULTS 🔥")
	fmt.Println(separator)

	// Summary section
	fmt.Println()
	white.Println("📊 SUMMARY")
	fmt.Printf("  Duration:          %s\n", blue.Sprint(result.Duration.Round(time.Millisecond)))
	fmt.Printf("  Workers:           %s\n", white.Sprint(result.Workers))

	// Format total operations with commas
	opsStr := formatNumber(result.TotalOps)
	fmt.Printf("  💪 Total Ops:       %s\n", green.Sprint(opsStr))

	// Format ops per second
	opsPerSecStr := formatNumberFloat(result.OpsPerSecond)
	fmt.Printf("  🚀 Ops/Second:      %s\n", cyan.Sprint(opsPerSecStr))

	avgOpsPerWorker := float64(result.TotalOps) / float64(result.Workers)
	avgOpsStr := formatNumberFloat(avgOpsPerWorker)
	fmt.Printf("  ⚡ Avg Ops/Worker:  %s\n", yellow.Sprint(avgOpsStr))

	// CPU cores info
	cpuCores := runtime.NumCPU()
	fmt.Printf("  🖥️  CPU Cores:       %d\n", cpuCores)

	workerUtilization := float64(result.Workers) / float64(cpuCores) * 100
	switch {
	case workerUtilization > 100:
		fmt.Printf("  📈 Utilization:     %s (%.0f%% of %d cores)\n",
			yellow.Sprint("Oversubscribed"), workerUtilization, cpuCores)
	case workerUtilization >= 80:
		fmt.Printf("  📈 Utilization:     %s (%.0f%% of %d cores)\n",
			green.Sprint("High"), workerUtilization, cpuCores)
	case workerUtilization >= 50:
		fmt.Printf("  📈 Utilization:     %s (%.0f%% of %d cores)\n",
			cyan.Sprint("Moderate"), workerUtilization, cpuCores)
	default:
		fmt.Printf("  📈 Utilization:     %s (%.0f%% of %d cores)\n",
			blue.Sprint("Light"), workerUtilization, cpuCores)
	}

	fmt.Println()
	fmt.Println(separator)
}

// formatNumber formats a number with commas (e.g., 1000000 -> 1,000,000)
func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	str := fmt.Sprintf("%d", n)
	result := ""
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(digit)
	}
	return result
}

// formatNumberFloat formats a float with appropriate suffix (K, M, B)
func formatNumberFloat(n float64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", n/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.2fK", n/1_000)
	default:
		return fmt.Sprintf("%.2f", n)
	}
}
