#!/bin/bash

# Cross-platform build script for devManager Desktop Application
# This script ensures Node.js is available and then delegates to build.js

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Node.js is installed
check_node() {
    if ! command -v node &> /dev/null; then
        print_error "Node.js is not installed or not in PATH"
        print_status "Please install Node.js and try again"
        print_status "Visit: https://nodejs.org/"
        exit 1
    fi
    
    local node_version=$(node --version)
    print_success "Found Node.js $node_version"
}

# Check if we're in the correct directory
check_directory() {
    if [[ ! -f "build.js" ]]; then
        print_error "build.js not found in current directory"
        print_status "Please run this script from the devmanager-app directory"
        exit 1
    fi
}

# Main execution
main() {
    print_status "devManager Desktop Application Builder"
    print_status "Checking prerequisites..."
    
    check_node
    check_directory
    
    print_status "Starting build process..."
    
    # Pass all arguments to build.js
    if [[ $# -eq 0 ]]; then
        node build.js
    else
        node build.js "$@"
    fi
    
    print_success "Build process completed"
}

# Handle script interruption
trap 'print_warning "Build interrupted by user"; exit 130' INT

# Run main function with all arguments
main "$@"