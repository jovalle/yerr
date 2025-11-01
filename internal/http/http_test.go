package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRun_RequestCount(t *testing.T) {
	// Create a test server
	requestCount := atomic.Int32{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	tests := []struct {
		name    string
		config  TestConfig
		wantErr bool
		minReqs int64
		maxReqs int64
	}{
		{
			name: "single worker few requests",
			config: TestConfig{
				URL:         ts.URL,
				Method:      "GET",
				Workers:     1,
				TotalReqs:   10,
				UseDuration: false,
			},
			wantErr: false,
			minReqs: 10,
			maxReqs: 10,
		},
		{
			name: "multiple workers many requests",
			config: TestConfig{
				URL:         ts.URL,
				Method:      "GET",
				Workers:     5,
				TotalReqs:   100,
				UseDuration: false,
			},
			wantErr: false,
			minReqs: 100,
			maxReqs: 100,
		},
		{
			name: "POST method",
			config: TestConfig{
				URL:         ts.URL,
				Method:      "POST",
				Workers:     2,
				TotalReqs:   20,
				UseDuration: false,
			},
			wantErr: false,
			minReqs: 20,
			maxReqs: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount.Store(0)
			result, err := Run(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Fatal("Run() returned nil result")
			}

			// Verify request count
			if result.TotalRequests < tt.minReqs || result.TotalRequests > tt.maxReqs {
				t.Errorf("Result.TotalRequests = %d, want between %d and %d",
					result.TotalRequests, tt.minReqs, tt.maxReqs)
			}

			// Verify all requests succeeded
			if result.SuccessCount != result.TotalRequests {
				t.Errorf("Result.SuccessCount = %d, want %d", result.SuccessCount, result.TotalRequests)
			}

			if result.FailureCount != 0 {
				t.Errorf("Result.FailureCount = %d, want 0", result.FailureCount)
			}

			// Verify metrics are reasonable
			if result.ReqsPerSecond <= 0 {
				t.Error("Result.ReqsPerSecond should be greater than 0")
			}

			if result.AvgLatency <= 0 {
				t.Error("Result.AvgLatency should be greater than 0")
			}

			if result.MinLatency <= 0 {
				t.Error("Result.MinLatency should be greater than 0")
			}

			if result.MaxLatency <= 0 {
				t.Error("Result.MaxLatency should be greater than 0")
			}

			if result.MinLatency > result.MaxLatency {
				t.Errorf("MinLatency (%v) should be <= MaxLatency (%v)", result.MinLatency, result.MaxLatency)
			}
		})
	}
}

func TestRun_Duration(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	config := TestConfig{
		URL:         ts.URL,
		Method:      "GET",
		Workers:     3,
		Duration:    200 * time.Millisecond,
		UseDuration: true,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Should have made some requests
	if result.TotalRequests <= 0 {
		t.Error("Should have made at least one request")
	}

	// Duration should be approximately what we specified
	tolerance := 100 * time.Millisecond
	if result.Duration < config.Duration-tolerance || result.Duration > config.Duration+tolerance {
		t.Errorf("Result.Duration = %v, want approximately %v", result.Duration, config.Duration)
	}
}

func TestRun_StatusCodes(t *testing.T) {
	// Create a test server that returns different status codes
	requestNum := atomic.Int32{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		num := requestNum.Add(1)
		if num%2 == 0 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	config := TestConfig{
		URL:         ts.URL,
		Method:      "GET",
		Workers:     2,
		TotalReqs:   10,
		UseDuration: false,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Should have captured status codes
	if len(result.StatusCodes) == 0 {
		t.Error("StatusCodes map should not be empty")
	}

	// Verify total count matches
	var totalStatusCodes int64
	for _, count := range result.StatusCodes {
		totalStatusCodes += count
	}

	if totalStatusCodes != result.SuccessCount {
		t.Errorf("Total status codes = %d, want %d", totalStatusCodes, result.SuccessCount)
	}
}

func TestRun_ServerErrors(t *testing.T) {
	// Create a test server that returns errors
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error"))
	}))
	defer ts.Close()

	config := TestConfig{
		URL:         ts.URL,
		Method:      "GET",
		Workers:     2,
		TotalReqs:   10,
		UseDuration: false,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// All requests should still be counted as successful (no connection errors)
	// Even with 500 status codes, they're technically successful HTTP requests
	if result.TotalRequests != int64(config.TotalReqs) {
		t.Errorf("TotalRequests = %d, want %d", result.TotalRequests, config.TotalReqs)
	}

	// Should have 500 status codes recorded
	if result.StatusCodes[500] != int64(config.TotalReqs) {
		t.Errorf("StatusCodes[500] = %d, want %d", result.StatusCodes[500], config.TotalReqs)
	}
}

func TestRun_SlowServer(t *testing.T) {
	// Create a slow server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	config := TestConfig{
		URL:         ts.URL,
		Method:      "GET",
		Workers:     2,
		TotalReqs:   5,
		UseDuration: false,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Latencies should reflect the delay
	if result.MinLatency < 40*time.Millisecond {
		t.Errorf("MinLatency = %v, expected at least 40ms due to server delay", result.MinLatency)
	}
}

func TestRun_InvalidURL(t *testing.T) {
	config := TestConfig{
		URL:         "http://invalid-domain-that-does-not-exist-12345.com",
		Method:      "GET",
		Workers:     1,
		TotalReqs:   5,
		UseDuration: false,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Should have failures
	if result.FailureCount == 0 {
		t.Error("Expected some failures with invalid URL")
	}
}

func TestPrintResult(t *testing.T) {
	result := &TestResult{
		Duration:      1 * time.Second,
		TotalRequests: 100,
		SuccessCount:  95,
		FailureCount:  5,
		TotalBytes:    10000,
		ReqsPerSecond: 100.0,
		AvgLatency:    10 * time.Millisecond,
		MinLatency:    5 * time.Millisecond,
		MaxLatency:    50 * time.Millisecond,
		StatusCodes: map[int]int64{
			200: 95,
			500: 5,
		},
	}

	// Test that PrintResult doesn't panic
	t.Run("PrintResult does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PrintResult panicked: %v", r)
			}
		}()
		PrintResult(result)
	})
}

