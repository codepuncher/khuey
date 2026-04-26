# Benchmark Results

This directory stores performance benchmark results for the khuey backend.

## Contents

- `benchmark_*.txt` - Timestamped benchmark results from `scripts/benchmark.sh`
- `loadtest_*.txt` - Load test results from `scripts/load-test.sh`
- `cpu_*.prof` - CPU profiles (when using `--cpuprofile` flag)
- `mem_*.prof` - Memory profiles (when using `--memprofile` flag)

## Running Benchmarks

### Basic Benchmark Run

```bash
./scripts/benchmark.sh
```

This runs all benchmarks with default settings (10 seconds per benchmark).

### Advanced Options

```bash
# Run for 30 seconds with memory stats
./scripts/benchmark.sh --benchtime 30s --mem

# Generate CPU and memory profiles
./scripts/benchmark.sh --cpuprofile --memprofile

# Run specific benchmark
./scripts/benchmark.sh --bench BenchmarkConfigLoad

# Help
./scripts/benchmark.sh --help
```

## Running Load Tests

### Basic Load Test

```bash
./scripts/load-test.sh
```

This runs stress tests for 60 seconds with 10 concurrent operations.

### Advanced Options

```bash
# Run for 2 minutes with 20 concurrent operations
./scripts/load-test.sh --duration 120s --concurrency 20

# Help
./scripts/load-test.sh --help
```

## Performance Regression Tests

Run regression tests to ensure performance stays within acceptable thresholds:

```bash
cd backend
go test -v -run TestPerformance ./...
```

These tests fail if critical operations exceed their performance budgets:
- Config load: < 100ms
- Config validation: < 10ms
- Config save: < 100ms
- Client creation: < 500ms
- Multiple operations: < 500ms total

## Viewing Profiles

### CPU Profile

```bash
go tool pprof backend/benchmark_results/cpu_20240101-120000.prof
```

Inside pprof:
- `top` - Show top functions by CPU time
- `list <function>` - Show source code with CPU usage
- `web` - Open interactive graph (requires graphviz)

### Memory Profile

```bash
go tool pprof backend/benchmark_results/mem_20240101-120000.prof
```

Inside pprof:
- `top` - Show top functions by memory allocation
- `list <function>` - Show source code with memory usage
- `web` - Open interactive graph

## Interpreting Results

### Benchmark Output Format

```
BenchmarkConfigLoad-8    5000    250000 ns/op    12345 B/op    123 allocs/op
│                  │     │       │                │             │
│                  │     │       │                │             └─ Allocations per operation
│                  │     │       │                └─ Bytes allocated per operation
│                  │     │       └─ Nanoseconds per operation
│                  │     └─ Number of iterations
│                  └─ GOMAXPROCS value
└─ Benchmark name
```

### What to Look For

**Performance Improvements:**
- Lower ns/op (faster operations)
- Lower B/op (less memory allocated)
- Lower allocs/op (fewer allocations)

**Performance Regressions:**
- Increasing ns/op over time
- Increasing memory usage
- More allocations

### Comparison Example

```bash
# Save baseline
./scripts/benchmark.sh > baseline.txt

# Make changes...

# Compare after changes
./scripts/benchmark.sh > after-changes.txt

# Use benchcmp to compare (install: go install golang.org/x/tools/cmd/benchcmp@latest)
benchcmp baseline.txt after-changes.txt
```

## Best Practices

1. **Run benchmarks regularly** - Establish baseline performance
2. **Compare over time** - Detect gradual performance degradation
3. **Test before and after changes** - Validate optimization work
4. **Profile when needed** - Use CPU/memory profiles to find bottlenecks
5. **Run regression tests** - Catch major performance issues early

## Git Ignore

Benchmark results are not committed to git. Each developer runs benchmarks locally to measure performance on their system.

Add custom patterns to `.git/info/exclude` if needed.
