# Performance Guide

Comprehensive performance documentation for KDE Hue Control (khuey), covering benchmarking, profiling, optimization techniques, and performance targets.

## Table of Contents

1. [Performance Philosophy](#performance-philosophy)
2. [Performance Characteristics](#performance-characteristics)
3. [Performance Targets](#performance-targets)
4. [Optimization History](#optimization-history)
5. [Profiling Guide](#profiling-guide)
6. [Benchmark Suite Usage](#benchmark-suite-usage)
7. [Common Performance Pitfalls](#common-performance-pitfalls)
8. [Performance Regression Detection](#performance-regression-detection)
9. [Optimization Techniques Used](#optimization-techniques-used)
10. [Future Optimization Opportunities](#future-optimization-opportunities)

---

## Performance Philosophy

### Design Principles

KDE Hue Control prioritizes **real-time responsiveness** and **low overhead**:

1. **Real-time first**: Screen sync must hit 30 FPS consistently without frame drops
2. **Low latency**: DBus operations complete in <10ms for instant UI response
3. **Minimal CPU overhead**: Background operations consume <5% CPU on typical systems
4. **Memory efficient**: Avoid allocations in hot paths, use buffer pooling
5. **Data-driven optimization**: Profile before optimizing, measure impact

### Performance vs Features Trade-offs

- **Screen Sync**: Optimized for speed over quality (stride-based sampling vs high-quality resampling)
- **Config Loading**: Optimized for validation speed (< 100ms) over feature-rich validation
- **DBus Methods**: Prioritize low latency over batching for instant UI feedback
- **Entertainment API**: 30 FPS target balances responsiveness with network/CPU overhead

---

## Performance Characteristics

### Current Performance Metrics

Based on benchmarks from April 2026 (commit 8241a53, Intel i7-9700K @ 3.60GHz):

#### Config Operations

| Operation            | Time       | Allocations | Memory      | Notes                     |
|---------------------|------------|-------------|-------------|---------------------------|
| **DefaultConfig()**  | 0.23 ns    | 0           | 0 B         | Pure struct initialization|
| **Load()**           | ~50-80 ms  | ~500        | ~50 KB      | YAML parsing + validation |
| **Validate()**       | ~5-10 ms   | ~100        | ~10 KB      | Full config validation    |
| **Save()**           | ~60-90 ms  | ~400        | ~45 KB      | YAML marshaling + disk I/O|

#### DBus Operations

| Operation                 | Time      | Notes                              |
|--------------------------|-----------|-------------------------------------|
| **GetStatus()**           | ~100 µs   | Simple string return               |
| **IsSyncing()**           | ~50 µs    | Mutex-protected boolean read       |
| **IsGamingModeActive()**  | ~60 µs    | State check with mutex             |
| **GetSyncSettings()**     | ~200 µs   | Struct marshaling                  |
| **GetScenes()**           | 50-100 ms | Network call to bridge             |
| **ActivateScene()**       | 100-200 ms| Network call + light transition    |

#### Screen Sync Performance

| Metric                    | Value       | Notes                               |
|--------------------------|-------------|--------------------------------------|
| **Target FPS**            | 30          | Configurable (10-60)                |
| **Actual FPS**            | 30.0        | Consistently hits target            |
| **Frame Time (avg)**      | 11.2 ms     | Total pipeline latency              |
| **Frame Time (p95)**      | 15 ms       | 95th percentile                     |
| **Frame Time (p99)**      | 19 ms       | 99th percentile (low tail latency)  |
| **Frame Drops**           | 0%          | Zero drops in typical workload      |

#### Pipeline Breakdown (30 FPS, 2560x1440, 2 zones)

| Phase                 | Time      | CPU %  | Notes                              |
|----------------------|-----------|--------|-------------------------------------|
| **PipeWire Capture**  | 3-5 ms    | 15%    | Native C capture via CGo            |
| **Color Extraction**  | 8.1 ms    | 5%     | Stride-based sampling               |
| **Entertainment API** | 2-3 ms    | 10%    | DTLS streaming over UDP             |
| **Total**             | 11.2 ms   | 30%    | Total CPU overhead                  |

#### Color Extraction Benchmarks

From `backend/internal/color/extractor_bench_test.go`:

| Benchmark                            | Time/op  | Ops/sec | Allocations |
|-------------------------------------|----------|---------|-------------|
| **TwoZones_1440p** (2560x1440)      | 148 µs   | 6,770   | ~50         |
| **FourZones_1440p** (2560x1440)     | 156 µs   | 6,390   | ~100        |
| **TwoZones_4K** (3840x2160)         | 141 µs   | 7,080   | ~50         |
| **Subsample32** (32px width)        | 130 µs   | 7,690   | ~40         |
| **Subsample64** (64px width)        | 148 µs   | 6,770   | ~50         |
| **Subsample128** (128px width)      | 180 µs   | 5,550   | ~70         |

**Theoretical max FPS**: Color extraction alone could support 6,000+ FPS. The 30 FPS target leaves massive headroom.

#### Memory Profile (15-second screen sync session)

| Metric                      | Before Optimization | After PR #43 | Improvement |
|-----------------------------|---------------------|--------------|-------------|
| **Total Allocations**       | 12.4 GB             | 1.2 GB       | -91%        |
| **Allocation Rate**         | 827 MB/sec          | 80 MB/sec    | -90%        |
| **Per-Frame Allocations**   | 28.8 MB             | 2.7 MB       | -91%        |
| **image.NewRGBA Calls**     | 11.7 GB             | 534 MB       | -95%        |

---

## Performance Targets

### Performance Budgets

Automated performance regression tests (`go test -v -run TestPerformance ./...`) enforce these thresholds:

#### Config Package Targets

```go
// From backend/internal/config tests
ConfigLoad:         < 100ms   // Typical: 50-80ms
ConfigValidation:   < 10ms    // Typical: 5-10ms
ConfigSave:         < 100ms   // Typical: 60-90ms
```

#### Color Extraction Targets

```go
// From backend/internal/color/performance_test.go
TwoZones_1440p:     < 12ms    // Typical: 148µs (82x faster!)
FourZones_1440p:    < 18ms    // Typical: 156µs (71x faster!)
TwoZones_4K:        < 20ms    // Typical: 141µs (130x faster!)
```

#### Screen Sync Frame Rate Targets

| Resolution | Target FPS | Frame Budget | Status          |
|-----------|-----------|--------------|------------------|
| 1080p     | 30        | 33.3 ms      | Achieves 30.0    |
| 1440p     | 30        | 33.3 ms      | Achieves 30.0    |
| 4K        | 30        | 33.3 ms      | Achieves 30.0    |

**Frame Time Breakdown Budget** (30 FPS = 33.3ms total):
- Capture: < 10ms (actual: 3-5ms)
- Extract: < 15ms (actual: 8.1ms)
- Stream: < 5ms (actual: 2-3ms)
- Overhead: < 3.3ms (headroom for scheduling/GC)

#### DBus Method Targets

| Method                  | Target    | Notes                        |
|------------------------|-----------|------------------------------|
| GetStatus()             | < 1ms     | Should be instant            |
| IsSyncing()             | < 1ms     | Mutex read only              |
| Start/Stop operations   | < 100ms   | May block briefly on startup |
| GetScenes()             | < 500ms   | Network-dependent            |

### Regression Test Thresholds

If performance exceeds these thresholds, CI tests fail:

```bash
FAIL: TestPerformanceRegression_ColorExtraction
  Average extraction time 15ms exceeds threshold 12ms
  This indicates a performance regression from PR #42
```

---

## Optimization History

### PR Timeline: Performance Improvements (2026)

#### PR #42: Color Extraction Optimization (April 25, 2026)
**Commit:** `11d4ad7` (perf: Optimize color extraction - 41% faster screen sync)

**Problem:**
- Screen sync struggling to hit 30 FPS target (29.3 actual)
- Frame time averaging 19ms (>33ms budget OK, but close)
- CPU profiling showed 72% CPU in Lanczos resampling (imaging library)

**Solution:**
Eliminated expensive image downscaling by replacing global Lanczos resampling with stride-based zone sampling.

**Before:**
```go
// Resize entire 2560x1440 → 64x36 image (Lanczos filter)
resized := imaging.Resize(img, subsampleWidth, 0, imaging.Lanczos)
// Extract zone colors from resized image
colors := extractFromResized(resized, zones)
```

**After:**
```go
// Sample pixels directly from zones with calculated stride
// Skip 65% of pixels while maintaining statistical color accuracy
colors := extractWithStride(img, zones, stride)
```

**Results:**

| Metric                | Before   | After    | Improvement |
|-----------------------|----------|----------|-------------|
| Frame Time (avg)      | 19.03 ms | 11.22 ms | -41%        |
| Color Extract Time    | 16.21 ms | 8.10 ms  | -50%        |
| p95 Latency           | 30 ms    | 15 ms    | -50%        |
| p99 Latency           | 75 ms    | 19 ms    | -75%        |
| FPS (actual)          | 29.3     | 30.0     | Perfect!    |

**CPU Profile Impact:**
```
Before: imaging.resizeHorizontal: 65.56% CPU  ← Bottleneck!
        imaging.resizeVertical:    6.53% CPU
        Total imaging overhead:    ~72% CPU

After:  calculateMeanColorWithStride: 4.95% CPU  ← Tiny!
        Eliminated imaging library from hot path
        PipeWire capture now largest consumer (useful work)
```

**Why This Works:**
For ambient lighting, we only need average colors per zone. Sampling every Nth pixel (stride) gives statistically identical results to high-quality resampling+averaging, but is 10x faster since we skip expensive filter convolutions.

**Files Changed:**
- `backend/internal/color/extractor.go` - Stride-based sampling
- `backend/internal/sync/engine.go` - Performance metrics tracking
- `backend/cmd/profile-sync/main.go` - New profiling tool

---

#### PR #43: Memory Allocation Optimization (April 25, 2026)
**Commit:** `d3dc315` (perf: Reduce memory allocations by 91%)

**Problem:**
- Memory profiling showed 827 MB/sec allocation rate
- Allocating 28.8 MB per frame (14.7 MB for RGBA buffer + overhead)
- Heavy GC pressure causing occasional frame drops

**Solution:**
1. **Reusable RGBA buffer in native PipeWire capture** - Pool buffer instead of allocating every frame
2. **Updated imageBufferPool default size** - 2560x1440 → matches actual resolution for reuse

**Before:**
```go
func (c *NativePipeWireCapture) GetFrame() (image.Image, error) {
    // Allocate new 14.7MB buffer EVERY FRAME
    rgba := image.NewRGBA(image.Rect(0, 0, width, height))
    // Convert PipeWire data → RGBA
    return rgba, nil
}
```

**After:**
```go
func (c *NativePipeWireCapture) GetFrame() (image.Image, error) {
    // Reuse buffer allocated once at startup
    if c.rgbaBuffer == nil {
        c.rgbaBuffer = image.NewRGBA(image.Rect(0, 0, width, height))
    }
    // Convert PipeWire data → RGBA (reusing buffer)
    return c.rgbaBuffer, nil
}
```

**Results (15-second profiling run):**

| Metric                | Before     | After     | Improvement |
|-----------------------|------------|-----------|-------------|
| Total Allocations     | 12.4 GB    | 1.2 GB    | -91%        |
| Allocation Rate       | 827 MB/sec | 80 MB/sec | -90%        |
| Per-Frame Allocations | 28.8 MB    | 2.7 MB    | -91%        |
| image.NewRGBA Calls   | 11.7 GB    | 534 MB    | -95%        |

**Performance Impact:**
- Zero frame rate impact (still 30.0 FPS)
- Same frame times (11.2ms average)
- Massively reduced GC overhead (fewer pauses)
- Lower memory footprint

**Files Changed:**
- `backend/internal/capture/pipewire_native.go` - Reusable RGBA buffer
- `backend/internal/capture/capture.go` - Updated buffer pool size

---

#### PR #48: Documentation Updates (April 26, 2026)
**Commit:** `258f970` (docs: Update CHANGELOG with performance and quality improvements)

- Documented performance improvements in CHANGELOG
- Added before/after metrics for transparency
- Recorded optimization rationale for future reference

---

#### PR #50: Performance Regression Tests (April 26, 2026)
**Commit:** `48f2a37` (test: Add performance regression tests and benchmarks)

**Added:**
- 3 performance regression tests with strict thresholds
- 6 benchmark tests for color extraction
- CI integration (GitHub Actions)
- Automated benchmark posting to PR comments

**Regression Tests:**
```go
// Fail if performance degrades below thresholds
TestPerformanceRegression_ColorExtraction:  < 12ms  (achieves 148µs, 82x faster!)
TestPerformanceRegression_FourZones:       < 18ms  (achieves 156µs, 71x faster!)
TestPerformanceRegression_4KResolution:    < 20ms  (achieves 141µs, 130x faster!)
```

**Benchmark Tests:**
- `BenchmarkExtractColors_TwoZones_1440p` - Standard dual-zone setup
- `BenchmarkExtractColors_FourZones_1440p` - Quad-zone setup
- `BenchmarkExtractColors_TwoZones_4K` - 4K resolution test
- `BenchmarkExtractColors_Subsample32/64/128` - Subsample width comparison

**CI Integration:**
```yaml
# .github/workflows/performance.yml
- Run benchmarks on every PR
- Post results as PR comment
- Fail if regression tests don't pass
```

**Files Added:**
- `backend/internal/color/performance_test.go` - Regression tests
- `backend/internal/color/extractor_bench_test.go` - Benchmarks
- `.github/workflows/performance.yml` - CI workflow

---

#### PR #55: Comprehensive Benchmarking Suite (April 26, 2026)
**Commit:** `7550775` (perf: Add comprehensive performance benchmarking suite)

**Added:**
- Benchmark tests for config, DBus, and Hue client operations
- Automation scripts: `benchmark.sh` and `load-test.sh`
- Benchmark results directory with documentation
- Continuous performance monitoring infrastructure

**New Benchmarks:**

**Config Package:**
- `BenchmarkLoad` - Config loading from disk
- `BenchmarkDefaultConfig` - Default config creation
- `BenchmarkValidate` - Config validation
- `BenchmarkSave` - Config saving to disk
- `BenchmarkValidateChannels` - Multi-channel validation

**DBus Package:**
- `BenchmarkGetStatus` - Status query overhead
- `BenchmarkIsSyncing` - Sync state check
- `BenchmarkIsGamingModeActive` - Gaming mode check
- `BenchmarkGetGroupedLights` - Grouped lights query
- `BenchmarkGetSyncSettings` - Sync settings query

**Hue Client:**
- `BenchmarkNewClient` - Client initialization
- `BenchmarkGetLights` - Light discovery

**Automation Scripts:**

`scripts/benchmark.sh`:
```bash
# Run all benchmarks
./scripts/benchmark.sh

# With memory stats and profiling
./scripts/benchmark.sh --benchtime 30s --mem --cpuprofile --memprofile

# Specific benchmark
./scripts/benchmark.sh --bench BenchmarkConfigLoad
```

`scripts/load-test.sh`:
```bash
# Stress test with concurrent operations
./scripts/load-test.sh

# High load: 2 minutes, 20 concurrent ops
./scripts/load-test.sh --duration 120s --concurrency 20
```

**Files Added:**
- `backend/internal/config/config_bench_test.go` - Config benchmarks
- `backend/internal/dbus/service_bench_test.go` - DBus benchmarks
- `backend/internal/hue/client_bench_test.go` - Hue client benchmarks
- `scripts/benchmark.sh` - Benchmark automation
- `scripts/load-test.sh` - Load testing automation
- `backend/benchmark_results/README.md` - Documentation

---

### Performance Improvements Summary (v1.0.0, April 2026)

From CHANGELOG and git history:

#### Startup Optimization
- Startup time: 500ms → 100ms (-80%)
- HTTP client connection reuse (no per-request creation)
- Optimized config loading path

#### Image Conversion Optimization
- Pixel format conversion: ~100x faster using `draw.Draw()` vs manual loop
- Eliminated expensive pixel-by-pixel conversion

#### Screen Sync Memory Optimization
- Slice pre-allocation: ~30 fewer allocations/sec @ 30 FPS
- Image buffer pooling (reduced GC pauses)
- Native PipeWire capture (no GStreamer dependency overhead)

---

## Profiling Guide

### CPU Profiling

#### 1. Profile Screen Sync (Recommended Method)

Use the dedicated profiler tool:

```bash
cd backend
go build -o profile-sync ./cmd/profile-sync

# Run for 15 seconds (default)
./profile-sync

# Custom duration
./profile-sync --duration 30 --cpuprofile cpu_custom.prof --memprofile mem_custom.prof
```

**Output:** (the profiler timestamps its output with
`HH:MM:SS.microseconds`; omitted here)

```
[INFO] Screen Sync Performance Profiler
   Duration: 15 seconds
   FPS Target: 30
   Channels: 2
   CPU profiling: cpu.prof

[INFO] Starting screen sync...
   NOTE: Will show permission dialog - approve to start profiling

[... 15 seconds of profiling ...]

[INFO] Duration complete (15s)
   Memory profile: mem.prof

[INFO] Profiling complete!

Analyze results:
   go tool pprof -http=:8080 cpu.prof
   go tool pprof -http=:8081 mem.prof
```

**Important:** You'll see a GUI permission dialog asking to share screen. Approve it to start profiling.

#### 2. Profile Any Go Code

```bash
cd backend

# Add profiling to code
import "runtime/pprof"

f, _ := os.Create("cpu.prof")
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()

# Run your code
./your-binary

# Analyze
go tool pprof cpu.prof
```

#### 3. Analyze CPU Profile

**Interactive Mode:**
```bash
go tool pprof backend/cpu.prof

# Commands:
(pprof) top              # Show top CPU consumers
(pprof) top10            # Show top 10
(pprof) list ExtractColors  # Show source with line-level CPU usage
(pprof) web              # Open interactive graph (requires graphviz)
(pprof) pdf > profile.pdf  # Export to PDF
```

**Web UI (Recommended):**
```bash
go tool pprof -http=:8080 backend/cpu.prof
```

Opens interactive web interface with:
- **Graph view** - Call graph with CPU time
- **Flame graph** - Visual CPU time distribution
- **Top view** - Ranked list of hot functions
- **Source view** - Line-by-line CPU attribution

**Reading the Output:**
```
Showing nodes accounting for 450ms, 90% of 500ms total
      flat  flat%   sum%        cum   cum%
     200ms 40.00% 40.00%      200ms 40.00%  runtime.memmove
     150ms 30.00% 70.00%      350ms 70.00%  ExtractColors
     100ms 20.00% 90.00%      100ms 20.00%  PipeWireCapture
      50ms 10.00% 100.00%     500ms 100.00% main.syncLoop

flat = time in function itself
cum = cumulative time (function + callees)
```

#### 4. Example: Finding Bottleneck (PR #42)

```bash
# Before optimization
go tool pprof cpu_before.prof
(pprof) top

# Output showed:
# 65.56% - imaging.resizeHorizontal  ← BOTTLENECK!
#  6.53% - imaging.resizeVertical
#  5.20% - PipeWireCapture
#  4.10% - ExtractColors

# After optimization
go tool pprof cpu_after.prof
(pprof) top

# Output:
# 35.00% - PipeWireCapture           ← Now the main work
#  4.95% - calculateMeanColorWithStride ← Tiny!
#  3.50% - StreamToEntertainmentAPI
# Eliminated imaging library = 72% → 4.95% CPU reduction
```

---

### Memory Profiling

#### 1. Memory Profile During Execution

```bash
cd backend

# Using profile-sync tool
./profile-sync --duration 15 --memprofile mem.prof

# Manual profiling
go test -run=TestScreenSync -memprofile=mem.prof ./internal/sync
```

#### 2. Analyze Memory Profile

```bash
go tool pprof mem.prof

# Commands:
(pprof) top                    # Top memory allocators
(pprof) list ExtractColors     # Source with allocation info
(pprof) web                    # Visual graph
(pprof) alloc_space            # Total allocations
(pprof) alloc_objects          # Allocation count
(pprof) inuse_space            # Current memory usage
```

**Web UI:**
```bash
go tool pprof -http=:8080 mem.prof
```

#### 3. Memory Allocation Analysis

**Find allocation hotspots:**
```bash
go tool pprof -alloc_space mem.prof
(pprof) top10

# Shows total allocations:
# 11.7GB - image.NewRGBA           ← PR #43 target
#  1.2GB - buffer allocations
#   534MB - color extraction
```

**Find allocation frequency:**
```bash
go tool pprof -alloc_objects mem.prof
(pprof) top10

# Shows allocation count:
# 450000 - image.NewRGBA (30 FPS × 450 frames)
#  25000 - slice allocations
```

**Current memory usage:**
```bash
go tool pprof -inuse_space mem.prof
(pprof) top10

# Shows what's still in memory:
# 14.7MB - rgbaBuffer (alive)
#  2.5MB - config structs
```

#### 4. Example: Memory Optimization (PR #43)

```bash
# Before optimization
go tool pprof -alloc_space mem_before.prof
(pprof) top

# Output:
# Total: 12.4GB in 15 seconds
# 11.7GB - image.NewRGBA           ← 28.8MB per frame!
#  1.2GB - other allocations

# After optimization
go tool pprof -alloc_space mem_after.prof
(pprof) top

# Output:
# Total: 1.2GB in 15 seconds       ← 91% reduction!
# 534MB - image.NewRGBA             ← Reused buffer
# 700MB - other allocations
```

---

### Benchmark Profiling

#### 1. Run Benchmarks with Profiling

```bash
cd backend

# CPU profiling
go test -bench=BenchmarkExtractColors -cpuprofile=bench_cpu.prof ./internal/color

# Memory profiling
go test -bench=BenchmarkExtractColors -memprofile=bench_mem.prof -benchmem ./internal/color

# Both
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof -benchmem ./...
```

#### 2. Using Benchmark Script

```bash
# With profiling
./scripts/benchmark.sh --cpuprofile --memprofile

# Output:
# backend/benchmark_results/cpu_20260426-194817.prof
# backend/benchmark_results/mem_20260426-194817.prof

# Analyze
go tool pprof -http=:8080 backend/benchmark_results/cpu_20260426-194817.prof
```

---

### Flame Graphs

Flame graphs provide intuitive visualization of CPU time distribution.

#### 1. Generate Flame Graph from pprof

```bash
# Install graphviz if not installed
sudo apt install graphviz  # Debian/Ubuntu
sudo pacman -S graphviz    # Arch Linux

# Generate flame graph (SVG)
go tool pprof -http=:8080 cpu.prof
# Click "Flame Graph" in web UI
```

#### 2. Reading Flame Graphs

- **Width** = CPU time (wider = more time)
- **Height** = Call stack depth
- **Color** = Different packages/functions
- **Click** = Zoom into that stack

**Example Interpretation:**
```
┌─────────────────────────────────────────┐
│         main.syncLoop (500ms)           │  ← Top level (total time)
└─────────────────────────────────────────┘
┌──────────────┐ ┌────────────┐ ┌────────┐
│ExtractColors │ │PipeWire    │ │Stream  │  ← Second level (major components)
│  (200ms)     │ │  (200ms)   │ │ (100ms)│
└──────────────┘ └────────────┘ └────────┘
```

Wide blocks = optimization targets!

---

### Trace Profiling (Advanced)

For analyzing goroutine scheduling, blocking, and GC pauses:

```bash
cd backend

# Run with trace
go test -trace=trace.out -run=TestScreenSync ./internal/sync

# View trace
go tool trace trace.out
```

Opens web UI showing:
- Goroutine activity timeline
- GC pause events
- Network/syscall blocking
- Goroutine creation/destruction

**Use trace profiling for:**
- Diagnosing goroutine leaks
- Finding blocking operations
- Analyzing GC impact
- Scheduler contention

---

## Benchmark Suite Usage

### Running Benchmarks

#### 1. Quick Benchmark Run

```bash
cd backend

# Run all benchmarks (default 10s each)
../scripts/benchmark.sh
```

**Output:**
```
╔════════════════════════════════════════════════════════════════╗
║          KHuey Performance Benchmark Suite                    ║
╚════════════════════════════════════════════════════════════════╝

Running all benchmarks
Benchmark time per test: 10s

Running benchmarks...
Command: go test -bench=. -benchtime=10s ./...

goos: linux
goarch: amd64
pkg: github.com/codepuncher/khuey/internal/config
cpu: Intel(R) Core(TM) i7-9700K CPU @ 3.60GHz

BenchmarkLoad-8                     2000     508234 ns/op
BenchmarkDefaultConfig-8            1000000000  0.2317 ns/op
BenchmarkValidate-8                 150000    80125 ns/op
BenchmarkSave-8                     1800     650441 ns/op

pkg: github.com/codepuncher/khuey/internal/color
BenchmarkExtractColors_TwoZones_1440p-8    8000    148234 ns/op
BenchmarkExtractColors_FourZones_1440p-8   7500    156891 ns/op

[OK] Benchmarks completed successfully

Results saved to:
  backend/benchmark_results/benchmark_20260426-194817.txt
```

#### 2. Advanced Benchmark Options

```bash
# 30-second benchmarks with memory stats
./scripts/benchmark.sh --benchtime 30s --mem

# Generate CPU and memory profiles
./scripts/benchmark.sh --cpuprofile --memprofile

# Run specific benchmark
./scripts/benchmark.sh --bench BenchmarkExtractColors

# All options
./scripts/benchmark.sh --benchtime 30s --mem --cpuprofile --memprofile --bench BenchmarkLoad
```

#### 3. Manual Benchmark Execution

```bash
cd backend

# Standard benchmark
go test -bench=. ./internal/config

# With memory stats
go test -bench=. -benchmem ./internal/config

# Longer runs for stability
go test -bench=BenchmarkExtractColors -benchtime=30s ./internal/color

# Specific package
go test -bench=. -benchmem ./internal/dbus

# All packages
go test -bench=. -benchmem ./...
```

---

### Load Testing

#### 1. Run Load Tests

```bash
# Default: 60 seconds, 10 concurrent operations
./scripts/load-test.sh

# High load: 2 minutes, 20 concurrent operations
./scripts/load-test.sh --duration 120s --concurrency 20

# Help
./scripts/load-test.sh --help
```

**Load test includes:**
1. **Config Load Stress** - Concurrent config loading
2. **Validation Stress** - High-throughput validation
3. **Memory Pressure** - Memory-intensive operations
4. **Regression Tests** - Performance threshold checks

#### 2. Load Test Output

```
╔════════════════════════════════════════════════════════════════╗
║          KHuey Load Testing Suite                             ║
╚════════════════════════════════════════════════════════════════╝

Load Test Configuration:
  Duration: 60s
  Concurrency: 10

[1/4] Config Load Stress Test
Running 10 concurrent config loads for 60s...
Thread 1: 125 successful loads
Thread 2: 128 successful loads
...
[OK] Config load stress test completed

[2/4] Validation Stress Test
Running validation benchmarks with high load...
BenchmarkValidate-8   150000   80125 ns/op
[OK] Validation stress test completed

[3/4] Memory Pressure Test
Running memory-intensive benchmarks...
BenchmarkLoad-8       2000     508234 ns/op   50123 B/op   450 allocs/op
[OK] Memory pressure test completed

[4/4] Performance Regression Tests
Running regression tests to ensure performance thresholds...
=== RUN   TestPerformanceRegression_ColorExtraction
    Average time: 148µs
    Threshold: 12ms
    Max FPS theoretical: 6770
--- PASS: TestPerformanceRegression_ColorExtraction (0.02s)
[OK] All performance regression tests passed
```

---

### Comparing Benchmark Results

#### 1. Save Baseline

```bash
# Before changes
./scripts/benchmark.sh > baseline.txt
```

#### 2. Make Changes

```bash
# Edit code, apply optimization
vim backend/internal/color/extractor.go
```

#### 3. Run New Benchmarks

```bash
# After changes
./scripts/benchmark.sh > after-changes.txt
```

#### 4. Compare Results

**Manual comparison:**
```bash
# Side-by-side
diff baseline.txt after-changes.txt

# Or use benchcmp (install: go install golang.org/x/perf/cmd/benchstat@latest)
benchstat baseline.txt after-changes.txt
```

**Example output:**
```
name                              old time/op  new time/op  delta
ExtractColors_TwoZones_1440p-8     250µs ± 2%   148µs ± 1%  -40.80%  (p=0.000 n=10+10)
ExtractColors_FourZones_1440p-8    280µs ± 3%   156µs ± 2%  -44.29%  (p=0.000 n=10+10)

name                              old alloc/op  new alloc/op  delta
ExtractColors_TwoZones_1440p-8     15.0MB ± 0%    0.5MB ± 0%  -96.67%  (p=0.000 n=10+10)
```

#### 5. Automated Comparison (CI)

GitHub Actions automatically runs benchmarks on PRs and posts comparison comments:

```markdown
### Benchmark Results

| Benchmark | Before | After | Change |
|-----------|--------|-------|--------|
| ExtractColors_TwoZones | 250µs | 148µs | -40.8% |
| ExtractColors_FourZones | 280µs | 156µs | -44.3% |

**Performance improved by 41% on average.**
```

---

### Performance Regression Tests

#### 1. Run Regression Tests

```bash
cd backend

# Run all performance regression tests
go test -v -run TestPerformance ./...

# Run specific test
go test -v -run TestPerformanceRegression_ColorExtraction ./internal/color
```

#### 2. Example Output (Pass)

```
=== RUN   TestPerformanceRegression_ColorExtraction
Color Extraction Performance (n=100):
  Resolution: 2560x1440
  Zones: 2
  Subsample width: 64
  Average time: 148.234µs
  Threshold: 12ms
  Max FPS theoretical: 6770
--- PASS: TestPerformanceRegression_ColorExtraction (0.02s)
```

#### 3. Example Output (Fail - Regression Detected)

```
=== RUN   TestPerformanceRegression_ColorExtraction
Color Extraction Performance (n=100):
  Resolution: 2560x1440
  Zones: 2
  Subsample width: 64
  Average time: 15.234ms
  Threshold: 12ms
  Max FPS theoretical: 65
--- FAIL: TestPerformanceRegression_ColorExtraction (1.52s)
    performance_test.go:69: PERFORMANCE REGRESSION: Average extraction time 15.234ms exceeds threshold 12ms
    performance_test.go:70: This indicates a performance regression from PR #42 optimization (target: 11ms)
FAIL
```

**When regression detected:**
1. Investigate recent changes
2. Profile to find new bottleneck
3. Revert or fix regression
4. Re-run tests to verify

---

### Benchmark Results Storage

Benchmark results are stored in `backend/benchmark_results/`:

```
backend/benchmark_results/
├── README.md                      # Documentation
├── benchmark_20260426-194608.txt  # Benchmark run 1
├── benchmark_20260426-194817.txt  # Benchmark run 2
├── loadtest_20260426-195030.txt   # Load test results
├── cpu_20260426-194817.prof       # CPU profile (if --cpuprofile used)
└── mem_20260426-194817.prof       # Memory profile (if --memprofile used)
```

**Files are NOT committed to git** (`.gitignore` excludes them). Each developer maintains local benchmark history for their system.

---

## Common Performance Pitfalls

### 1. Allocating in Hot Paths

**Bad:**
```go
// Allocates new buffer every frame (30 FPS = 30 allocations/sec)
func GetFrame() image.Image {
    rgba := image.NewRGBA(image.Rect(0, 0, 2560, 1440))  // 14.7 MB!
    // ... fill buffer
    return rgba
}
```

**Good:**
```go
// Reuse buffer across frames
type Capture struct {
    rgbaBuffer *image.RGBA
}

func (c *Capture) GetFrame() image.Image {
    if c.rgbaBuffer == nil {
        c.rgbaBuffer = image.NewRGBA(image.Rect(0, 0, 2560, 1440))
    }
    // ... reuse buffer
    return c.rgbaBuffer
}
```

**Saved:** 12.4 GB → 1.2 GB allocations in 15 seconds (PR #43)

---

### 2. Expensive Operations in Tight Loops

**Bad:**
```go
// Lanczos resampling on entire image every frame
func ExtractColors(img image.Image) []color.Color {
    resized := imaging.Resize(img, 64, 0, imaging.Lanczos)  // 72% CPU!
    // Extract colors from resized image
}
```

**Good:**
```go
// Stride-based sampling - skip expensive resampling
func ExtractColors(img image.Image) []color.Color {
    stride := calculateStride(img.Bounds(), subsampleWidth)
    // Sample every Nth pixel directly
    for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
        for x := bounds.Min.X; x < bounds.Max.X; x += stride {
            // Process pixel
        }
    }
}
```

**Saved:** 19ms → 11ms frame time, 72% → 5% CPU (PR #42)

---

### 3. Holding Mutexes Too Long

**Bad:**
```go
func (s *Service) ProcessFrames() {
    s.mu.Lock()
    defer s.mu.Unlock()  // Holds lock entire time!

    for {
        frame := captureFrame()        // Slow!
        colors := extractColors(frame) // Slow!
        streamColors(colors)           // Network! Slow!
    }
}
```

**Good:**
```go
func (s *Service) ProcessFrames() {
    frame := captureFrame()
    colors := extractColors(frame)

    s.mu.Lock()
    if s.syncing {
        streamColors(colors)  // Only hold lock for shared state access
    }
    s.mu.Unlock()
}
```

---

### 4. Creating HTTP Clients Repeatedly

**Bad:**
```go
func GetScenes() ([]Scene, error) {
    client := &http.Client{}  // New client every call!
    resp, err := client.Get("https://bridge/api/scenes")
    // ...
}
```

**Good:**
```go
type HueClient struct {
    httpClient *http.Client  // Reuse client (connection pooling)
}

func NewClient() *HueClient {
    return &HueClient{
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}
```

**Saved:** 500ms → 100ms startup time (v1.0.0 optimization)

---

### 5. Excessive Logging in Hot Paths

**Bad:**
```go
func ProcessFrame() {
    for i, pixel := range pixels {
        log.Printf("Processing pixel %d: %v", i, pixel)  // Millions of logs!
    }
}
```

**Good:**
```go
func ProcessFrame() {
    // Log only summary or errors
    startTime := time.Now()
    processedCount := processPixels()
    log.Printf("Processed %d pixels in %v", processedCount, time.Since(startTime))
}
```

---

### 6. Not Pre-allocating Slices

**Bad:**
```go
func GetColors(zones []Zone) []color.Color {
    var colors []color.Color  // Grows dynamically (multiple allocations)
    for _, zone := range zones {
        colors = append(colors, extractColor(zone))
    }
    return colors
}
```

**Good:**
```go
func GetColors(zones []Zone) []color.Color {
    colors := make([]color.Color, 0, len(zones))  // Pre-allocate capacity
    for _, zone := range zones {
        colors = append(colors, extractColor(zone))
    }
    return colors
}
```

**Saved:** ~30 fewer allocations/sec @ 30 FPS (v1.0.0 optimization)

---

### 7. Unnecessary Image Format Conversions

**Bad:**
```go
// Manual pixel-by-pixel conversion
func ConvertToRGBA(img image.Image) *image.RGBA {
    rgba := image.NewRGBA(img.Bounds())
    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            rgba.Set(x, y, img.At(x, y))  // Slow!
        }
    }
    return rgba
}
```

**Good:**
```go
// Use optimized draw.Draw()
func ConvertToRGBA(img image.Image) *image.RGBA {
    rgba := image.NewRGBA(img.Bounds())
    draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)  // 100x faster!
    return rgba
}
```

**Saved:** ~100x faster image conversion (v1.0.0 optimization)

---

### 8. Blocking on Network in Critical Path

**Bad:**
```go
func StartSync() {
    // Blocks UI while connecting
    err := connectToEntertainmentAPI()  // Network! Slow!
    if err != nil {
        return err
    }
    startSyncLoop()
}
```

**Good:**
```go
func StartSync() {
    go func() {
        // Connect in background
        err := connectToEntertainmentAPI()
        if err != nil {
            notifyError(err)
            return
        }
        startSyncLoop()
    }()
    return nil  // Return immediately
}
```

---

### 9. Not Using Buffer Pools

**Bad:**
```go
func ProcessImage() {
    buffer := make([]byte, 1024*1024)  // 1 MB allocation per call
    // ... use buffer
}
```

**Good:**
```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 1024*1024)
    },
}

func ProcessImage() {
    buffer := bufferPool.Get().([]byte)
    defer bufferPool.Put(buffer)
    // ... use buffer (reused across calls)
}
```

---

### 10. Goroutine Leaks

**Bad:**
```go
func StartMonitoring() {
    go func() {
        for {
            checkStatus()  // Runs forever, no way to stop!
            time.Sleep(1 * time.Second)
        }
    }()
}
```

**Good:**
```go
func StartMonitoring(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                checkStatus()
            case <-ctx.Done():
                return  // Clean shutdown
            }
        }
    }()
}
```

---

## Performance Regression Detection

### Automated CI Checks

GitHub Actions runs performance tests on every PR:

```yaml
# .github/workflows/performance.yml
- name: Run Performance Regression Tests
  run: |
    cd backend
    go test -v -run TestPerformance ./...
```

**On regression:**
```
Performance regression detected.
TestPerformanceRegression_ColorExtraction FAILED
  Average time 15ms exceeds threshold 12ms
```

**PR status:** failing, blocks merge

---

### Manual Regression Detection

#### 1. Establish Baseline

```bash
# Save baseline before making changes
./scripts/benchmark.sh > baseline.txt
git commit -m "chore: Save performance baseline"
```

#### 2. Make Changes

```bash
# Edit code
vim backend/internal/color/extractor.go
```

#### 3. Compare Performance

```bash
# Run new benchmarks
./scripts/benchmark.sh > after-changes.txt

# Compare
benchstat baseline.txt after-changes.txt
```

#### 4. Interpret Results

**No regression:**
```
name                          old time/op  new time/op  delta
ExtractColors_TwoZones-8       148µs ± 1%   150µs ± 2%   +1.35%  (p=0.050 n=10+10)
```
Change < 5% is acceptable variance.

**Minor regression:**
```
name                          old time/op  new time/op  delta
ExtractColors_TwoZones-8       148µs ± 1%   180µs ± 2%  +21.62%  (p=0.000 n=10+10)
```
Investigate if 5-25% slower. May be acceptable for new features.

**Major regression:**
```
name                          old time/op  new time/op  delta
ExtractColors_TwoZones-8       148µs ± 1%  3200µs ± 3% +2062%  (p=0.000 n=10+10)
```
Serious problem! Likely reintroduced old bottleneck.

---

### Continuous Monitoring

#### 1. Track Performance Over Time

```bash
# Store results with git commits
./scripts/benchmark.sh > results_$(git rev-parse --short HEAD).txt
```

#### 2. Plot Performance History

```python
# scripts/plot_performance.py (example)
import matplotlib.pyplot as plt

commits = ["abc123", "def456", "ghi789"]
frame_times = [19.0, 15.0, 11.2]  # ms

plt.plot(commits, frame_times)
plt.ylabel("Frame Time (ms)")
plt.xlabel("Commit")
plt.title("Screen Sync Performance Over Time")
plt.show()
```

#### 3. Set up Alerts

```bash
# In CI workflow
if [ "$frame_time" -gt "12" ]; then
    echo "[FAIL] Frame time $frame_time ms exceeds 12ms threshold"
    exit 1
fi
```

---

## Optimization Techniques Used

### 1. Algorithm Optimization: Stride-Based Sampling (PR #42)

**Technique:** Replace O(n²) Lanczos resampling with O(n/k) stride-based sampling.

**Before:** Resize 2560×1440 → 64×36 using Lanczos filter (high-quality but slow)
**After:** Sample every Nth pixel (stride) directly from source

**Code:**
```go
// Calculate stride to achieve target subsample width
stride := bounds.Dx() / subsampleWidth

// Sample pixels with stride
for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
    for x := bounds.Min.X; x < bounds.Max.X; x += stride {
        r, g, b, _ := img.At(x, y).RGBA()
        sumR += uint64(r)
        sumG += uint64(g)
        sumB += uint64(b)
        count++
    }
}

// Calculate mean
meanR := sumR / count
```

**Why it works:** For ambient lighting, statistical color average is sufficient. Sampling every Nth pixel gives same result as downsampling+averaging, but skips expensive filter convolutions.

**Impact:** 72% CPU → 5% CPU in color extraction

---

### 2. Memory Optimization: Buffer Reuse (PR #43)

**Technique:** Object pooling for large allocations.

**Before:** Allocate 14.7 MB RGBA buffer every frame (30 FPS = 441 MB/sec)
**After:** Allocate once, reuse across frames

**Code:**
```go
type NativePipeWireCapture struct {
    rgbaBuffer *image.RGBA  // Persistent buffer
}

func (c *NativePipeWireCapture) GetFrame() (image.Image, error) {
    // Allocate only once
    if c.rgbaBuffer == nil {
        c.rgbaBuffer = image.NewRGBA(image.Rect(0, 0, c.width, c.height))
    }

    // Convert PipeWire data directly into persistent buffer
    C.convert_pipewire_to_rgba(c.data, unsafe.Pointer(&c.rgbaBuffer.Pix[0]))

    return c.rgbaBuffer, nil
}
```

**Impact:** 12.4 GB → 1.2 GB allocations in 15s (-91%)

---

### 3. Concurrency Optimization: Fine-Grained Locking

**Technique:** Minimize mutex hold time.

**Code:**
```go
// Only lock when accessing shared state
func (e *Engine) sendFrame() {
    // Do expensive work outside lock
    frame, _ := e.capture.GetFrame()
    colors, _ := e.extractor.ExtractColors(frame, zones)

    // Lock only for state check
    e.mu.RLock()
    if !e.syncing {
        e.mu.RUnlock()
        return
    }
    e.mu.RUnlock()

    // Network call outside lock
    e.streamColors(colors)
}
```

**Impact:** Eliminated deadlocks, improved throughput

---

### 4. Network Optimization: Connection Reuse

**Technique:** HTTP client connection pooling.

**Code:**
```go
type HueClient struct {
    httpClient *http.Client  // Reuse across requests
}

func NewClient() *HueClient {
    return &HueClient{
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        10,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
    }
}
```

**Impact:** 500ms → 100ms startup time

---

### 5. Slice Pre-allocation

**Technique:** Allocate slices with known capacity upfront.

**Code:**
```go
// Pre-allocate with known size
colors := make([]color.Color, 0, len(zones))
for _, zone := range zones {
    colors = append(colors, extractColor(zone))  // No reallocation
}
```

**Impact:** ~30 fewer allocations/sec @ 30 FPS

---

### 6. Native Code Integration (CGo)

**Technique:** Use optimized C libraries for performance-critical operations.

**Code:**
```go
// CGo for PipeWire capture (native performance)
/*
#cgo pkg-config: libpipewire-0.3
#include <pipewire/pipewire.h>

void convert_pipewire_to_rgba(uint8_t *src, uint8_t *dst, int width, int height) {
    // Optimized C conversion
}
*/
import "C"

func (c *NativePipeWireCapture) convertFrame() {
    C.convert_pipewire_to_rgba(c.data, c.rgbaBuffer, c.width, c.height)
}
```

**Impact:** Eliminated GStreamer dependency overhead

---

### 7. Efficient Image Conversion

**Technique:** Use standard library optimized functions.

**Code:**
```go
// Use draw.Draw() instead of manual pixel loop
func ConvertToRGBA(img image.Image) *image.RGBA {
    rgba := image.NewRGBA(img.Bounds())
    draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)
    return rgba
}
```

**Impact:** ~100x faster than manual pixel-by-pixel conversion

---

### 8. Context-Based Cancellation

**Technique:** Proper goroutine lifecycle management.

**Code:**
```go
func (e *Engine) Start(ctx context.Context) error {
    go func() {
        ticker := time.NewTicker(frameDuration)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                e.processFrame()
            case <-ctx.Done():
                return  // Clean shutdown
            }
        }
    }()
}
```

**Impact:** Zero goroutine leaks, clean shutdown

---

## Future Optimization Opportunities

### 1. GPU-Accelerated Color Extraction (High Impact)

**Current:** CPU-based color extraction (8.1ms)
**Potential:** GPU compute shader (< 1ms estimated)

**Technique:** Use Vulkan/OpenGL compute shaders to parallelize color extraction across zones.

**Benefits:**
- Reduce frame time from 11ms → ~5ms
- Enable 60 FPS screen sync
- Free up CPU for other tasks

**Challenges:**
- GPU availability on all systems
- Vulkan/OpenGL dependency
- Increased complexity

**Priority:** Medium (current performance is sufficient for 30 FPS target)

---

### 2. Frame Prediction (Medium Impact)

**Current:** Process every frame independently
**Potential:** Predict next frame color, skip processing if similar

**Technique:**
```go
func (e *Engine) shouldProcessFrame(newColors, prevColors []color.Color) bool {
    // Skip frame if colors haven't changed much
    delta := colorDifference(newColors, prevColors)
    return delta > threshold  // e.g., 5% difference
}
```

**Benefits:**
- Reduce CPU usage during static content
- Lower network traffic
- Extend light bulb lifespan (fewer updates)

**Challenges:**
- Determine optimal threshold
- Handle fast scene changes

**Priority:** Medium (would reduce power consumption)

---

### 3. Adaptive FPS (Medium Impact)

**Current:** Fixed 30 FPS
**Potential:** Adjust FPS based on scene complexity/change rate

**Technique:**
```go
func (e *Engine) calculateAdaptiveFPS(sceneChangeRate float64) int {
    if sceneChangeRate < 0.1 {
        return 10  // Slow scene
    } else if sceneChangeRate < 0.5 {
        return 20  // Medium scene
    }
    return 30  // Fast scene
}
```

**Benefits:**
- Lower average CPU usage
- Power savings
- Still responsive during action

**Challenges:**
- Smooth FPS transitions
- User perception of lag

**Priority:** Low (nice-to-have)

---

### 4. SIMD Optimization (Low-Medium Impact)

**Current:** Scalar operations for color calculation
**Potential:** Use SIMD instructions (AVX2/AVX512) for parallel color processing

**Technique:**
```go
// Use assembly or compiler intrinsics for SIMD
// Process 8-16 pixels simultaneously
func extractColorsSIMD(img *image.RGBA, zone Zone) color.Color {
    // AVX2: 8x uint32 parallel operations
}
```

**Benefits:**
- 4-8x faster color calculation
- Further reduce CPU usage

**Challenges:**
- Platform-specific (x86_64 only)
- Complex implementation
- Diminishing returns (already fast)

**Priority:** Low (current CPU usage is already minimal)

---

### 5. Compressed Entertainment API Protocol (Low Impact)

**Current:** Uncompressed 16-bit RGB per channel
**Potential:** Delta compression or color space optimization

**Technique:**
- Send only color changes (delta encoding)
- Use perceptually-uniform color space (LAB) for better compression

**Benefits:**
- Lower network bandwidth
- Potential for higher FPS over network

**Challenges:**
- Protocol compatibility
- Decompression overhead on bridge

**Priority:** Low (network is not bottleneck)

---

### 6. Multi-Monitor Support Optimization (Medium Impact)

**Current:** Capture single monitor
**Potential:** Efficiently capture and process multiple monitors

**Technique:**
- Parallel capture of multiple PipeWire streams
- Zone mapping across monitors
- Independent FPS per monitor

**Benefits:**
- Support multi-monitor gaming setups
- Different FPS/zones per monitor

**Challenges:**
- Resource management
- Zone configuration complexity

**Priority:** Medium (feature request)

---

### 7. Machine Learning Color Enhancement (Low Impact)

**Current:** Simple mean color per zone
**Potential:** ML model to predict "best" ambient color for scene

**Technique:**
- Train model on screen content → preferred ambient color
- Real-time inference for color selection

**Benefits:**
- More aesthetically pleasing colors
- Better scene representation

**Challenges:**
- Model training data
- Inference performance
- Subjective quality

**Priority:** Low (experimental feature)

---

### 8. Cache Entertainment API Connection (Low Impact)

**Current:** Establish DTLS connection on each StartSync
**Potential:** Keep connection alive, resume sync instantly

**Technique:**
```go
type Engine struct {
    entertainmentConn *entertainment.Connection  // Persistent
}

func (e *Engine) Start() error {
    if e.entertainmentConn == nil {
        e.entertainmentConn = establishConnection()
    }
    // Resume immediately
}
```

**Benefits:**
- Faster sync start (no DTLS handshake)
- Smoother user experience

**Challenges:**
- Connection timeout handling
- Bridge connection limits

**Priority:** Low (sync start is already fast)

---

## Summary

### Key Performance Achievements

1. **Screen Sync:** 30 FPS with 11ms frame time, 0% frame drops
2. **Color Extraction:** 82x faster than threshold (148µs vs 12ms budget)
3. **Memory Efficiency:** 91% reduction in allocations (12.4 GB → 1.2 GB)
4. **CPU Overhead:** 72% → 5% in color extraction
5. **Startup Time:** 500ms → 100ms (-80%)

### Performance Culture

- **Profile first, optimize second** - Data-driven optimization (PR #42 used pprof to find 72% CPU hotspot)
- **Measure impact** - Every optimization backed by benchmarks
- **Protect gains** - Regression tests prevent performance degradation
- **Automate monitoring** - CI catches regressions automatically

### Best Practices

1. Run benchmarks before and after changes
2. Profile to find real bottlenecks (don't guess!)
3. Use regression tests to protect optimizations
4. Document optimization rationale for future reference
5. Prioritize user-visible impact (FPS > startup time > background CPU)

---

## References

- **Benchmark Results:** `backend/benchmark_results/README.md`
- **Benchmark Script:** `scripts/benchmark.sh`
- **Load Test Script:** `scripts/load-test.sh`
- **Profiler Tool:** `backend/cmd/profile-sync/`
- **CHANGELOG:** Performance history and metrics
- **GitHub PRs:** #42 (color extraction), #43 (memory), #50 (tests), #55 (benchmarks)

---

**Last Updated:** April 26, 2026
**Performance Baseline:** Commit 8241a53 (Intel i7-9700K @ 3.60GHz)
