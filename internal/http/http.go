// Package http provides HTTP load testing functionality with configurable
// workers, request counts, and methods. It tracks detailed metrics including
// latency percentiles, status codes, and error types.
package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
)

// TestConfig holds configuration for HTTP load testing
type TestConfig struct {
	URL         string
	Method      string
	Workers     int
	TotalReqs   int
	Duration    time.Duration
	Timeout     time.Duration
	UseDuration bool // If true, run for Duration; if false, run for TotalReqs
	Headers     map[string][]string
	Body        []byte
	BasicAuth   *BasicAuth
	BearerToken string
}

// BasicAuth configures HTTP Basic authentication for requests.
type BasicAuth struct {
	Username string
	Password string
}

// TestResult contains statistics from an HTTP load test
//
//nolint:govet // fieldalignment: readability is more important than minimal padding
type TestResult struct {
	Duration      time.Duration
	AvgLatency    time.Duration
	MinLatency    time.Duration
	MaxLatency    time.Duration
	P50Latency    time.Duration
	P90Latency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	TotalRequests int64
	SuccessCount  int64
	FailureCount  int64
	ConnErrors    int64
	ReadErrors    int64
	StatusErrors  int64
	TotalBytes    int64
	ReqsPerSecond float64
	Latencies     []time.Duration // Bounded sample used for percentile calculations
	LatencyCount  int64           // Total latency observations represented (success + HTTP status errors)
	Sampled       bool            // True when Latencies is a sampled subset of observations
	StatusCodes   map[int]int64
}

type errorType int

const (
	errNone errorType = iota
	errConnect
	errRead
	errStatus
)

type requestResult struct {
	err        error
	errType    errorType
	latency    time.Duration
	bytes      int64
	statusCode int
}

const maxLatencySamples = 100_000