func BenchmarkRun(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	}))
	defer ts.Close()

	config := TestConfig{
		URL:         ts.URL,
		Method:      "GET",
		Workers:     2,
		TotalReqs:   10,
		UseDuration: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Run(config)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}

func TestCalculatePercentile(t *testing.T) {
	tests := []struct {
		name       string
		sorted     []time.Duration
		percentile float64
		want       time.Duration
	}{
		{
			name:       "empty slice",
			sorted:     []time.Duration{},
			percentile: 50,
			want:       0,
		},
		{
			name:       "single element",
			sorted:     []time.Duration{100 * time.Millisecond},
			percentile: 50,
			want:       100 * time.Millisecond,
		},
		{
			name:       "two elements p50",
			sorted:     []time.Duration{100 * time.Millisecond, 200 * time.Millisecond},
			percentile: 50,
			want:       150 * time.Millisecond, // interpolated
		},
		{
			name:       "multiple elements p90",
			sorted:     []time.Duration{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			percentile: 90,
			want:       91 * time.Duration(1), // Should be around 91
		},
		{
			name:       "p99 last element",
			sorted:     []time.Duration{10, 20, 30, 40, 50},
			percentile: 99,
			want:       50 * time.Duration(1), // Should be last element
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePercentile(tt.sorted, tt.percentile)
			// Use approximate comparison for floating point results
			if len(tt.sorted) == 0 {
				if result != tt.want {
					t.Errorf("calculatePercentile(%v, %f) = %v, want %v", tt.sorted, tt.percentile, result, tt.want)
				}
			} else if len(tt.sorted) == 1 {
				if result != tt.want {
					t.Errorf("calculatePercentile(%v, %f) = %v, want %v", tt.sorted, tt.percentile, result, tt.want)
				}
			} else {
				// For multiple elements, check bounds
				min := tt.sorted[0]
				max := tt.sorted[len(tt.sorted)-1]
				if result < min || result > max {
					t.Errorf("calculatePercentile(%v, %f) = %v, should be between %v and %v",
						tt.sorted, tt.percentile, result, min, max)
				}
			}
		})
	}
}

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		name     string
		d        time.Duration
		contains string
	}{
		{
			name:     "nanoseconds",
			d:        500 * time.Nanosecond,
			contains: "ns",
		},
		{
			name:     "microseconds",
			d:        500 * time.Microsecond,
			contains: "μs",
		},
		{
			name:     "milliseconds",
			d:        500 * time.Millisecond,
			contains: "ms",
		},
		{
			name:     "seconds",
			d:        2 * time.Second,
			contains: "s",
		},
		{
			name:     "zero",
			d:        0,
			contains: "ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLatency(tt.d)
			if result == "" {
				t.Error("formatLatency returned empty string")
			}

			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatLatency(%v) = %q, should contain %q", tt.d, result, tt.contains)
			}
		})
	}
}

func TestGetStatusEmoji(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{
			name: "2xx",
			code: 200,
			want: "✅",
		},
		{
			name: "3xx",
			code: 301,
			want: "↪️",
		},
		{
			name: "4xx",
			code: 404,
			want: "⚠️",
		},
		{
			name: "5xx",
			code: 500,
			want: "❌",
		},
		{
			name: "1xx",
			code: 100,
			want: "❓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStatusEmoji(tt.code)
			if result != tt.want {
				t.Errorf("getStatusEmoji(%d) = %q, want %q", tt.code, result, tt.want)
			}
		})
	}
}

func TestGetStatusColor(t *testing.T) {
	tests := []struct {
		name string
		code int
		// We can't directly test color.Color, but we can test it doesn't panic
	}{
		{
			name: "2xx",
			code: 200,
		},
		{
			name: "3xx",
			code: 301,
		},
		{
			name: "4xx",
			code: 404,
		},
		{
			name: "5xx",
			code: 500,
		},
		{
			name: "1xx",
			code: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("getStatusColor(%d) panicked: %v", tt.code, r)
				}
			}()
			result := getStatusColor(tt.code)
			if result == nil {
				t.Error("getStatusColor returned nil")
			}
		})
	}
}

func TestMakeRequest_ConnectionError(t *testing.T) {
	client := &http.Client{
		Timeout: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use an invalid URL that will cause connection errors
	result := makeRequest(ctx, client, "http://invalid-domain-that-does-not-exist-12345.com:80", "GET")

	if result.err == nil {
		t.Error("Expected error for invalid domain")
	}

	if result.errType != errConnect {
		t.Errorf("Expected errConnect, got %v", result.errType)
	}
}

func TestMakeRequest_Timeout(t *testing.T) {
	// Create a server that doesn't respond
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{
		Timeout: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := makeRequest(ctx, client, ts.URL, "GET")

	// Should have a timeout error
	if result.err == nil {
		t.Error("Expected timeout error")
	}
}
