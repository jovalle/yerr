<div align="center">

<style>
@keyframes yerr-zoom {
  /* Settled in center */
  0%, 100% {
    transform: translateX(0) scale(1);
    opacity: 1;
  }
  25% {
    transform: translateX(0) scale(1);
    opacity: 1;
  }
  /* Accelerate and exit right */
  40% {
    transform: translateX(150vw) scale(1.5);
    opacity: 0;
  }
  /* Quick zoom left to right - start off screen */
  45% {
    transform: translateX(-130vw) scale(0.8);
    opacity: 0;
  }
  48% {
    transform: translateX(-130vw) scale(0.8);
    opacity: 1;
  }
  58% {
    transform: translateX(150vw) scale(0.8);
    opacity: 1;
  }
  60% {
    transform: translateX(150vw) scale(0.8);
    opacity: 0;
  }
  /* Zoom from left to center and settle */
  63% {
    transform: translateX(-130vw) scale(1.2);
    opacity: 0;
  }
  66% {
    transform: translateX(-130vw) scale(1.2);
    opacity: 1;
  }
  80% {
    transform: translateX(0) scale(1);
    opacity: 1;
  }
}

.yerr-logo {
  animation: yerr-zoom 6s cubic-bezier(0.4, 0, 0.2, 1) infinite;
  transform-origin: center;
}
</style>

<img src=".github/assets/yerr.png" alt="yerr logo" width="200" class="yerr-logo"/>

# `yerr`

