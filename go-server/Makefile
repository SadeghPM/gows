# ==============================================================================
# GoWS Makefile
# ==============================================================================

# Executable name
APP_NAME=gows

# Default values
PORT ?= 8080

# Build the main application
.PHONY: build
build:
	@echo "Building $(APP_NAME)..."
	go build -o $(APP_NAME) main.go hub.go
	@echo "Build complete."

# Run the project locally
.PHONY: run
run: build
	@echo "Running $(APP_NAME) on port $(PORT)..."
	./$(APP_NAME)

# Build for Linux (amd64)
.PHONY: build-linux
build-linux:
	@echo "Building $(APP_NAME) for Linux architecture..."
	GOOS=linux GOARCH=amd64 go build -o $(APP_NAME)-linux-amd64 main.go hub.go
	@echo "Linux build complete."

# Clean compiled binaries
.PHONY: clean
clean:
	@echo "Cleaning up binaries..."
	rm -f $(APP_NAME)
	@echo "Clean complete."

# Install the dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed."

# Run the installer script for Ubuntu/systemd
.PHONY: install-ubuntu
install-ubuntu:
	@echo "Running systemd installation script (requires sudo)..."
	sudo ./install.sh
