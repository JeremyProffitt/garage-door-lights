function devicesPage() {
    return {
        devices: [],
        patterns: [],
        showAddModal: false,
        showPatternModal: false,
        selectedDevice: null,
        isLoading: true,
        isRefreshing: false,
        refreshMessage: '',
        showHidden: false,
        newDevice: {
            name: '',
            particleId: ''
        },

        init() {
            this.loadDevices();
            this.loadPatterns();
        },

        get filteredDevices() {
            let filtered = this.devices.filter(d => this.showHidden || !d.isHidden);
            // Sort: online devices first, then by name
            return filtered.sort((a, b) => {
                if (a.isOnline !== b.isOnline) {
                    return b.isOnline ? 1 : -1;
                }
                return a.name.localeCompare(b.name);
            });
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

        async addDevice() {
            const resp = await fetch('/api/devices', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify(this.newDevice)
            });

            const data = await resp.json();

            if (data.success) {
                this.showAddModal = false;
                this.newDevice = {name: '', particleId: ''};
                this.loadDevices();
            } else {
                alert('Error: ' + data.error);
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
            this.showPatternModal = true;
        },

        async assignPattern(patternId) {
            const resp = await fetch(`/api/devices/${this.selectedDevice.deviceId}/pattern`, {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({patternId})
            });

            const data = await resp.json();

            if (data.success) {
                // Send pattern to device
                await this.sendPatternToDevice(this.selectedDevice.deviceId, patternId);
                this.showPatternModal = false;
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

        async testDevice(device) {
            // Send a test pattern (rainbow) to the device
            const resp = await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify({
                    deviceId: device.deviceId,
                    command: 'setPattern',
                    argument: '4:50'
                })
            });

            const data = await resp.json();

            if (data.success) {
                alert('Test pattern sent to ' + device.name);
            } else {
                alert('Error: ' + data.error);
            }
        },

        getPatternName(patternId) {
            const pattern = this.patterns.find(p => p.patternId === patternId);
            return pattern ? pattern.name : 'Unknown';
        }
    }
}