A lightweight, cross-platform CPU and HTTP load testing tool written in Go.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Version](https://img.shields.io/github/v/release/jovalle/yerr?label=version)](https://github.com/jovalle/yerr/releases)
[![Build Status](https://github.com/jovalle/yerr/actions/workflows/ci.yml/badge.svg)](https://github.com/jovalle/yerr/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/jovalle/yerr/branch/main/graph/badge.svg)](https://codecov.io/gh/jovalle/yerr)

<!-- [Demo](https://blog4j.me) • [Documentation](https://yerr.github.io) • [Report Bug](https://github.com/jovalle/yerr/issues) • [Request Feature](https://github.com/jovalle/yerr/issues) -->

</div>

## ✨ Features

- **CPU Load Testing**: Stress test and benchmark CPUs with configurable worker and process counts
- **HTTP Load Testing**: Stress test and benchmark web services with concurrent requests
- **Advanced Statistics**: Percentile latency reporting (P50, P90, P95, P99) inspired by wrk
- **Error Categorization**: Detailed error tracking (connection, read, status errors)
- **Cross-Platform**: Runs on Linux, macOS, and Windows
- **Lightweight**: No dependencies required to run, minimal resource overhead
- **Fast**: Built with Go's concurrency primitives for maximum performance

## 🏎️ Inspiration

### Why `yerr`?

While [wg/wrk](https://github.com/wg/wrk) is a legendary HTTP benchmarking tool, it has been largely abandoned and doesn't address the modern CPU topologies that have become mainstay since its creation. `yerr` was created to:

- **Support Modern CPUs**: Native support for heterogeneous computing architectures (P-cores/E-cores) found in Apple Silicon and Intel's 12th+ gen processors
- **Handle HT/SMT**: Proper detection and control over hyperthreading and simultaneous multithreading
- **Cross-Platform**: True cross-platform support with native builds for Windows, Linux, and macOS
- **Modern Go**: Built with modern Go for better concurrency, easier maintenance, and cross-compilation
- **User Experience**: Colorful, emoji-rich output that makes benchmarking fun and informative

## ⁉️ Okay but why "yerr"?

For the fun of it. It's a fusion of that NYC energy and the sound of a fast car passing you on the FDR.

<div align="center"><img src=".github/assets/clout.png" alt="Clout the Street Soldier" width="300"/></div>

## 📲 Installation

### Build Locally

```bash
git clone https://github.com/jovalle/yerr.git
cd yerr
make build

# If you have direnv installed, the bin directory is automatically added to PATH
direnv allow
# Now you can run: yerr cpu load -w 4 -d 5
```

## 🚀 Usage

### CPU Information

View comprehensive CPU information including vendor, model, architecture, cores, cache, and specifications:

```bash
# Show detailed CPU information
yerr cpu info

# Output includes:
# - Vendor (Apple, Intel, AMD)
# - Model name and architecture
# - Core configuration (P-cores/E-cores for heterogeneous CPUs)
# - Cache hierarchy (L1i/L1d/L2/L3)
# - Memory configuration
# - Recommended worker counts
```

### CPU Load Testing

Test CPU performance with configurable workers:

```bash
# Auto-detect and use all logical cores
yerr cpu load

# Use only physical cores, ignore hyperthreading/SMT
yerr cpu load --physical-only

# Run CPU test with 4 workers for 10 seconds
yerr cpu load --workers 4 --duration 10

# Short flags
yerr cpu load -w 8 -d 30 -p
```

**What are CPU Workers?**

A CPU worker is a goroutine (lightweight thread) that performs intensive mathematical operations to stress the CPU. Each worker runs continuously, executing operations like:

- Square roots, trigonometric functions (sin, cos, tan)
- Logarithms and exponentials
- Floating-point arithmetic

**Physical Cores vs Logical Cores:**

Modern CPUs use technologies like Intel's Hyper-Threading (HT) or AMD's Simultaneous Multithreading (SMT) to present 2 logical cores per physical core. For example:

- 4 physical cores + HT = 8 logical cores
- 8 physical cores + SMT = 16 logical cores

By default, yerr detects and uses **all logical cores** for maximum stress testing. Use `--physical-only` to limit workers to physical cores only, which better represents single-threaded performance per core.

**Heterogeneous Computing Architectures (P-cores + E-cores):**

Modern processors like Apple Silicon (M1...M5) and Intel's 12th+ gen use **heterogeneous architectures** with two types of cores:

- **Performance cores (P-cores)**: High-performance, high-power cores for demanding tasks
  - Larger, more complex microarchitecture
  - Higher clock speeds and better single-threaded performance
  - Examples: M3 Pro has 6 P-cores, Intel i9-13900K has 8 P-cores

- **Efficiency cores (E-cores)**: Power-efficient cores for background tasks
  - Smaller, simpler design with lower power consumption
  - Lower clock speeds but excellent for multi-threaded workloads
  - Examples: M3 Pro has 6 E-cores, Intel i9-13900K has 16 E-cores

**Common Configurations:**

- **Apple M1**: 4P+4E (8 cores total)
- **Apple M1 Pro/Max**: 8P+2E or 6P+2E (10 or 8 cores)
- **Apple M3 Pro**: 6P+6E (12 cores)
- **Intel 12900K**: 8P+8E (24 threads: 16 P-threads + 8 E-cores)
- **Intel 13900K**: 8P+16E (32 threads: 16 P-threads + 16 E-cores)

`yerr` automatically detects heterogeneous CPUs and can target specific core types for testing.

**Flags:**

- `-w, --workers`: Number of CPU workers (default: 0 = auto-detect all logical cores)
- `-d, --duration`: Test duration in seconds (default: 0 = run forever until CTRL+C)
- `-p, --physical-only`: Use only physical cores, ignore hyperthreading/SMT (for non-heterogeneous CPUs)
- `-c, --core-type`: Core type to use: `all`, `performance`, `efficiency` (for heterogeneous CPUs)

**Examples for Heterogeneous CPUs:**

```bash
# Test only P-cores (maximum single-threaded performance)
yerr cpu load --core-type performance -d 10

# Test only E-cores (test efficiency cores)
yerr cpu load --core-type efficiency -d 10

# Test all cores (default)
yerr cpu load -d 10

# Short flags
yerr cpu load -c performance -d 10
yerr cpu load -c efficiency -d 10
```

**⚠️ Note:** By default, CPU load tests run indefinitely. You must press CTRL+C to stop the test. Use `-d` to set a specific duration.

### HTTP Load Testing

Benchmark HTTP endpoints:

```bash
# Run 100 requests with 10 concurrent workers
yerr http https://example.com --workers 10 --requests 100

# Run for 30 seconds instead of fixed request count
yerr http https://example.com -w 20 -d 30

# Use POST method
yerr http https://api.example.com/endpoint -X POST -w 5 -n 50
```

**Flags:**

- `-w, --workers`: Number of concurrent workers (default: 10)
- `-n, --requests`: Total number of requests (default: 100)
- `-d, --duration`: Test duration in seconds (0 = use request count)
- `-X, --method`: HTTP method (default: GET)

## 🥽 Examples

### CPU Information

```bash
$ yerr cpu info

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💻 CPU INFORMATION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔧 PROCESSOR
  Vendor:            Apple
  Model:             Apple M1
  Architecture:      arm64
  Machine:           MacBookAir10,1

📊 CORE CONFIGURATION
  Type:              Heterogeneous (Performance + Efficiency)
  Total Cores:       8
  Physical Cores:    8
    • P-cores:       4
    • E-cores:       4
  Hyperthreading:    No

💾 CACHE HIERARCHY
  Performance Cores:
    L1i Cache:       192 KB
    L1d Cache:       128 KB
    L2 Cache:        12.00 MB

  Efficiency Cores:
    L1i Cache:       128 KB
    L1d Cache:       64 KB
    L2 Cache:        4.00 MB
  Cache Line Size:   128 B

⚙️ MEMORY CONFIGURATION
  Page Size:         16 KB

🎯 RECOMMENDED WORKER COUNTS
  Maximum Load:      8 workers (all cores)
  P-cores Only:      4 workers (high performance)
  E-cores Only:      4 workers (efficiency)

📝 USAGE EXAMPLES
  # Test all cores
  yerr cpu load -d 10

  # Test P-cores only
  yerr cpu load -c performance -d 10

  # Test E-cores only
  yerr cpu load -c efficiency -d 10

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### CPU Load Test

```bash
$ yerr cpu load -w 4 -d 5

💻 CPU Info: 8 logical cores (4 physical + SMT/HT)
🏃 Running CPU load test with 4 workers for 5 seconds...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔥 CPU LOAD TEST RESULTS 🔥
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 SUMMARY
  Duration:          5s
  Workers:           4
  💪 Total Ops:      150,617,702
  🚀 Ops/Second:     30.12M
  ⚡ Avg Ops/Worker: 37.65M
  🖥️ CPU Cores:      8
  📈 Utilization:    Moderate (50% of 8 cores)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### HTTP Load Test

```bash
$ yerr http https://example.com -w 10 -n 1000

🏃 Running HTTP load test against https://example.com
   Workers: 10, Requests: 1000, Method: GET

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚡ HTTP LOAD TEST RESULTS ⚡
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 SUMMARY
  Duration:          2.34s
  Total Requests:    1000
  ✅ Successful:     1000 (100.00%)
  🚀 Requests/sec:   427.35
  📦 Total Data:     12.45 MB

⏱️ LATENCY
  Average:           23.20ms
  Min:               15.40ms
  Max:               89.70ms

📈 PERCENTILES
  50% (median):      20.50ms
  90%:               35.20ms
  95%:               45.80ms
  99%:               75.30ms

📋 STATUS CODES
  ✅ 200: 1000

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## 🗂️ Project Structure

```text
yerr/
├── bin/        # Build output for binary (enrc auto imports to PATH)
├── cmd/yerr/   # CLI entry point
└── internal/
    ├── cpu/    # CPU load testing engine
    └── http/   # HTTP load testing engine
```

## { } Why Go?

- **Cross-compilation**: Easy builds for all major platforms
- **Concurrency**: Goroutines provide efficient parallel execution
- **Single Binary**: No runtime dependencies
- **Performance**: Compiled binaries with minimal overhead
- **Standard Library**: Excellent HTTP and networking support built-in

---

<div align="center">

  **[⬆ back to top](#yerr)**

</div>
