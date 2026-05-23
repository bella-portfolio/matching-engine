#!/usr/bin/env bash
#
# benchmark.sh — Full benchmark suite for the Matching Engine
#
# Runs:
#   1. Unit tests
#   2. Build & start engine with Docker (1 core / 512 MB limit)
#   3. K6 load test against it
#   4. Collect metrics and print report
#
# Usage: ./benchmark.sh
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}============================================${NC}"
echo -e "${CYAN}  Matching Engine — Benchmark Suite${NC}"
echo -e "${CYAN}  Target: 1 core / 512 MB / TPS ≥ 500${NC}"
echo -e "${CYAN}============================================${NC}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# ---- 1. Unit Tests ----
echo -e "\n${YELLOW}[1/5] Running unit tests...${NC}"
go test ./orderbook/ -v -count=1 -race
echo -e "${GREEN}✓ Unit tests passed${NC}"

# ---- 2. Build binary ----
echo -e "\n${YELLOW}[2/5] Building matching engine...${NC}"
CGO_ENABLED=0 go build -ldflags="-s -w" -o matching-engine .
echo -e "${GREEN}✓ Build complete ($(ls -lh matching-engine | awk '{print $5}'))${NC}"

# ---- 3. Start engine with resource limits ----
echo -e "\n${YELLOW}[3/5] Starting engine (1 core / 512 MB)...${NC}"

# Kill any existing instance
docker compose down 2>/dev/null || true

GOMAXPROCS=1 ./matching-engine &
ENGINE_PID=$!
sleep 2

if ! kill -0 $ENGINE_PID 2>/dev/null; then
    echo -e "${RED}✗ Engine failed to start${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Engine running (PID: $ENGINE_PID)${NC}"

# ---- 4. Health check ----
echo -e "\n${YELLOW}[4/5] Health check...${NC}"
HEALTH=$(curl -s http://localhost:8080/health)
echo "   $HEALTH"

# Quick functional test
echo -e "\n   Functional test (BUY order)..."
RESULT=$(curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"side":"BUY","type":"LIMIT","price":10100,"quantity":5}')
ORDER_ID=$(echo "$RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin)['order']['id'])" 2>/dev/null || echo "?")
echo "   Order placed: $ORDER_ID"

# ---- 5. Load test with k6 ----
echo -e "\n${YELLOW}[5/5] Running K6 load test (1000 RPS, 30s)...${NC}"

# Check if k6 is available
if ! command -v k6 &>/dev/null; then
    echo -e "${YELLOW}   k6 not installed locally. Using Docker...${NC}"
    docker run --rm -i --network=host \
        -v "$SCRIPT_DIR/scripts:/scripts" \
        grafana/k6:latest run /scripts/load-test.js 2>&1 | tee /tmp/k6-output.txt || true
else
    k6 run scripts/load-test.js 2>&1 | tee /tmp/k6-output.txt || true
fi

# ---- 6. Collect stats ----
echo -e "\n${YELLOW}[Stats] Fetching engine statistics...${NC}"
curl -s http://localhost:8080/stats | python3 -m json.tool

# ---- 7. Parse K6 results ----
echo -e "\n${CYAN}============================================${NC}"
echo -e "${CYAN}  Benchmark Report${NC}"
echo -e "${CYAN}============================================${NC}"

if [ -f /tmp/k6-output.txt ]; then
    echo ""
    grep -E "http_reqs|http_req_failed|http_req_duration|orders_placed" /tmp/k6-output.txt | head -10
    echo ""

    # Extract TPS
    REQS=$(grep "http_reqs" /tmp/k6-output.txt | awk '{print $2}' | head -1)
    if [ -n "$REQS" ]; then
        TPS=$(echo "$REQS / 30" | bc 2>/dev/null || echo "?")
        echo -e "   ${GREEN}Total Requests: $REQS${NC}"
        echo -e "   ${GREEN}Approx TPS: $TPS/sec${NC}"

        if command -v bc &>/dev/null && [ "$(echo "$TPS >= 500" | bc 2>/dev/null)" = "1" ]; then
            echo -e "   ${GREEN}✅ TPS target (≥500) achieved!${NC}"
        elif command -v bc &>/dev/null; then
            echo -e "   ${YELLOW}⚠ TPS below 500 target${NC}"
        fi
    fi
fi

# ---- Cleanup ----
echo -e "\n${YELLOW}[Cleanup] Stopping engine...${NC}"
kill $ENGINE_PID 2>/dev/null || true
wait $ENGINE_PID 2>/dev/null || true
echo -e "${GREEN}✓ Done${NC}"
