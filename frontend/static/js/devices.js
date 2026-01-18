function devicesPage() {
    return {
        devices: [],
        patterns: [],
        showStripModal: false,
        stripConfigDevice: null,
        stripConfig: [],
        deviceVariables: null,
        isSavingStrips: false,
        isLoadingVariables: false,
        isLoading: true,
        isRefreshing: false,
        refreshMessage: '',
        showHidden: false,

        init() {
            this.loadDevices();
            this.loadPatterns();
        },

        get filteredDevices() {
            return this.devices.filter(d => this.showHidden || !d.isHidden);
        },

        get readyDevices() {
            return this.filteredDevices
                .filter(d => d.isOnline && d.isReady)
                .sort((a, b) => a.name.localeCompare(b.name));
        },

        get onlineDevices() {
            return this.filteredDevices
                .filter(d => d.isOnline && !d.isReady)
                .sort((a, b) => a.name.localeCompare(b.name));
        },

        get offlineDevices() {
            return this.filteredDevices
                .filter(d => !d.isOnline)
                .sort((a, b) => a.name.localeCompare(b.name));
        },

        get deviceCount() {
            return this.devices.filter(d => !d.isHidden).length;
        },

        get hiddenDeviceCount() {
            return this.devices.filter(d => d.isHidden).length;
        },

        async loadDevices() {
            this.isLoading = true;
            const resp = await fetch('/api/devices', {
                credentials: 'same-origin'
            });
            const data = await resp.json();
            if (data.success) {
                this.devices = data.data || [];
            }
            this.isLoading = false;
        },

        async loadPatterns() {
            const resp = await fetch('/api/patterns', {
                credentials: 'same-origin'
            });
            const data = await resp.json();
            if (data.success) {
                // Filter out any test patterns (handled separately in UI)
                this.patterns = (data.data || []).filter(p =>
                    !p.name.toLowerCase().includes('rainbow bytecode test')
                );
            }
        },

        async refreshFromParticle() {
            this.isRefreshing = true;
            this.refreshMessage = '';
            try {
                const resp = await fetch('/api/particle/devices/refresh', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    credentials: 'same-origin'
                });

                const data = await resp.json();

                if (data.success) {
                    const count = data.count || 0;
                    this.refreshMessage = `Successfully refreshed! Found ${count} device(s) from Particle.io`;
                    await this.loadDevices();
                    // Clear message after 5 seconds
                    setTimeout(() => {
                        this.refreshMessage = '';
                    }, 5000);
                } else {
                    this.refreshMessage = 'Error refreshing devices: ' + (data.error || 'Unknown error');
                }
            } catch (err) {
                this.refreshMessage = 'Error refreshing devices: ' + err.message;
            } finally {
                this.isRefreshing = false;
            }
        },

        async toggleHidden(deviceId, currentHidden) {
            const resp = await fetch(`/api/devices/${deviceId}`, {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({isHidden: !currentHidden})
            });

            const data = await resp.json();

            if (data.success) {
                this.loadDevices();
            } else {
                NotificationBanner.error('Error: ' + data.error);
            }
        },

        async checkDeviceReadiness(device) {
            // Fetch device variables to check current firmware status
            try {
                const vars = await this.fetchDeviceVariables(device.deviceId);
                if (vars && vars.firmwareVersion) {
                    NotificationBanner.info(`Device ${device.name}: Firmware ${vars.firmwareVersion}, Platform ${vars.platform || 'Unknown'}, Strips: ${vars.numStrips || 0}. Click "Refresh from Particle.io" to update.`);
                } else {
                    NotificationBanner.warning('Could not read firmware info from device. The device may be offline or not running the LED controller firmware.');
                }
            } catch (err) {
                NotificationBanner.error('Error checking device: ' + err.message);
            }
        },

        getPatternName(patternId) {
            const pattern = this.patterns.find(p => p.patternId === patternId);
            return pattern ? pattern.name : 'Unknown';
        },

        getStripPatternId(device, pin) {
            const strip = device.ledStrips?.find(s => s.pin === pin);
            return strip?.patternId || '';
        },

        setStripPattern(device, stripIndex, patternId) {
            // Update the pattern ID in the local device state
            if (device.ledStrips && device.ledStrips[stripIndex]) {
                device.ledStrips[stripIndex].patternId = patternId;
            }
        },

        // Rainbow Bytecode Test - known-working test pattern
        RAINBOW_BYTECODE_TEST: 'TENMAgAI8gBQClEDBFEEzGc=',

        async applyStripPattern(device, pin) {
            const strip = device.ledStrips?.find(s => s.pin === pin);
            if (!strip || !strip.patternId) {
                NotificationBanner.warning('Please select a pattern first');
                return;
            }

            // Handle special bytecode test patterns
            if (strip.patternId === '__rainbow_bytecode_test__') {
                try {
                    await this.sendBytecodeToStrip(device.deviceId, pin, this.RAINBOW_BYTECODE_TEST);
                    NotificationBanner.success(`Rainbow Bytecode Test applied to strip D${pin}`);
                } catch (err) {
                    NotificationBanner.error('Error applying bytecode: ' + err.message);
                }
                return;
            }

            const pattern = this.patterns.find(p => p.patternId === strip.patternId);
            if (!pattern) {
                NotificationBanner.error('Pattern not found');
                return;
            }

            try {
                // Check if this is a Glow Blaster pattern with WLED binary
                if (pattern.category === 'glowblaster' && (pattern.wledBinary || pattern.bytecode)) {
                    await this.sendBytecodeToStrip(device.deviceId, pin, pattern.wledBinary || pattern.bytecode);
                    NotificationBanner.success(`Pattern "${pattern.name}" applied to strip D${pin}`);
                    return;
                }

                // First save the strip pattern assignment to the database
                const saveResp = await fetch(`/api/devices/${device.deviceId}`, {
                    method: 'PUT',
                    headers: {'Content-Type': 'application/json'},
                    credentials: 'same-origin',
                    body: JSON.stringify({ ledStrips: device.ledStrips })
                });

                const saveData = await saveResp.json();
                if (!saveData.success) {
                    NotificationBanner.error('Error saving strip pattern: ' + saveData.error);
                    return;
                }

                // Then send the pattern to the device for this specific strip
                await this.sendPatternToStrip(device.deviceId, pin, pattern);
                NotificationBanner.success(`Pattern "${pattern.name}" applied to strip D${pin}`);
            } catch (err) {
                NotificationBanner.error('Error applying pattern: ' + err.message);
            }
        },

        async sendBytecodeToStrip(deviceId, pin, bytecode) {
            // Bytecode is already base64-encoded from Go backend or hardcoded
            const resp = await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({
                    deviceId,
                    command: 'setBytecode',
                    argument: `${pin},${bytecode}`
                })
            });
            const data = await resp.json();
            if (!data.success) {
                throw new Error(data.error || 'Failed to send bytecode');
            }
        },

        async sendPatternToStrip(deviceId, pin, pattern) {
            // Check if pattern already has WLED state or binary
            if (pattern.wledState || pattern.wledBinary || pattern.bytecode) {
                let bytecode = pattern.wledBinary || pattern.bytecode;

                // If we have WLED JSON but no binary, compile it
                if (!bytecode && pattern.wledState) {
                    const compileResp = await fetch('/api/glowblaster/compile', {
                        method: 'POST',
                        headers: {'Content-Type': 'application/json'},
                        credentials: 'same-origin',
                        body: JSON.stringify({ lcl: pattern.wledState })
                    });
                    const compileData = await compileResp.json();
                    if (compileData.success && compileData.data?.bytecode) {
                        bytecode = compileData.data.bytecode;
                    } else {
                        throw new Error('Failed to compile pattern: ' + (compileData.data?.errors?.join(', ') || 'Unknown error'));
                    }
                }

                if (bytecode) {
                    await this.sendBytecodeToStrip(deviceId, pin, bytecode);
                    return;
                }
            }

            // Build WLED JSON from pattern fields
            const effectMap = {
                'solid': 0,
                'pulse': 2,
                'wave': 67,
                'rainbow': 9,
                'fire': 66,
                'candle': 71
            };

            // Helper to clamp values to 0-255 range
            const clamp = (val) => Math.max(0, Math.min(255, val || 0));

            // Read effectId from metadata first, then fall back to type mapping
            const effectId = pattern.metadata?.effectId
                ? parseInt(pattern.metadata.effectId)
                : (effectMap[pattern.type] || 71);

            // Read speed/intensity/custom1 from metadata (stored as 0-255) or fall back to defaults
            const speed = pattern.metadata?.speed
                ? parseInt(pattern.metadata.speed)
                : 128;
            const intensity = pattern.metadata?.intensity
                ? parseInt(pattern.metadata.intensity)
                : 128;
            const custom1 = pattern.metadata?.custom1
                ? parseInt(pattern.metadata.custom1)
                : 128;

            // Build colors array with clamped RGB values
            const colors = pattern.colors?.map(c => [
                clamp(c.r), clamp(c.g), clamp(c.b)
            ]) || [[
                clamp(pattern.red || 255),
                clamp(pattern.green || 100),
                clamp(pattern.blue || 0)
            ]];

            const wledJson = JSON.stringify({
                on: true,
                bri: clamp(pattern.brightness || 200),
                seg: [{
                    id: 0,
                    start: 0,
                    stop: 8,
                    fx: effectId,
                    sx: clamp(speed),
                    ix: clamp(intensity),
                    c1: clamp(custom1),
                    col: colors,
                    on: true
                }]
            });

            // Compile WLED JSON to binary
            const compileResp = await fetch('/api/glowblaster/compile', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({ lcl: wledJson })
            });
            const compileData = await compileResp.json();

            if (!compileData.success || !compileData.data?.bytecode) {
                throw new Error('Failed to compile pattern: ' + (compileData.data?.errors?.join(', ') || 'Unknown error'));
            }

            // Send bytecode to device
            await this.sendBytecodeToStrip(deviceId, pin, compileData.data.bytecode);
        },

        async openStripConfig(device) {
            this.stripConfigDevice = device;
            this.deviceVariables = null;
            this.isLoadingVariables = true;
            this.showStripModal = true;

            // Load strip configuration from database first
            this.stripConfig = device.ledStrips
                ? JSON.parse(JSON.stringify(device.ledStrips))
                : [];

            // If device is ready, fetch current config from device
            if (device.isReady) {
                try {
                    const vars = await this.fetchDeviceVariables(device.deviceId);
                    this.deviceVariables = vars;
                } catch (err) {
                    console.error('Failed to fetch device variables:', err);
                }
            }
            this.isLoadingVariables = false;
        },

        async fetchDeviceVariables(deviceId) {
            const resp = await fetch(`/api/particle/devices/${deviceId}/variables`, {
                credentials: 'same-origin'
            });
            const data = await resp.json();
            if (data.success) {
                return data.data;
            }
            throw new Error(data.error || 'Failed to fetch device variables');
        },

        async syncFromDevice() {
            if (!this.stripConfigDevice || !this.deviceVariables) return;

            // Use the strips from device variables
            if (this.deviceVariables.strips && this.deviceVariables.strips.length > 0) {
                this.stripConfig = this.deviceVariables.strips.map(s => ({
                    pin: s.pin,
                    ledCount: s.ledCount
                }));
            } else {
                this.stripConfig = [];
            }
        },

        addStrip() {
            const maxStrips = this.deviceVariables?.maxStrips || 4;
            if (this.stripConfig.length < maxStrips) {
                // Find first unused pin
                const usedPins = this.stripConfig.map(s => s.pin);
                let nextPin = 0;
                for (let i = 0; i <= 7; i++) {
                    if (!usedPins.includes(i)) {
                        nextPin = i;
                        break;
                    }
                }
                const defaultLeds = this.deviceVariables?.maxLedsPerStrip ? Math.min(8, this.deviceVariables.maxLedsPerStrip) : 8;
                this.stripConfig.push({ pin: nextPin, ledCount: defaultLeds });
            }
        },

        getMaxStrips() {
            return this.deviceVariables?.maxStrips || 4;
        },

        getMaxLedsPerStrip() {
            return this.deviceVariables?.maxLedsPerStrip || 60;
        },

        removeStrip(index) {
            this.stripConfig.splice(index, 1);
        },

        async saveStripConfig() {
            this.isSavingStrips = true;

            try {
                // Validate for duplicate pins
                const pins = this.stripConfig.map(s => s.pin);
                const uniquePins = new Set(pins);
                if (pins.length !== uniquePins.size) {
                    NotificationBanner.error('Each strip must use a different pin.');
                    this.isSavingStrips = false;
                    return;
                }

                const resp = await fetch(`/api/devices/${this.stripConfigDevice.deviceId}`, {
                    method: 'PUT',
                    headers: {'Content-Type': 'application/json'},
                    credentials: 'same-origin',
                    body: JSON.stringify({ ledStrips: this.stripConfig })
                });

                const data = await resp.json();

                if (data.success) {
                    // Send strip configuration to device via Particle
                    await this.syncStripsToDevice(this.stripConfigDevice.deviceId, this.stripConfig);
                    this.showStripModal = false;
                    this.loadDevices();
                    NotificationBanner.success('LED strip configuration saved and applied to device');
                } else {
                    NotificationBanner.error('Error: ' + data.error);
                }
            } catch (err) {
                NotificationBanner.error('Error saving configuration: ' + err.message);
            } finally {
                this.isSavingStrips = false;
            }
        },

        async syncStripsToDevice(deviceId, strips) {
            // First clear all existing strips on the device
            await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({
                    deviceId,
                    command: 'clearAll',
                    argument: '1'
                })
            });

            // Add each configured strip
            for (const strip of strips) {
                await fetch('/api/particle/command', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    credentials: 'same-origin',
                    body: JSON.stringify({
                        deviceId,
                        command: 'addStrip',
                        argument: `${strip.pin},${strip.ledCount}`
                    })
                });
            }

            // Save config to device EEPROM
            await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({
                    deviceId,
                    command: 'saveConfig',
                    argument: '1'
                })
            });
        }
    }
}
