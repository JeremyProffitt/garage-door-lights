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
                this.patterns = data.data || [];
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

        async applyStripPattern(device, pin) {
            const strip = device.ledStrips?.find(s => s.pin === pin);
            if (!strip || !strip.patternId) {
                NotificationBanner.warning('Please select a pattern first');
                return;
            }

            const pattern = this.patterns.find(p => p.patternId === strip.patternId);
            if (!pattern) {
                NotificationBanner.error('Pattern not found');
                return;
            }

            try {
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

        async sendPatternToStrip(deviceId, pin, pattern) {
            // Map pattern type to firmware number
            const patternMap = {
                'candle': 1,
                'solid': 2,
                'pulse': 3,
                'wave': 4,
                'rainbow': 5,
                'fire': 6
            };
            const patternNum = patternMap[pattern.type] || 2;

            // Send pattern command: "pin,pattern,speed"
            await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({
                    deviceId,
                    command: 'setPattern',
                    argument: `${pin},${patternNum},${pattern.speed || 50}`
                })
            });

            // Send color command: "pin,R,G,B"
            const color = pattern.colors?.[0] || { r: pattern.red || 255, g: pattern.green || 100, b: pattern.blue || 0 };
            await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({
                    deviceId,
                    command: 'setColor',
                    argument: `${pin},${color.r},${color.g},${color.b}`
                })
            });

            // Send brightness command: "pin,brightness"
            await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({
                    deviceId,
                    command: 'setBright',
                    argument: `${pin},${pattern.brightness || 128}`
                })
            });

            // Save to device EEPROM
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