// Run executes an HTTP load test with the given configuration
//
//nolint:gocyclo // complexity is inherent to load testing orchestration
func Run(config TestConfig) (*TestResult, error) {
	if err := validateConfig(&config); err != nil {
		return nil, err
	}
	config.Method = strings.ToUpper(strings.TrimSpace(config.Method))
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	var ctx context.Context
	var cancel context.CancelFunc

	if config.UseDuration {
		ctx, cancel = context.WithTimeout(context.Background(), config.Duration)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        config.Workers,
			MaxIdleConnsPerHost: config.Workers,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	var (
		requestsCompleted atomic.Int64
		successCount      atomic.Int64
		failureCount      atomic.Int64
		connErrors        atomic.Int64
		readErrors        atomic.Int64
		statusErrors      atomic.Int64
		totalBytes        atomic.Int64
		minLatency        = time.Duration(1<<63 - 1) // Max duration
		maxLatency        = time.Duration(0)
		totalLatency      = time.Duration(0)
		latencies         []time.Duration
		latencyCount      int64
		sampledLatencies  bool
		statusCodes       = make(map[int]int64)
		statusMutex       sync.Mutex
		latencyMutex      sync.Mutex
	)

	startTime := time.Now()
	sampleRNG := rand.New(rand.NewSource(startTime.UnixNano())) //nolint:gosec // reservoir sampling needs speed, not crypto
	var wg sync.WaitGroup
	resultsChan := make(chan requestResult, config.Workers*2)
	collectorDone := make(chan struct{})

	// Result collector
	go func() {
		defer close(collectorDone)
		for result := range resultsChan {
			// Record status code if we got one
			if result.statusCode > 0 {
				statusMutex.Lock()
				statusCodes[result.statusCode]++
				statusMutex.Unlock()
			}

			if result.err == nil {
				successCount.Add(1)
				totalBytes.Add(result.bytes)

				latencyMutex.Lock()
				sampledLatencies = addLatencySample(&latencies, &latencyCount, result.latency, sampleRNG) || sampledLatencies
				totalLatency += result.latency
				if result.latency < minLatency {
					minLatency = result.latency
				}
				if result.latency > maxLatency {
					maxLatency = result.latency
				}
				latencyMutex.Unlock()
			} else {
				failureCount.Add(1)
				switch result.errType {
				case errConnect:
					connErrors.Add(1)
				case errRead:
					readErrors.Add(1)
				case errStatus:
					statusErrors.Add(1)
					// For status errors, still add bytes and track latency
					if result.statusCode > 0 {
						totalBytes.Add(result.bytes)
						latencyMutex.Lock()
						sampledLatencies = addLatencySample(&latencies, &latencyCount, result.latency, sampleRNG) || sampledLatencies
						totalLatency += result.latency
						if result.latency < minLatency {
							minLatency = result.latency
						}
						if result.latency > maxLatency {
							maxLatency = result.latency
						}
						latencyMutex.Unlock()
					}
				}
			}
		}
	}()

	// Launch workers
	if config.UseDuration {
		// Duration-based testing
		for i := 0; i < config.Workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						result := makeRequest(ctx, client, &config)
						requestsCompleted.Add(1)
						resultsChan <- result
					}
				}
			}()
		}
	} else {
		// Request count-based testing
		requestChan := make(chan struct{}, config.TotalReqs)
		for i := 0; i < config.TotalReqs; i++ {
			requestChan <- struct{}{}
		}
		close(requestChan)

		for i := 0; i < config.Workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range requestChan {
					result := makeRequest(ctx, client, &config)
					requestsCompleted.Add(1)
					resultsChan <- result
				}
			}()
		}
	}

	// Wait for all workers to complete
	wg.Wait()
	close(resultsChan)
	<-collectorDone

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	completed := requestsCompleted.Load()
	success := successCount.Load()

	// Read latency metrics under lock
	latencyMutex.Lock()
	avgLatency := time.Duration(0)
	if latencyCount > 0 {
		avgLatency = totalLatency / time.Duration(latencyCount)
	}

	if minLatency == time.Duration(1<<63-1) {
		minLatency = 0
	}

	finalMinLatency := minLatency
	finalMaxLatency := maxLatency

	// Calculate percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := calculatePercentile(latencies, 50)
	p90 := calculatePercentile(latencies, 90)
	p95 := calculatePercentile(latencies, 95)
	p99 := calculatePercentile(latencies, 99)
	latencyMutex.Unlock()

	reqsPerSecond := 0.0
	if duration > 0 {
		reqsPerSecond = float64(completed) / duration.Seconds()
	}

	return &TestResult{
		Duration:      duration,
		TotalRequests: completed,
		SuccessCount:  success,
		FailureCount:  failureCount.Load(),
		ConnErrors:    connErrors.Load(),
		ReadErrors:    readErrors.Load(),
		StatusErrors:  statusErrors.Load(),
		TotalBytes:    totalBytes.Load(),
		ReqsPerSecond: reqsPerSecond,
		AvgLatency:    avgLatency,
		MinLatency:    finalMinLatency,
		MaxLatency:    finalMaxLatency,
		P50Latency:    p50,
		P90Latency:    p90,
		P95Latency:    p95,
		P99Latency:    p99,
		StatusCodes:   statusCodes,
		Latencies:     latencies,
		LatencyCount:  latencyCount,
		Sampled:       sampledLatencies,
	}, nil
}

func addLatencySample(samples *[]time.Duration, observed *int64, latency time.Duration, rng *rand.Rand) bool {
	*observed++
	if len(*samples) < maxLatencySamples {
		*samples = append(*samples, latency)
		return false
	}

	// Reservoir sampling keeps a uniform sample without unbounded memory growth.
	idx := rng.Int63n(*observed)
	if idx < int64(maxLatencySamples) {
		(*samples)[idx] = latency
	}

	return true
}

