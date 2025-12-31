
set -e

echo "UploadStream Observability "
echo "=========================================="

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

#  Install dependencies
echo -e "${BLUE}[1/5]${NC} Installing dependencies..."
go get github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging
go get github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus
go get go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
go get go.uber.org/zap
go get go.opentelemetry.io/sdk/trace
go get go.opentelemetry.io/exporters/stdout/stdouttrace
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
go mod tidy
echo -e "${GREEN}✓${NC} Dependencies installed"

#  Create directory structure
echo -e "${BLUE}[2/5]${NC} Creating directory structure..."
mkdir -p internal/observability
mkdir -p internal/middleware
mkdir -p scripts
mkdir -p data/files
echo -e "${GREEN}✓${NC} Directories created"

# Show file structure
echo -e "${BLUE}[3/5]${NC} Generated files:"
echo "  - internal/observability/logger.go"
echo "  - internal/observability/tracing.go"
echo "  - internal/observability/metrics.go"
echo "  - internal/middleware/logging_interceptor.go"
echo "  - internal/middleware/chain.go"
echo "  - cmd/server/main.go (updated)"
echo -e "${GREEN}✓${NC} All files created"

#  Test compilation
echo -e "${BLUE}[4/5]${NC} Testing compilation..."
go build -o /tmp/uploadstream-test cmd/server/main.go
echo -e "${GREEN}✓${NC} Server compiles successfully"

# Show next steps
echo -e "${BLUE}[5/5]${NC} Next steps:"
echo ""
echo "1. Start the server:"
echo "   ${BLUE}export UPLOADSTREAM='postgres://...'${NC}"
echo "   ${BLUE}export ENV=development${NC}"
echo "   ${BLUE}go run cmd/server/main.go${NC}"
echo ""
echo "2. In another terminal, access:"
echo "   Logs: Console output (real-time)"
echo "   Traces: Console output (JSON)"
echo "   Metrics: ${BLUE}curl http://localhost:9090/metrics${NC}"
echo ""
echo "3. Run the client:"
echo "   ${BLUE}go run cmd/client/main.go${NC}"
echo ""
echo "4. View metrics:"
echo "   ${BLUE}curl http://localhost:9090/metrics | grep grpc_${NC}"
echo ""
echo -e "${GREEN}✓ Setup complete!${NC}"
echo ""
echo "For production setup, see OBSERVABILITY_GUIDE.md"