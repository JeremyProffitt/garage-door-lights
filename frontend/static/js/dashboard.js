function dashboard() {
    return {
        devices: [],
        patterns: [],
        particleStatus: 'checking',
        selectedPatternId: '',
        isLoading: true,

        get deviceCount() {
            return this.devices.filter(d => !d.isHidden).length;
        },

        get readyDevices() {
            return this.devices
                .filter(d => !d.isHidden && d.isOnline && d.isReady)
                .sort((a, b) => a.name.localeCompare(b.name));
        },

        get onlineNotReadyCount() {
            return this.devices.filter(d => !d.isHidden && d.isOnline && !d.isReady).length;
        },

        get offlineCount() {
            return this.devices.filter(d => !d.isHidden && !d.isOnline).length;
        },

        init() {
            this.checkParticleConnection();
            this.loadDevices();
            this.loadPatterns();
        },

        async checkParticleConnection() {
            try {
                const resp = await fetch('/api/particle/devices/refresh', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success) {
                    this.particleStatus = 'connected';
                } else {
                    this.particleStatus = 'error';
                }
            } catch (err) {
                this.particleStatus = 'error';
            }
        },

        async loadDevices() {
            this.isLoading = true;
            try {
                const resp = await fetch('/api/devices', {
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success) {
                    this.devices = data.data || [];
                }
            } catch (err) {
                console.error('Failed to load devices:', err);
                this.devices = [];
            }
            this.isLoading = false;
        },

        async loadPatterns() {
            try {
                const resp = await fetch('/api/patterns', {
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success) {
                    this.patterns = (data.data || []).map(p => {
                        // Convert old single-color format to new multi-color format
                        if (!p.colors && p.red !== undefined) {
                            p.colors = [{ r: p.red, g: p.green, b: p.blue, percentage: 100 }];
                        }
                        return p;
                    });
                    // Auto-select the first pattern
                    if (this.patterns.length > 0) {
                        this.selectedPatternId = this.patterns[0].patternId;
                    }
                }
            } catch (err) {
                console.error('Failed to load patterns:', err);
                this.patterns = [];
            }
        },

        getSelectedPattern() {
            if (!this.selectedPatternId) return null;
            return this.patterns.find(p => p.patternId === this.selectedPatternId);
        },

        editSelectedPattern() {
            if (this.selectedPatternId) {
                // Navigate to patterns page with edit parameter
                window.location.href = `/patterns?edit=${this.selectedPatternId}`;
            }
        },

        getStripPatternId(device, pin) {
            const strip = device.ledStrips?.find(s => s.pin === pin);
            return strip?.patternId || '';
        },

        setStripPattern(device, stripIndex, patternId) {
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
        }
    }
}
