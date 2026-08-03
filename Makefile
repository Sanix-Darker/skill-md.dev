.PHONY: build run test clean docker docker-run install dev fmt lint

# Variables
BINARY_NAME=skillf
VERSION?=dev
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GOFLAGS?=-buildvcs=false
LDFLAGS=-ldflags "-X github.com/sanixdarker/skillf/internal/cli.Version=$(VERSION) -X github.com/sanixdarker/skillf/internal/cli.Commit=$(COMMIT)"

# Build
build:
	go build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/skillf

build-linux:
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/skillf

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 ./cmd/skillf
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 ./cmd/skillf

build-windows:
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe ./cmd/skillf

build-all: build-linux build-darwin build-windows

# Run
run: build
	./$(BINARY_NAME) serve

dev:
	go run $(GOFLAGS) ./cmd/skillf serve --debug

# Test
test:
	go test $(GOFLAGS) -v ./...

test-coverage:
	go test $(GOFLAGS) -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
	rm -f coverage.out coverage.html
	rm -f skillf.db

# Docker
docker:
	docker build -t skillf:$(VERSION) .

docker-run:
	docker run -p 8080:8080 -v skillf_data:/data skillf:$(VERSION)

docker-compose:
	docker compose up -d

docker-compose-down:
	docker compose down

# Install locally
install: build
	cp $(BINARY_NAME) /usr/local/bin/

# Format
fmt:
	go fmt ./...
	gofmt -s -w .

# Lint
lint:
	golangci-lint run

# Tidy
tidy:
	go mod tidy

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  build-all      - Build for all platforms"
	@echo "  run            - Build and run the server"
	@echo "  dev            - Run in development mode with debug"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  clean          - Remove built files"
	@echo "  docker         - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-compose - Run with docker compose"
	@echo "  install        - Install binary to /usr/local/bin"
	@echo "  fmt            - Format code"
	@echo "  lint           - Run linter"
	@echo "  tidy           - Run go mod tidy"
