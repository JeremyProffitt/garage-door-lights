#!/bin/bash
#
# Particle Firmware Flash Script
# Compiles and flashes firmware to a Particle device
#
# Usage: ./flash.sh [device-name] [platform]
#   device-name: Name of the Particle device (default: garage-door-lights)
#   platform:    Device platform: argon, boron, photon, p2, electron (default: auto-detect)
#
# Examples:
#   ./flash.sh                          # Flash to garage-door-lights (auto-detect platform)
#   ./flash.sh my-device                # Flash to my-device (auto-detect platform)
#   ./flash.sh my-device argon          # Flash to my-device as Argon
#   ./flash.sh "" photon                # Flash to default device as Photon
#

set -e

# Configuration
DEFAULT_DEVICE="garage-door-lights"
FIRMWARE_FILE="candle-lights.ino"
OUTPUT_DIR="bin"
SUPPORTED_PLATFORMS=("argon" "boron" "photon" "p2" "electron" "photon2")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_info() {
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

# Check if Particle CLI is installed
check_particle_cli() {
    if ! command -v particle &> /dev/null; then
        print_error "Particle CLI is not installed."
        echo "Install it with: npm install -g particle-cli"
        echo "Or visit: https://docs.particle.io/getting-started/developer-tools/cli/"
        exit 1
    fi
}

# Check if logged in to Particle
check_login() {
    if ! particle whoami &> /dev/null; then
        print_warning "Not logged in to Particle Cloud."
        echo "Please run: particle login"
        exit 1
    fi
    print_info "Logged in as: $(particle whoami)"
}

# Detect device platform
detect_platform() {
    local device=$1
    print_info "Detecting platform for device: $device"

    # Get device info from Particle Cloud
    local device_info=$(particle list 2>/dev/null | grep -i "^$device" || true)

    if [ -z "$device_info" ]; then
        print_warning "Could not find device '$device' in your account."
        print_info "Available devices:"
        particle list 2>/dev/null || true
        return 1
    fi

    # Extract platform from device info (format: "name [device_id] (platform) is online/offline")
    local platform=$(echo "$device_info" | grep -oE '\((argon|boron|photon|p2|electron|photon2)\)' | tr -d '()' || true)

    if [ -n "$platform" ]; then
        echo "$platform"
        return 0
    fi

    return 1
}

# Validate platform
validate_platform() {
    local platform=$1
    for p in "${SUPPORTED_PLATFORMS[@]}"; do
        if [ "$p" = "$platform" ]; then
            return 0
        fi
    done
    return 1
}

# Compile firmware
compile_firmware() {
    local platform=$1
    local output_file=$2

    print_info "Compiling firmware for $platform..."

    # Create output directory if needed
    mkdir -p "$OUTPUT_DIR"

    # Compile
    if particle compile "$platform" "$FIRMWARE_FILE" --saveTo "$output_file"; then
        print_success "Compiled successfully: $output_file"
        return 0
    else
        print_error "Compilation failed!"
        return 1
    fi
}

# Flash firmware OTA
flash_ota() {
    local device=$1
    local firmware=$2

    print_info "Flashing firmware to $device via OTA..."

    if particle flash "$device" "$firmware"; then
        print_success "Firmware flashed successfully!"
        return 0
    else
        print_error "Flash failed!"
        return 1
    fi
}

# Flash firmware via USB
flash_usb() {
    local firmware=$1

    print_warning "USB flashing requires device in DFU mode."
    echo "Put your device in DFU mode:"
    echo "  1. Hold both RESET and MODE buttons"
    echo "  2. Release RESET while holding MODE"
    echo "  3. Wait for LED to blink yellow"
    echo "  4. Release MODE"
    echo ""
    read -p "Press Enter when device is in DFU mode..."

    print_info "Flashing firmware via USB..."

    if particle flash --usb "$firmware"; then
        print_success "Firmware flashed successfully!"
        return 0
    else
        print_error "USB flash failed!"
        return 1
    fi
}

# Main function
main() {
    local device="${1:-$DEFAULT_DEVICE}"
    local platform="${2:-}"
    local flash_mode="${3:-ota}"

    echo "========================================"
    echo "  Particle Firmware Flash Script"
    echo "========================================"
    echo ""

    # Check prerequisites
    check_particle_cli
    check_login

    # Handle empty device name (when platform is specified as second arg)
    if [ -z "$device" ]; then
        device="$DEFAULT_DEVICE"
    fi

    print_info "Target device: $device"

    # Detect or validate platform
    if [ -z "$platform" ]; then
        platform=$(detect_platform "$device") || {
            print_error "Could not auto-detect platform."
            echo ""
            echo "Please specify platform manually:"
            echo "  ./flash.sh $device <platform>"
            echo ""
            echo "Supported platforms: ${SUPPORTED_PLATFORMS[*]}"
            exit 1
        }
        print_info "Auto-detected platform: $platform"
    else
        if ! validate_platform "$platform"; then
            print_error "Invalid platform: $platform"
            echo "Supported platforms: ${SUPPORTED_PLATFORMS[*]}"
            exit 1
        fi
        print_info "Using specified platform: $platform"
    fi

    # Check if firmware file exists
    if [ ! -f "$FIRMWARE_FILE" ]; then
        print_error "Firmware file not found: $FIRMWARE_FILE"
        exit 1
    fi

    # Compile firmware
    local output_file="$OUTPUT_DIR/${platform}_firmware.bin"
    compile_firmware "$platform" "$output_file" || exit 1

    # Flash firmware
    echo ""
    print_info "Flash mode: $flash_mode"

    if [ "$flash_mode" = "usb" ]; then
        flash_usb "$output_file"
    else
        flash_ota "$device" "$output_file"
    fi

    echo ""
    echo "========================================"
    print_success "Done!"
    echo "========================================"
}

# Show help
show_help() {
    echo "Particle Firmware Flash Script"
    echo ""
    echo "Usage: $0 [options] [device-name] [platform]"
    echo ""
    echo "Arguments:"
    echo "  device-name    Name of the Particle device (default: garage-door-lights)"
    echo "  platform       Device platform: argon, boron, photon, p2, electron"
    echo "                 If not specified, auto-detects from Particle Cloud"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -u, --usb      Flash via USB (DFU mode) instead of OTA"
    echo "  -l, --list     List available devices"
    echo "  -c, --compile  Compile only (don't flash)"
    echo ""
    echo "Examples:"
    echo "  $0                           # Flash to garage-door-lights (auto-detect)"
    echo "  $0 my-device                 # Flash to my-device (auto-detect platform)"
    echo "  $0 my-device argon           # Flash to my-device as Argon"
    echo "  $0 -u my-device              # Flash via USB"
    echo "  $0 -c \"\" photon              # Compile for Photon only"
}

# Parse command line options
FLASH_MODE="ota"
COMPILE_ONLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -u|--usb)
            FLASH_MODE="usb"
            shift
            ;;
        -l|--list)
            check_particle_cli
            check_login
            print_info "Available devices:"
            particle list
            exit 0
            ;;
        -c|--compile)
            COMPILE_ONLY=true
            shift
            ;;
        *)
            break
            ;;
    esac
done

# Run main with remaining arguments
if [ "$COMPILE_ONLY" = true ]; then
    check_particle_cli
    platform="${2:-argon}"
    if ! validate_platform "$platform"; then
        print_error "Invalid platform: $platform"
        echo "Supported platforms: ${SUPPORTED_PLATFORMS[*]}"
        exit 1
    fi
    mkdir -p "$OUTPUT_DIR"
    compile_firmware "$platform" "$OUTPUT_DIR/${platform}_firmware.bin"
else
    main "$1" "$2" "$FLASH_MODE"
fi
