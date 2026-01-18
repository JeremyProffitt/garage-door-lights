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

        async sendBytecodeToStrip(deviceId, pin, bytecode) {
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
            console.log('[sendPatternToStrip] Starting for pattern:', pattern.name);
            console.log('[sendPatternToStrip] Has wledState:', !!pattern.wledState, 'length:', pattern.wledState?.length || 0);
            console.log('[sendPatternToStrip] Has wledBinary:', !!pattern.wledBinary);
            console.log('[sendPatternToStrip] Has bytecode:', !!pattern.bytecode);

            // Check if pattern already has WLED state or binary
            if (pattern.wledState || pattern.wledBinary || pattern.bytecode) {
                let bytecode = pattern.wledBinary || pattern.bytecode;
                console.log('[sendPatternToStrip] Inside WLED block, bytecode available:', !!bytecode);

                // If we have WLED JSON but no binary, compile it
                if (!bytecode && pattern.wledState) {
                    console.log('[sendPatternToStrip] Compiling wledState...');
                    const compileResp = await fetch('/api/glowblaster/compile', {
                        method: 'POST',
                        headers: {'Content-Type': 'application/json'},
                        credentials: 'same-origin',
                        body: JSON.stringify({ lcl: pattern.wledState })
                    });
                    const compileData = await compileResp.json();
                    console.log('[sendPatternToStrip] Compile response:', compileData);
                    if (compileData.success && compileData.data?.bytecode) {
                        bytecode = compileData.data.bytecode;
                        console.log('[sendPatternToStrip] Compiled bytecode length:', bytecode.length);
                    } else {
                        throw new Error('Failed to compile pattern: ' + (compileData.data?.errors?.join(', ') || 'Unknown error'));
                    }
                }

                if (bytecode) {
                    console.log('[sendPatternToStrip] Sending bytecode to strip...');
                    await this.sendBytecodeToStrip(deviceId, pin, bytecode);
                    console.log('[sendPatternToStrip] Bytecode sent successfully');
                    return;
                }
            }
            console.log('[sendPatternToStrip] Falling through to legacy path (no WLED data)');

            // Build WLED JSON from pattern fields (legacy patterns without wledState)
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

            console.log('[sendPatternToStrip] Built WLED JSON:', wledJson);

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

            console.log('[sendPatternToStrip] Compiled, sending bytecode...');

            // Send bytecode to device
            await this.sendBytecodeToStrip(deviceId, pin, compileData.data.bytecode);
        }
    }
}
