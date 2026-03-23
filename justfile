default:
    @just --list

# ── Quality gate ─────────────────────────────────────────────

# Run all checks (format + lint + vet + test + build)
all: fmt vet lint test test-frontend build

# ── Testing ──────────────────────────────────────────────────

# Run Go tests
test:
    go test ./... -count=1

# Run Go tests with verbose output
test-verbose:
    go test ./... -count=1 -v

# Run Go coverage report
coverage:
    go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

# Run frontend tests
test-frontend:
    cd frontend && npm run test

# Run frontend tests in watch mode
test-frontend-watch:
    cd frontend && npm run test:watch

# ── Linting & Formatting ────────────────────────────────────

# Format Go code
fmt:
    find . \( -path ./.go -o -path ./bin -o -path ./frontend \) -prune -o -name '*.go' -print | xargs gofumpt -w

# Run go vet
vet:
    go vet ./...

# Run golangci-lint
lint:
    golangci-lint run ./cmd/... ./internal/...

# Lint frontend
lint-frontend:
    cd frontend && npm run lint

# Run Go vulnerability check
vuln:
    govulncheck ./...

# ── Building ─────────────────────────────────────────────────

# Build frontend
build-frontend:
    cd frontend && npm run build

# Build the symphony binary (with embedded frontend)
build: build-frontend
    cp -r frontend/dist/* internal/status/dist/ 2>/dev/null || mkdir -p internal/status/dist && cp -r frontend/dist/* internal/status/dist/
    CGO_ENABLED=0 go build -o bin/symphony ./cmd/symphony

# Build Go binary only (no frontend rebuild)
build-go:
    CGO_ENABLED=0 go build -o bin/symphony ./cmd/symphony

# Clean build artifacts
clean:
    rm -rf bin/ coverage.out frontend/dist/ internal/status/dist/

# ── Docker ───────────────────────────────────────────────────

# Build container image
docker-build:
    docker build -t symphony -f deploy/Dockerfile .

# ── Development ──────────────────────────────────────────────

# Run symphony with a local workflow
run *args: build
    ./bin/symphony --workflow WORKFLOW.md {{args}}

# Start frontend dev server (proxies API to localhost:8080)
dev-frontend:
    cd frontend && npm run dev

# Install frontend dependencies
setup-frontend:
    cd frontend && npm install

# Tidy Go modules
tidy:
    go mod tidy
