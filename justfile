default:
    @just --list

# ── Quality gate ─────────────────────────────────────────────

# Run all checks (format + lint + vet + test + build)
all: fmt vet lint test build

# ── Testing ──────────────────────────────────────────────────

# Run all Go tests
test:
    go test ./... -count=1

# Run tests with verbose output
test-verbose:
    go test ./... -count=1 -v

# Run Go coverage report
coverage:
    go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

# ── Linting & Formatting ────────────────────────────────────

# Format Go code
fmt:
    gofumpt -w .

# Run go vet
vet:
    go vet ./...

# Run golangci-lint
lint:
    golangci-lint run ./...

# Run Go vulnerability check
vuln:
    govulncheck ./...

# ── Building ─────────────────────────────────────────────────

# Build the symphony binary
build:
    CGO_ENABLED=0 go build -o bin/symphony ./cmd/symphony

# Clean build artifacts
clean:
    rm -rf bin/ coverage.out

# ── Docker ───────────────────────────────────────────────────

# Build container image
docker-build:
    docker build -t symphony -f deploy/Dockerfile .

# ── Development ──────────────────────────────────────────────

# Run symphony with a local workflow
run *args: build
    ./bin/symphony --workflow WORKFLOW.md {{args}}

# Tidy Go modules
tidy:
    go mod tidy
