#!/bin/bash
# Load testing script for khuey backend
# Simulates high load scenarios and stress tests critical operations

set -e

# pkg-config for libpipewire emits -fno-strict-overflow, which cgo rejects
# unless it is allowlisted. Needed by any go command that reaches
# internal/capture.
export CGO_CFLAGS_ALLOW='-fno-strict-overflow'

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"
RESULTS_DIR="$BACKEND_DIR/benchmark_results"

# Ensure results directory exists
mkdir -p "$RESULTS_DIR"

# Generate timestamp
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
RESULTS_FILE="$RESULTS_DIR/loadtest_${TIMESTAMP}.txt"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          KHuey Load Testing Suite                             ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Parse command line arguments
DURATION="60s"
CONCURRENCY=10

while [[ $# -gt 0 ]]; do
    case $1 in
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --concurrency)
            CONCURRENCY="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --duration <time>        Duration to run load tests (default: 60s)"
            echo "  --concurrency <n>        Number of concurrent operations (default: 10)"
            echo "  --help                   Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                # Run with defaults"
            echo "  $0 --duration 120s --concurrency 20"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

echo -e "${YELLOW}Load Test Configuration:${NC}"
echo -e "  Duration: $DURATION"
echo -e "  Concurrency: $CONCURRENCY"
echo ""

# Change to backend directory
cd "$BACKEND_DIR"

# Write header to results file
{
    echo "KHuey Load Test Results"
    echo "======================="
    echo ""
    echo "Timestamp: $(date)"
    echo "Hostname: $(hostname)"
    echo "Duration: $DURATION"
    echo "Concurrency: $CONCURRENCY"
    echo "Go Version: $(go version)"
    echo "Git Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
    echo ""
    echo "========================================"
    echo ""
} > "$RESULTS_FILE"

# Test 1: Config Load Stress Test
echo -e "${GREEN}[1/4] Config Load Stress Test${NC}"
echo "Testing concurrent config loading operations..."
echo ""

{
    echo "Test 1: Config Load Stress Test"
    echo "--------------------------------"
    echo ""
} >> "$RESULTS_FILE"

# Create temporary config for testing
TEMP_CONFIG_DIR=$(mktemp -d)
TEMP_CONFIG_FILE="$TEMP_CONFIG_DIR/config.yaml"
cat > "$TEMP_CONFIG_FILE" << 'EOF'
Bridge: 192.168.1.100
Key: test-api-key-1234567890abcdefghijklmnopqrstuvwxyz
clientkey: test-client-key-abcdefghijklmnopqrstuvwxyz1234567890
entertainmentConfigurationId: abc123-def456-ghi789
sync:
  fps: 30
  subsampleWidth: 64
channels:
  - id: 0
    active: true
    deviceName: "Light 1"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 1.0, y: 1.0}
log_level: info
EOF

# Run concurrent config load test
START_TIME=$(date +%s)

echo "Running $CONCURRENCY concurrent config loads for $DURATION..."
timeout "$DURATION" bash -c "
    for i in {1..$CONCURRENCY}; do
        (
            count=0
            while true; do
                XDG_CONFIG_HOME='$TEMP_CONFIG_DIR' go test -run TestLoadValidFile -timeout 5s ./internal/config > /dev/null 2>&1
                if [ \$? -eq 0 ]; then
                    count=\$((count + 1))
                    echo \"Thread \$i: \$count successful loads\"
                fi
            done
        ) &
    done
    wait
" 2>&1 | tee -a "$RESULTS_FILE" || true

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

{
    echo ""
    echo "Config Load Test Results:"
    echo "  Duration: ${ELAPSED}s"
    echo "  Concurrency: $CONCURRENCY"
    echo ""
} >> "$RESULTS_FILE"

# Cleanup
rm -rf "$TEMP_CONFIG_DIR"

echo -e "${GREEN}✓ Config load stress test completed${NC}"
echo ""

# Test 2: Validation Stress Test
echo -e "${GREEN}[2/4] Validation Stress Test${NC}"
echo "Testing concurrent validation operations..."
echo ""

{
    echo "Test 2: Validation Stress Test"
    echo "-------------------------------"
    echo ""
} >> "$RESULTS_FILE"

# Run validation benchmark with high iteration count
echo "Running validation benchmarks with high load..."
go test -bench=BenchmarkValidate -benchtime="$DURATION" ./internal/config 2>&1 | tee -a "$RESULTS_FILE"

echo -e "${GREEN}✓ Validation stress test completed${NC}"
echo ""

# Test 3: Memory Pressure Test
echo -e "${GREEN}[3/4] Memory Pressure Test${NC}"
echo "Testing memory usage under load..."
echo ""

{
    echo "Test 3: Memory Pressure Test"
    echo "-----------------------------"
    echo ""
} >> "$RESULTS_FILE"

# Run benchmarks with memory profiling
echo "Running memory-intensive benchmarks..."
go test -bench=. -benchmem -benchtime=30s ./internal/config 2>&1 | tee -a "$RESULTS_FILE"

echo -e "${GREEN}✓ Memory pressure test completed${NC}"
echo ""

# Test 4: Performance Regression Tests
echo -e "${GREEN}[4/4] Performance Regression Tests${NC}"
echo "Running regression tests to ensure performance thresholds..."
echo ""

{
    echo "Test 4: Performance Regression Tests"
    echo "-------------------------------------"
    echo ""
} >> "$RESULTS_FILE"

# Run performance regression tests
if go test -v -run TestPerformance ./... 2>&1 | tee -a "$RESULTS_FILE"; then
    echo -e "${GREEN}✓ All performance regression tests passed${NC}"
else
    echo -e "${RED}✗ Some performance regression tests failed${NC}"
    echo -e "${YELLOW}Check results for details on performance degradation${NC}"
fi

echo ""

# Summary
echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Load Test Summary                                            ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

{
    echo ""
    echo "========================================"
    echo "Load Test Summary"
    echo "========================================"
    echo ""
    echo "All tests completed at $(date)"
    echo ""
    echo "Results saved to: $RESULTS_FILE"
    echo ""
} >> "$RESULTS_FILE"

echo -e "${BLUE}Results saved to:${NC}"
echo -e "  ${YELLOW}$RESULTS_FILE${NC}"
echo ""

echo -e "${YELLOW}Load testing complete!${NC}"
echo ""
echo -e "${YELLOW}Recommendations:${NC}"
echo -e "  - Review results for any performance degradation"
echo -e "  - Compare with previous load test results"
echo -e "  - Run regression tests regularly: go test -v -run TestPerformance ./..."
echo -e "  - Monitor memory usage in production with: go tool pprof"
echo ""
