#!/bin/bash
# Benchmark automation script for khuey backend
# Runs all benchmark tests and saves results with timestamp

set -eo pipefail

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
RESULTS_FILE="$RESULTS_DIR/benchmark_${TIMESTAMP}.txt"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          KHuey Performance Benchmark Suite                    ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Parse command line arguments
BENCHTIME="10s"
MEMORY="false"
CPU_PROFILE="false"
MEM_PROFILE="false"
SPECIFIC_BENCH=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --benchtime)
            BENCHTIME="$2"
            shift 2
            ;;
        --mem)
            MEMORY="true"
            shift
            ;;
        --cpuprofile)
            CPU_PROFILE="true"
            shift
            ;;
        --memprofile)
            MEM_PROFILE="true"
            shift
            ;;
        --bench)
            SPECIFIC_BENCH="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --benchtime <duration>   Time to run each benchmark (default: 10s)"
            echo "  --mem                    Include memory allocation stats"
            echo "  --cpuprofile             Generate CPU profile"
            echo "  --memprofile             Generate memory profile"
            echo "  --bench <pattern>        Run specific benchmark pattern"
            echo "  --help                   Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Run all benchmarks (10s each)"
            echo "  $0 --benchtime 30s --mem             # 30s benchmarks with memory stats"
            echo "  $0 --bench BenchmarkConfigLoad       # Run specific benchmark"
            echo "  $0 --cpuprofile --memprofile         # Generate profiling data"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Build benchmark command
BENCH_CMD="go test -bench="

if [ -n "$SPECIFIC_BENCH" ]; then
    BENCH_CMD+="$SPECIFIC_BENCH"
    echo -e "${YELLOW}Running specific benchmark: $SPECIFIC_BENCH${NC}"
else
    BENCH_CMD+="."
    echo -e "${YELLOW}Running all benchmarks${NC}"
fi

BENCH_CMD+=" -benchtime=$BENCHTIME"

if [ "$MEMORY" = "true" ]; then
    BENCH_CMD+=" -benchmem"
    echo -e "${YELLOW}Including memory allocation stats${NC}"
fi

if [ "$CPU_PROFILE" = "true" ]; then
    BENCH_CMD+=" -cpuprofile=$RESULTS_DIR/cpu_${TIMESTAMP}.prof"
    echo -e "${YELLOW}CPU profiling enabled${NC}"
fi

if [ "$MEM_PROFILE" = "true" ]; then
    BENCH_CMD+=" -memprofile=$RESULTS_DIR/mem_${TIMESTAMP}.prof"
    echo -e "${YELLOW}Memory profiling enabled${NC}"
fi

BENCH_CMD+=" ./..."

echo -e "${YELLOW}Benchmark time per test: $BENCHTIME${NC}"
echo ""

# Change to backend directory
cd "$BACKEND_DIR"

# Write header to results file
{
    echo "KHuey Performance Benchmark Results"
    echo "===================================="
    echo ""
    echo "Timestamp: $(date)"
    echo "Hostname: $(hostname)"
    echo "Go Version: $(go version)"
    echo "Git Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
    echo "Git Branch: $(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'unknown')"
    echo ""
    echo "Benchmark Configuration:"
    echo "  - Bench Time: $BENCHTIME"
    echo "  - Memory Stats: $MEMORY"
    echo "  - CPU Profile: $CPU_PROFILE"
    echo "  - Mem Profile: $MEM_PROFILE"
    if [ -n "$SPECIFIC_BENCH" ]; then
        echo "  - Specific Bench: $SPECIFIC_BENCH"
    fi
    echo ""
    echo "========================================"
    echo ""
} > "$RESULTS_FILE"

# Run benchmarks
echo -e "${GREEN}Running benchmarks...${NC}"
echo -e "${BLUE}Command: $BENCH_CMD${NC}"
echo ""

# Run and capture output
if $BENCH_CMD | tee -a "$RESULTS_FILE"; then
    echo ""
    echo -e "${GREEN}[OK]${NC} Benchmarks completed successfully"
    echo ""
    echo -e "${BLUE}Results saved to:${NC}"
    echo -e "  ${YELLOW}$RESULTS_FILE${NC}"
    
    if [ "$CPU_PROFILE" = "true" ]; then
        echo -e "${BLUE}CPU profile saved to:${NC}"
        echo -e "  ${YELLOW}$RESULTS_DIR/cpu_${TIMESTAMP}.prof${NC}"
        echo -e "  ${BLUE}View with: go tool pprof $RESULTS_DIR/cpu_${TIMESTAMP}.prof${NC}"
    fi
    
    if [ "$MEM_PROFILE" = "true" ]; then
        echo -e "${BLUE}Memory profile saved to:${NC}"
        echo -e "  ${YELLOW}$RESULTS_DIR/mem_${TIMESTAMP}.prof${NC}"
        echo -e "  ${BLUE}View with: go tool pprof $RESULTS_DIR/mem_${TIMESTAMP}.prof${NC}"
    fi
    
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  Benchmark Summary                                            ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # Extract and display summary
    echo -e "${BLUE}Top 5 slowest operations:${NC}"
    # grep exits 1 when --bench matched no benchmark
    { grep -E '^Benchmark[^[:space:]]+[[:space:]]+[0-9]' "$RESULTS_FILE" || [ $? -eq 1 ]; } |
        sort -k3 -n -r | head -5 | while read -r line; do
        echo "  $line"
    done
    
    echo ""
    echo -e "${YELLOW}Tip: Compare results over time to detect performance regressions${NC}"
    echo -e "${YELLOW}Tip: Run 'go test -v -run TestPerformance' for regression tests${NC}"
    
else
    echo ""
    echo -e "${RED}[FAIL]${NC} Benchmarks failed"
    echo -e "${RED}Check output above for errors${NC}"
    exit 1
fi
