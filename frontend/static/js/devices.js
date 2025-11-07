function devicesPage() {
    return {
        devices: [],
        patterns: [],
        showAddModal: false,
        showPatternModal: false,
        selectedDevice: null,
        newDevice: {
            name: '',
            particleId: ''
        },

        init() {
            this.loadDevices();
            this.loadPatterns();
        },

        async loadDevices() {
            const resp = await fetch('/api/devices');
            const data = await resp.json();
            if (data.success) {
                this.devices = data.data || [];
            }
        },

        async loadPatterns() {
            const resp = await fetch('/api/patterns');
            const data = await resp.json();
            if (data.success) {
                this.patterns = data.data || [];
            }
        },

        async addDevice() {
            const resp = await fetch('/api/devices', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
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

        async deleteDevice(deviceId) {
            if (!confirm('Are you sure you want to delete this device?')) return;

            const resp = await fetch(`/api/devices/${deviceId}`, {
                method: 'DELETE'
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
                body: JSON.stringify({deviceId, patternId})
            });
        },

        async testDevice(device) {
            // Send a test pattern (rainbow) to the device
            const resp = await fetch('/api/particle/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
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