func validateConfig(config *TestConfig) error {
	if strings.TrimSpace(config.URL) == "" {
		return fmt.Errorf("URL must not be empty")
	}
	if strings.TrimSpace(config.Method) == "" {
		return fmt.Errorf("HTTP method must not be empty")
	}
	if config.Workers <= 0 {
		return fmt.Errorf("workers must be > 0")
	}
	if config.Timeout < 0 {
		return fmt.Errorf("timeout must be >= 0")
	}
	if config.BasicAuth != nil && strings.TrimSpace(config.BasicAuth.Username) == "" {
		return fmt.Errorf("basic auth username must not be empty")
	}
	if config.BasicAuth != nil && strings.TrimSpace(config.BearerToken) != "" {
		return fmt.Errorf("basic auth and bearer token are mutually exclusive")
	}

	if config.UseDuration {
		if config.Duration <= 0 {
			return fmt.Errorf("duration must be > 0 when duration mode is enabled")
		}
		return nil
	}

	if config.TotalReqs <= 0 {
		return fmt.Errorf("total requests must be > 0 when duration mode is disabled")
	}

	return nil
}

// calculatePercentile returns the value at the given percentile (0-100)
func calculatePercentile(sorted []time.Duration, percentile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	index := (percentile / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	// Linear interpolation
	weight := index - float64(lower)
	return time.Duration(float64(sorted[lower])*(1-weight) + float64(sorted[upper])*weight)
}

func makeRequest(ctx context.Context, client *http.Client, config *TestConfig) requestResult {
	var bodyReader io.Reader = http.NoBody
	if len(config.Body) > 0 {
		bodyReader = bytes.NewReader(config.Body)
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, bodyReader)
	if err != nil {
		return requestResult{err: err, errType: errConnect}
	}
	for key, values := range config.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if config.BasicAuth != nil {
		req.SetBasicAuth(config.BasicAuth.Username, config.BasicAuth.Password)
	}
	if strings.TrimSpace(config.BearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(config.BearerToken))
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		// Categorize connection errors
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "no such host") ||
			strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "dial") {
			return requestResult{latency: latency, err: err, errType: errConnect}
		}
		return requestResult{latency: latency, err: err, errType: errRead}
	}
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		// Read error
		return requestResult{
			statusCode: resp.StatusCode,
			latency:    latency,
			bytes:      0,
			err:        err,
			errType:    errRead,
		}
	}

	// Check for HTTP errors (4xx, 5xx)
	if resp.StatusCode >= 400 {
		return requestResult{
			statusCode: resp.StatusCode,
			latency:    latency,
			bytes:      bytes,
			err:        fmt.Errorf("HTTP %d", resp.StatusCode),
			errType:    errStatus,
		}
	}

	return requestResult{
		statusCode: resp.StatusCode,
		latency:    latency,
		bytes:      bytes,
		err:        nil,
		errType:    errNone,
	}
}

