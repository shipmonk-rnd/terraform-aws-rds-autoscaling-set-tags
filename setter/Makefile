# Declare phony targets - these don't represent actual files
.PHONY: all build build-linux build-mac package coverage coverage-report lint check clean vendor

# Default target that runs the complete build and packaging process
all: clean vendor build package

# Builds the Go binary for AWS Lambda
# GOOS=linux - Target Linux OS (required for AWS Lambda)
# GOARCH=amd64 - Target 64-bit architecture
# CGO_ENABLED=0 - Disable CGO for static linking
# GOFLAGS=-trimpath - Remove file system paths from binary for reproducible builds
# -ldflags='-s -w' - Strip debugging information to reduce binary size

# Build information
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# LDFLAGS for version information
VERSION_FLAGS = -X counter/internal/version.Version=$(VERSION)
VERSION_FLAGS += -X counter/internal/version.GitCommit=$(COMMIT)
VERSION_FLAGS += -X counter/internal/version.BuildTime=$(DATE)

# Common build flags
BUILD_FLAGS = -mod=readonly -ldflags='-s -w $(VERSION_FLAGS)'

# Build for AWS Lambda (Linux amd64)
build:
	@echo "Building for AWS Lambda..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS=-trimpath go build \
		$(BUILD_FLAGS) \
		-o bootstrap cmd/main.go
	@echo "Built binary info:"
	@file bootstrap

# Build for Linux AMD64 local development
build-linux:
	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS=-trimpath go build \
		$(BUILD_FLAGS) \
		-o bootstrap cmd/main.go
	@echo "Built binary info:"
	@file bootstrap

# Build for local development on M-series Macs
build-mac:
	@echo "Building for Apple Silicon..."
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 GOFLAGS=-trimpath go build \
		$(BUILD_FLAGS) \
		-o bootstrap cmd/main.go
	@echo "Built binary info:"
	@file bootstrap

# Ensure dependencies are up to date and vendored
vendor:
	@echo "Updating and vendoring dependencies..."
	@go mod tidy
	@go mod vendor

# Creates a ZIP archive of the binary for AWS Lambda deployment
# -j flag removes path information from the archive
package:
	zip -j main.zip bootstrap

# Generate test coverage data and display coverage statistics
coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./internal/...
	@go tool cover -func=coverage.out

# Generate and open HTML coverage report
coverage-report: coverage
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Opening coverage report..."
	@open coverage.html

# Lint the code using golangci-lint
lint:
	@echo "Linting code..."
	@golangci-lint run --config=.golangci.yml ./internal/...

# Run all code quality checks (linting and tests)
check: lint coverage
	@echo "All checks completed"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f bootstrap main.zip
	@rm -f coverage.out coverage.html
	@rm -f __debug_bin
	@rm -rf vendor
	@find . -type f -name '.DS_Store' -delete
