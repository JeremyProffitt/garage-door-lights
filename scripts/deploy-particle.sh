#!/bin/bash

# Script to deploy Particle firmware
# This script uses the Particle CLI to compile and flash firmware to devices

set -e

echo "======================================"
echo "Particle Firmware Deployment"
echo "======================================"

# Check required environment variables
if [ -z "$PARTICLE_ACCESS_TOKEN" ]; then
    echo "Error: PARTICLE_ACCESS_TOKEN not set"
    exit 1
fi

if [ -z "$PARTICLE_DEVICE_ID" ]; then
    echo "Warning: PARTICLE_DEVICE_ID not set - will compile only without flashing"
    COMPILE_ONLY=true
else
    COMPILE_ONLY=false
fi

# Install Particle CLI if not present
if ! command -v particle &> /dev/null; then
    echo "Installing Particle CLI..."
    npm install -g particle-cli
fi

# Login to Particle Cloud
echo "Logging in to Particle Cloud..."
particle login --token "$PARTICLE_ACCESS_TOKEN"

# Compile firmware
echo "Compiling firmware for Argon..."
cd firmware

# Compile the firmware (cloud compile)
particle compile argon . --saveTo compiled-firmware.bin

if [ ! -f compiled-firmware.bin ]; then
    echo "Error: Firmware compilation failed"
    exit 1
fi

echo "Firmware compiled successfully!"
FILE_SIZE=$(stat -f%z compiled-firmware.bin 2>/dev/null || stat -c%s compiled-firmware.bin 2>/dev/null)
echo "Compiled firmware size: $FILE_SIZE bytes"

# Flash firmware if device ID is provided
if [ "$COMPILE_ONLY" = false ]; then
    echo "Flashing firmware to device: $PARTICLE_DEVICE_ID"

    # Check if device is online
    DEVICE_STATUS=$(particle list | grep "$PARTICLE_DEVICE_ID" || echo "OFFLINE")
    echo "Device status: $DEVICE_STATUS"

    # Flash the firmware
    particle flash "$PARTICLE_DEVICE_ID" compiled-firmware.bin

    echo "Firmware flashed successfully!"
    echo "Device will reboot and apply new firmware..."
else
    echo "Skipping firmware flash - no device ID provided"
    echo "Firmware binary saved to: firmware/compiled-firmware.bin"
fi

cd ..

echo ""
echo "======================================"
echo "Particle Firmware Deployment Complete!"
echo "======================================"
echo ""

if [ "$COMPILE_ONLY" = false ]; then
    echo "Next steps:"
    echo "1. Monitor device logs: particle serial monitor"
    echo "2. Check device status: particle list"
    echo "3. View device events: particle subscribe mine"
else
    echo "Compiled firmware is ready to flash manually:"
    echo "  particle flash $PARTICLE_DEVICE_ID firmware/compiled-firmware.bin"
fi
echo ""