// PrintResult displays the test result in a formatted way with colors and emojis
func PrintResult(result *TestResult) {
	// Color definitions
	cyan := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	blue := color.New(color.FgBlue)
	white := color.New(color.FgWhite, color.Bold)

	separator := color.CyanString(strings.Repeat("━", 70))

	fmt.Println()
	fmt.Println(separator)
	_, _ = cyan.Println("⚡ HTTP LOAD TEST RESULTS ⚡")
	fmt.Println(separator)

	// Summary section
	fmt.Println()
	_, _ = white.Println("📊 SUMMARY")
	fmt.Printf("  Duration:          %s\n", blue.Sprint(result.Duration.Round(time.Millisecond)))
	fmt.Printf("  Total Requests:    %s\n", white.Sprint(result.TotalRequests))

	successPct := 0.0
	failurePct := 0.0
	if result.TotalRequests > 0 {
		successPct = float64(result.SuccessCount) / float64(result.TotalRequests) * 100
		failurePct = float64(result.FailureCount) / float64(result.TotalRequests) * 100
	}

	switch {
	case successPct > 95:
		fmt.Printf("  ✅ Successful:      %s (%.2f%%)\n", green.Sprint(result.SuccessCount), successPct)
	case successPct > 90:
		fmt.Printf("  ⚠️  Successful:      %s (%.2f%%)\n", yellow.Sprint(result.SuccessCount), successPct)
	default:
		fmt.Printf("  ✅ Successful:      %s (%.2f%%)\n", white.Sprint(result.SuccessCount), successPct)
	}

	if result.FailureCount > 0 {
		fmt.Printf("  ❌ Failed:          %s (%.2f%%)\n", red.Sprint(result.FailureCount), failurePct)
		if result.ConnErrors > 0 {
			fmt.Printf("     └─ Connection:   %s\n", red.Sprint(result.ConnErrors))
		}
		if result.ReadErrors > 0 {
			fmt.Printf("     └─ Read:         %s\n", red.Sprint(result.ReadErrors))
		}
		if result.StatusErrors > 0 {
			fmt.Printf("     └─ Status:       %s\n", red.Sprint(result.StatusErrors))
		}
	}

	fmt.Printf("  🚀 Requests/sec:    %s\n", cyan.Sprintf("%.2f", result.ReqsPerSecond))

	dataMB := float64(result.TotalBytes) / (1024 * 1024)
	var dataStr string
	if dataMB >= 1000 {
		dataStr = fmt.Sprintf("%.2f GB", dataMB/1024)
	} else {
		dataStr = fmt.Sprintf("%.2f MB", dataMB)
	}
	fmt.Printf("  📦 Total Data:      %s\n", blue.Sprint(dataStr))

	// Latency section
	fmt.Println()
	_, _ = white.Println("⏱️ LATENCY")
	fmt.Printf("  Average:           %s\n", formatLatency(result.AvgLatency))
	fmt.Printf("  Min:               %s\n", green.Sprint(formatLatency(result.MinLatency)))
	fmt.Printf("  Max:               %s\n", red.Sprint(formatLatency(result.MaxLatency)))

	// Percentiles
	fmt.Println()
	_, _ = white.Println("📈 PERCENTILES")
	if result.Sampled {
		fmt.Printf("  Note:              %s\n", yellow.Sprintf("Approximate (sampled %d of %d latencies)", len(result.Latencies), result.LatencyCount))
	}
	fmt.Printf("  50%% (median):      %s\n", formatLatency(result.P50Latency))
	fmt.Printf("  90%%:               %s\n", formatLatency(result.P90Latency))
	fmt.Printf("  95%%:               %s\n", yellow.Sprint(formatLatency(result.P95Latency)))
	fmt.Printf("  99%%:               %s\n", red.Sprint(formatLatency(result.P99Latency)))

	// Status codes
	if len(result.StatusCodes) > 0 {
		fmt.Println()
		_, _ = white.Println("📋 STATUS CODES")

		// Sort status codes
		codes := make([]int, 0, len(result.StatusCodes))
		for code := range result.StatusCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)

		for _, code := range codes {
			count := result.StatusCodes[code]
			emoji := getStatusEmoji(code)
			codeColor := getStatusColor(code)
			fmt.Printf("  %s %s: %d\n", emoji, codeColor.Sprint(code), count)
		}
	}

	fmt.Println()
	fmt.Println(separator)
}

func formatLatency(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.2fμs", float64(d.Microseconds()))
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func getStatusEmoji(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "✅"
	case code >= 300 && code < 400:
		return "↪️"
	case code >= 400 && code < 500:
		return "⚠️"
	case code >= 500:
		return "❌"
	default:
		return "❓"
	}
}

func getStatusColor(code int) *color.Color {
	switch {
	case code >= 200 && code < 300:
		return color.New(color.FgGreen)
	case code >= 300 && code < 400:
		return color.New(color.FgCyan)
	case code >= 400 && code < 500:
		return color.New(color.FgYellow)
	case code >= 500:
		return color.New(color.FgRed)
	default:
		return color.New(color.FgWhite)
	}
}
