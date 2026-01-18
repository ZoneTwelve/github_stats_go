#!/bin/bash

# GitHub Readme Stats - Go Server Startup Script
# This script handles the setup and execution of the Go server

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GO_SERVER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT=${PORT:-3000}
GITHUB_TOKEN=${GITHUB_TOKEN:-""}
PAT_1=${PAT_1:-""}

# Functions
print_header() {
    echo -e "${BLUE}"
    echo "🚀 GitHub Readme Stats - Go Server"
    echo "=================================="
    echo -e "${NC}"
}

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_go_installation() {
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        echo "Please install Go 1.21 or later from: https://golang.org/dl/"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_info "Go version $GO_VERSION detected"
}

check_go_version() {
    GO_VERSION_NUM=$(go version | awk '{print $3}' | sed 's/go//' | awk -F. '{printf("%d%03d%03d", $1,$2,$3)}')
    MIN_VERSION_NUM="1021000"  # 1.21.0

    if [ "$GO_VERSION_NUM" -lt "$MIN_VERSION_NUM" ]; then
        print_error "Go version $GO_VERSION detected, but 1.21.0 or later is required"
        exit 1
    fi
}

setup_dependencies() {
    print_info "Setting up Go modules..."

    cd "$GO_SERVER_DIR"

    if [ ! -f "go.sum" ]; then
        print_info "Downloading dependencies..."
        go mod download
        go mod tidy
    else
        print_info "Dependencies already cached"
    fi
}

check_token() {
    if [ -z "$GITHUB_TOKEN" ] && [ -z "$PAT_1" ]; then
        print_warning "No GitHub token found in environment variables"
        echo "You can set it with:"
        echo "  export GITHUB_TOKEN=ghp_your_token_here"
        echo "or"
        echo "  export PAT_1=ghp_your_token_here"
        echo ""
        echo "Without a token, you'll be limited to 60 requests per hour."
        echo "Get a token from: https://github.com/settings/tokens"
        echo ""
        read -p "Continue without token? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Set your token and run again"
            exit 1
        fi
    else
        if [ -n "$GITHUB_TOKEN" ]; then
            print_info "Using GITHUB_TOKEN from environment"
        else
            print_info "Using PAT_1 from environment"
        fi
    fi
}

build_server() {
    print_info "Building Go server..."
    go build -o github-stats-server main.go
    print_info "Build completed successfully"
}

start_server() {
    print_info "Starting server on port $PORT..."
    echo ""

    # Check if binary exists and is executable
    if [ -f "github-stats-server" ] && [ -x "github-stats-server" ]; then
        print_info "Using pre-built binary"
        ./github-stats-server
    else
        print_info "Running with 'go run'"
        go run main.go
    fi
}

show_endpoints() {
    echo ""
    print_info "Server will be available at:"
    echo "  • Main API:    http://localhost:$PORT/api?username=Zonetwelve"
    echo "  • Languages:   http://localhost:$PORT/api/top-langs?username=Zonetwelve"
    echo "  • Repos:       http://localhost:$PORT/api/pin?username=Zonetwelve"
    echo "  • Health:      http://localhost:$PORT/health"
    echo "  • Info:        http://localhost:$PORT/"
    echo ""
    print_info "Press Ctrl+C to stop the server"
    echo ""
}

handle_sigint() {
    echo ""
    print_info "Shutting down server..."
    exit 0
}

# Main execution
main() {
    # Set up signal handling
    trap handle_sigint SIGINT SIGTERM

    print_header

    # Parse command line arguments
    case "${1:-}" in
        "help"|"-h"|"--help")
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  (no args)    Start the server"
            echo "  help         Show this help message"
            echo "  build        Build the server binary only"
            echo "  deps         Setup dependencies only"
            echo "  check        Check system requirements"
            echo ""
            echo "Environment variables:"
            echo "  PORT         Server port (default: 3000)"
            echo "  GITHUB_TOKEN GitHub Personal Access Token"
            echo "  PAT_1        Alternative token variable"
            echo ""
            echo "Examples:"
            echo "  $0                    # Start server with current settings"
            echo "  PORT=8080 $0          # Start on custom port"
            echo "  GITHUB_TOKEN=ghp_xxx $0  # Start with GitHub token"
            echo "  $0 build              # Build binary only"
            exit 0
            ;;
        "build")
            check_go_installation
            check_go_version
            setup_dependencies
            build_server
            print_info "Binary created: github-stats-server"
            exit 0
            ;;
        "deps")
            check_go_installation
            check_go_version
            setup_dependencies
            print_info "Dependencies ready"
            exit 0
            ;;
        "check")
            check_go_installation
            check_go_version
            setup_dependencies
            print_info "All checks passed! System ready."
            exit 0
            ;;
        "")
            # Start server
            check_go_installation
            check_go_version
            setup_dependencies
            check_token
            show_endpoints
            start_server
            ;;
        *)
            print_error "Unknown command: $1"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
