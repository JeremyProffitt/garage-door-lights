function devicesPage() {
    return {
        devices: [],
        patterns: [],
        showPatternModal: false,
        showStripModal: false,
        selectedDevice: null,
        stripConfigDevice: null,
        stripConfig: [],
        isSavingStrips: false,
        simulatingPatternId: null,
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
                alert('Error: ' + data.error);
            }
        },

        selectDeviceForPattern(device) {
            this.selectedDevice = device;
            this.simulatingPatternId = null;
            this.showPatternModal = true;
        },

        simulatePatternInAssign(pattern) {
            this.simulatingPatternId = pattern.patternId;
            setTimeout(() => {
                const container = document.getElementById('assignPatternLedsContainer');
                if (container) {
                    const simPattern = {
                        type: pattern.type,
                        red: pattern.red || 0,
                        green: pattern.green || 0,
                        blue: pattern.blue || 0,
                        brightness: pattern.brightness || 128,
                        speed: pattern.speed || 50
                    };
                    LEDSimulator.render(container, simPattern, 8);
                }
            }, 100);
        },

        async assignPattern(patternId) {
            console.log('Assigning pattern:', patternId, 'to device:', this.selectedDevice.deviceId);

            const resp = await fetch(`/api/devices/${this.selectedDevice.deviceId}/pattern`, {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({patternId})
            });

            const data = await resp.json();
            console.log('Assign pattern response:', data);

            if (data.success) {
                // Send pattern to device
                await this.sendPatternToDevice(this.selectedDevice.deviceId, patternId);
                this.showPatternModal = false;
                this.simulatingPatternId = null;
                this.loadDevices();
            } else {
                alert('Error: ' + data.error);
            }
        },

        async sendPatternToDevice(deviceId, patternId) {
            await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({deviceId, patternId})
            });
        },

        getPatternName(patternId) {
            const pattern = this.patterns.find(p => p.patternId === patternId);
            return pattern ? pattern.name : 'Unknown';
        },

        openStripConfig(device) {
            this.stripConfigDevice = device;
            // Deep clone the existing strips or start with empty array
            this.stripConfig = device.ledStrips
                ? JSON.parse(JSON.stringify(device.ledStrips))
                : [];
            this.showStripModal = true;
        },

        addStrip() {
            if (this.stripConfig.length < 4) {
                // Find first unused pin
                const usedPins = this.stripConfig.map(s => s.pin);
                let nextPin = 0;
                for (let i = 0; i <= 7; i++) {
                    if (!usedPins.includes(i)) {
                        nextPin = i;
                        break;
                    }
                }
                this.stripConfig.push({ pin: nextPin, ledCount: 8 });
            }
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
                    alert('Error: Each strip must use a different pin.');
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
                } else {
                    alert('Error: ' + data.error);
                }
            } catch (err) {
                alert('Error saving configuration: ' + err.message);
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
