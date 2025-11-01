package cpu

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		config  TestConfig
		wantErr bool
	}{
		{
			name: "single worker short duration",
			config: TestConfig{
				Workers:  1,
				Duration: 100 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "multiple workers",
			config: TestConfig{
				Workers:  4,
				Duration: 100 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "many workers",
			config: TestConfig{
				Workers:  10,
				Duration: 50 * time.Millisecond,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Run(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Fatal("Run() returned nil result")
			}

			// Verify result fields
			if result.Workers != tt.config.Workers {
				t.Errorf("Result.Workers = %d, want %d", result.Workers, tt.config.Workers)
			}

			if result.TotalOps <= 0 {
				t.Error("Result.TotalOps should be greater than 0")
			}

			if result.OpsPerSecond <= 0 {
				t.Error("Result.OpsPerSecond should be greater than 0")
			}

			// Duration should be close to configured duration (with some tolerance)
			tolerance := 100 * time.Millisecond
			if result.Duration < tt.config.Duration-tolerance || result.Duration > tt.config.Duration+tolerance {
				t.Errorf("Result.Duration = %v, want approximately %v", result.Duration, tt.config.Duration)
			}
		})
	}
}

func TestCPUIntensiveWork(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "very short duration",
			duration: 10 * time.Millisecond,
		},
		{
			name:     "short duration",
			duration: 50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.duration)
			defer cancel()

			ops := cpuIntensiveWork(ctx)

			if ops <= 0 {
				t.Error("cpuIntensiveWork should perform at least one operation")
			}

			// Ensure context timeout was respected
			select {
			case <-ctx.Done():
				// Expected - context should be done
			default:
				t.Error("context should be done after cpuIntensiveWork returns")
			}
		})
	}
}

func TestCPUIntensiveWorkCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Start work in goroutine
	done := make(chan int64, 1)
	go func() {
		ops := cpuIntensiveWork(ctx)
		done <- ops
	}()

	// Cancel after a short delay
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Wait for completion
	select {
	case ops := <-done:
		if ops <= 0 {
			t.Error("cpuIntensiveWork should have performed some operations before cancellation")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("cpuIntensiveWork did not respond to context cancellation")
	}
}

func TestTestResult(t *testing.T) {
	result := &TestResult{
		Duration:       1 * time.Second,
		Workers:        4,
		TotalOps:       1000000,
		OpsPerSecond:   1000000.0,
		CPUUtilization: 95.5,
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

func BenchmarkCPUIntensiveWork(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for i := 0; i < b.N; i++ {
		cpuIntensiveWork(ctx)
	}
}

func BenchmarkRun(b *testing.B) {
	config := TestConfig{
		Workers:  2,
		Duration: 100 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Run(config)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		n        int64
		contains string
	}{
		{
			name:     "single digit",
			n:        5,
			contains: "5",
		},
		{
			name:     "double digit",
			n:        99,
			contains: "99",
		},
		{
			name:     "triple digit",
			n:        999,
			contains: "999",
		},
		{
			name:     "thousand",
			n:        1000,
			contains: ",",
		},
		{
			name:     "million",
			n:        1000000,
			contains: ",",
		},
		{
			name:     "billion",
			n:        1000000000,
			contains: ",",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNumber(tt.n)
			if result == "" {
				t.Error("formatNumber returned empty string")
			}

			if tt.n < 1000 {
				// For numbers < 1000, should not contain commas
				if strings.Contains(result, ",") {
					t.Errorf("formatNumber(%d) = %q, should not contain comma", tt.n, result)
				}
			} else {
				// For numbers >= 1000, should contain commas
				if !strings.Contains(result, ",") {
					t.Errorf("formatNumber(%d) = %q, should contain comma", tt.n, result)
				}
			}

			// Result should contain digits from the original number (without commas)
			expectedStr := fmt.Sprintf("%d", tt.n)
			resultWithoutCommas := strings.ReplaceAll(result, ",", "")
			if !strings.Contains(resultWithoutCommas, expectedStr) {
				t.Errorf("formatNumber(%d) = %q, should contain digits from %d", tt.n, result, tt.n)
			}
		})
	}
}

func TestFormatNumberFloat(t *testing.T) {
	tests := []struct {
		name     string
		n        float64
		contains string
		suffix   string
	}{
		{
			name:     "less than 1000",
			n:        500.5,
			contains: "500.50",
			suffix:   "",
		},
		{
			name:     "thousand",
			n:        1500.0,
			contains: "K",
			suffix:   "K",
		},
		{
			name:     "million",
			n:        1500000.0,
			contains: "M",
			suffix:   "M",
		},
		{
			name:     "billion",
			n:        2500000000.0,
			contains: "B",
			suffix:   "B",
		},
		{
			name:     "fractional million",
			n:        1.5e6,
			contains: "M",
			suffix:   "M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNumberFloat(tt.n)
			if result == "" {
				t.Error("formatNumberFloat returned empty string")
			}

			if tt.suffix != "" {
				if !strings.Contains(result, tt.suffix) {
					t.Errorf("formatNumberFloat(%f) = %q, should contain %q", tt.n, result, tt.suffix)
				}
			}
		})
	}
}

func TestRun_ZeroWorkers(t *testing.T) {
	config := TestConfig{
		Workers:  0,
		Duration: 50 * time.Millisecond,
	}

	// This should still work (though not recommended)
	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run() with 0 workers should not error: %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if result.Workers != 0 {
		t.Errorf("Result.Workers = %d, want 0", result.Workers)
	}
}

func TestRun_VeryShortDuration(t *testing.T) {
	config := TestConfig{
		Workers:  2,
		Duration: 1 * time.Millisecond,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Should have some operations even with very short duration
	if result.TotalOps < 0 {
		t.Error("TotalOps should be >= 0")
	}
}
