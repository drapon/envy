#!/bin/bash
# envy release signing script
# This script signs released binaries

set -euo pipefail

# Color output definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Log functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Show usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
    -p, --platform PLATFORM    Platform to sign (macos, windows, all)
    -v, --version VERSION      Version number
    -d, --dist-dir DIR         Path to dist directory (default: ./dist)
    -h, --help                 Show this help

Examples:
    $0 --platform macos --version v1.0.0
    $0 --platform all --version v1.0.0 --dist-dir /path/to/dist
EOF
}

# Default values
PLATFORM="all"
VERSION=""
DIST_DIR="./dist"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -p|--platform)
            PLATFORM="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -d|--dist-dir)
            DIST_DIR="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Version check
if [ -z "$VERSION" ]; then
    log_error "Version not specified"
    usage
    exit 1
fi

# Dist directory check
if [ ! -d "$DIST_DIR" ]; then
    log_error "Dist directory not found: $DIST_DIR"
    exit 1
fi

# macOS signing function
sign_macos() {
    log_info "Starting macOS binary signing..."
    
    # Environment variable check
    if [ -z "${DEVELOPER_ID:-}" ]; then
        log_error "DEVELOPER_ID environment variable not set"
        return 1
    fi
    
    # Find macOS binaries
    for arch in amd64 arm64; do
        BINARY_PATH="$DIST_DIR/envy_Darwin_${arch}/envy"
        
        if [ -f "$BINARY_PATH" ]; then
            log_info "Signing: $BINARY_PATH"
            
            # Code signing
            if codesign --force --options runtime \
                --sign "$DEVELOPER_ID" \
                --timestamp \
                --verbose \
                "$BINARY_PATH"; then
                log_info "Signing successful: $BINARY_PATH"
                
                # Verify signature
                if codesign --verify --verbose "$BINARY_PATH"; then
                    log_info "Signature verification successful: $BINARY_PATH"
                else
                    log_error "Signature verification failed: $BINARY_PATH"
                    return 1
                fi
            else
                log_error "Signing failed: $BINARY_PATH"
                return 1
            fi
            
            # Create ZIP for notarization
            ZIP_PATH="$DIST_DIR/envy_Darwin_${arch}_signed.zip"
            log_info "Creating ZIP for notarization: $ZIP_PATH"
            
            cd "$(dirname "$BINARY_PATH")"
            zip -r "$ZIP_PATH" "$(basename "$BINARY_PATH")"
            cd - > /dev/null
            
            # Notarization (only if Apple ID is set)
            if [ -n "${APPLE_ID:-}" ] && [ -n "${APPLE_PASSWORD:-}" ] && [ -n "${TEAM_ID:-}" ]; then
                log_info "Running notarization..."
                
                xcrun notarytool submit "$ZIP_PATH" \
                    --apple-id "$APPLE_ID" \
                    --password "$APPLE_PASSWORD" \
                    --team-id "$TEAM_ID" \
                    --wait
                
                if [ $? -eq 0 ]; then
                    log_info "Notarization successful: $ZIP_PATH"
                    
                    # Staple the notarization
                    log_info "Stapling notarization to binary..."
                    xcrun stapler staple "$BINARY_PATH"
                else
                    log_warn "Notarization failed: $ZIP_PATH (signature is still valid)"
                fi
            else
                log_warn "Apple credentials not set, skipping notarization"
            fi
        else
            log_warn "Binary not found: $BINARY_PATH"
        fi
    done
}

# Windows signing function
sign_windows() {
    log_info "Starting Windows binary signing..."
    
    # Check for signing tools
    if ! command -v signtool &> /dev/null && ! command -v osslsigncode &> /dev/null; then
        log_warn "signtool or osslsigncode not found. Skipping Windows signing."
        return 0
    fi
    
    # Check certificates
    if [ -z "${WINDOWS_CERT_FILE:-}" ] || [ -z "${WINDOWS_CERT_PASSWORD:-}" ]; then
        log_warn "Windows certificate information not set. Skipping signing."
        return 0
    fi
    
    # Find Windows binaries
    for arch in amd64 386 arm64; do
        BINARY_PATH="$DIST_DIR/envy_Windows_${arch}/envy.exe"
        
        if [ -f "$BINARY_PATH" ]; then
            log_info "Signing: $BINARY_PATH"
            
            if command -v signtool &> /dev/null; then
                # Use signtool on Windows
                signtool sign /f "$WINDOWS_CERT_FILE" \
                    /p "$WINDOWS_CERT_PASSWORD" \
                    /tr http://timestamp.digicert.com \
                    /td sha256 \
                    /fd sha256 \
                    "$BINARY_PATH"
            else
                # Use osslsigncode on Linux/macOS
                osslsigncode sign \
                    -pkcs12 "$WINDOWS_CERT_FILE" \
                    -pass "$WINDOWS_CERT_PASSWORD" \
                    -t http://timestamp.digicert.com \
                    -h sha256 \
                    -in "$BINARY_PATH" \
                    -out "${BINARY_PATH}.signed"
                
                mv "${BINARY_PATH}.signed" "$BINARY_PATH"
            fi
            
            if [ $? -eq 0 ]; then
                log_info "Signing successful: $BINARY_PATH"
            else
                log_error "Signing failed: $BINARY_PATH"
                return 1
            fi
        else
            log_warn "Binary not found: $BINARY_PATH"
        fi
    done
}

# Regenerate checksums
regenerate_checksums() {
    log_info "Regenerating checksums..."
    
    CHECKSUM_FILE="$DIST_DIR/checksums.txt"
    
    # Backup existing checksum file
    if [ -f "$CHECKSUM_FILE" ]; then
        mv "$CHECKSUM_FILE" "${CHECKSUM_FILE}.bak"
    fi
    
    # Generate new checksums
    cd "$DIST_DIR"
    find . -type f -name "envy*" -not -name "*.sig" -not -name "*.pem" | while read -r file; do
        sha256sum "$file" >> checksums.txt
    done
    cd - > /dev/null
    
    log_info "Checksum regeneration complete: $CHECKSUM_FILE"
}

# Main process
main() {
    log_info "Starting signing for envy $VERSION"
    log_info "Platform: $PLATFORM"
    log_info "Dist directory: $DIST_DIR"
    
    case $PLATFORM in
        macos)
            sign_macos
            ;;
        windows)
            sign_windows
            ;;
        all)
            # Only run macOS signing on macOS
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sign_macos
            else
                log_warn "Skipping macOS signing on non-macOS platform"
            fi
            
            sign_windows
            ;;
        *)
            log_error "Unknown platform: $PLATFORM"
            exit 1
            ;;
    esac
    
    # Regenerate checksums
    regenerate_checksums
    
    log_info "Signing process complete"
}

# Run main process
main