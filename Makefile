.PHONY: build-windows build-linux-cli build-macos-cli test clean help

help:
	@echo "UNBOUND Build System"
	@echo "===================="
	@echo ""
	@echo "Available targets:"
	@echo "  build-windows      - Build Windows GUI application with Wails"
	@echo "  build-linux-cli    - Build Linux headless CLI binary (no GUI)"
	@echo "  build-macos-cli    - Build macOS headless CLI binary (no GUI)"
	@echo "  test               - Run all tests including CLI tests"
	@echo "  test-cli           - Run CLI-specific tests"
	@echo "  clean              - Remove build artifacts"
	@echo ""

build-windows:
	@echo "Building Windows GUI application..."
	wails build -clean
	@echo "Windows build complete: build/bin/unbound.exe"

build-linux-cli:
	@echo "Building Linux CLI binary (headless mode)..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/unbound-cli-linux .
	@echo "Linux CLI build complete: build/unbound-cli-linux"
	@echo "Size: $$(du -h build/unbound-cli-linux | cut -f1)"

build-macos-cli:
	@echo "Building macOS CLI binary (headless mode)..."
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o build/unbound-cli-macos .
	@echo "macOS CLI build complete: build/unbound-cli-macos"
	@echo "Size: $$(du -h build/unbound-cli-macos | cut -f1)"

build-all-cli: build-linux-cli build-macos-cli
	@echo "All CLI builds complete"

test:
	@echo "Running all tests..."
	go test -v ./...

test-cli:
	@echo "Running CLI-specific tests..."
	go test -v -run TestCLI ./...

test-engine:
	@echo "Running engine tests..."
	go test -v ./engine/...

clean:
	@echo "Cleaning build artifacts..."
	rm -rf build/bin
	rm -f build/unbound-cli-linux
	rm -f build/unbound-cli-macos
	rm -f unbound-test.exe
	rm -f unbound-sprint5.exe
	rm -f unbound-sprint3.exe
	@echo "Clean complete"

dev:
	@echo "Starting development mode..."
	wails dev

install-deps:
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

verify-build:
	@echo "Verifying build environment..."
	@which go || (echo "Go not found" && exit 1)
	@which wails || (echo "Wails not found - install with: go install github.com/wailsapp/wails/v2/cmd/wails@latest" && exit 1)
	@echo "Build environment OK"
