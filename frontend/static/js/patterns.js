function patternsPage() {
    return {
        patterns: [],
        devices: [],
        showCreateModal: false,
        showColorPicker: false,
        showRGBHelp: false,
        showSimulateModal: false,
        showUpdateDevicesModal: false,
        editingPattern: null,
        simulatingPattern: null,
        currentColorIndex: null,
        activeColorTab: 0,
        tempColor: '#ff6400',
        tempColorRGB: { r: 255, g: 100, b: 0 },
        updatedPatternId: null,
        isUpdatingDevices: false,
        form: {
            name: '',
            description: '',
            type: 'candle',
            colors: [
                { r: 255, g: 100, b: 0, percentage: 100 }
            ],
            brightness: 128,
            speed: 50
        },

        init() {
            this.loadPatterns().then(() => {
                // Check for edit query parameter
                const urlParams = new URLSearchParams(window.location.search);
                const editId = urlParams.get('edit');
                if (editId) {
                    const pattern = this.patterns.find(p => p.patternId === editId);
                    if (pattern) {
                        this.editPattern(pattern);
                    }
                    // Clean up URL
                    window.history.replaceState({}, document.title, window.location.pathname);
                }
            });
            this.loadDevices();
            // Watch for form changes to update live preview
            this.$watch('form', () => {
                if (this.showCreateModal) {
                    this.updateLivePreview();
                }
            }, { deep: true });
        },

        async loadDevices() {
            const resp = await fetch('/api/devices', {
                credentials: 'same-origin'
            });
            const data = await resp.json();
            if (data.success) {
                this.devices = data.data || [];
            }
        },

        async loadPatterns() {
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
            }
        },

        simulatePattern(pattern) {
            this.simulatingPattern = pattern;
            this.showSimulateModal = true;
            setTimeout(() => {
                const container = document.getElementById('simulateLedsContainer');
                if (container) {
                    const simPattern = {
                        type: pattern.type,
                        red: pattern.colors ? pattern.colors[0].r : pattern.red,
                        green: pattern.colors ? pattern.colors[0].g : pattern.green,
                        blue: pattern.colors ? pattern.colors[0].b : pattern.blue,
                        brightness: pattern.brightness,
                        speed: pattern.speed
                    };
                    LEDSimulator.render(container, simPattern, 8);
                }
            }, 100);
        },

        updateLivePreview() {
            setTimeout(() => {
                const container = document.getElementById('editLedsContainer');
                if (container) {
                    const activeColor = this.form.colors[this.activeColorTab] || this.form.colors[0];
                    const simPattern = {
                        type: this.form.type,
                        red: activeColor.r,
                        green: activeColor.g,
                        blue: activeColor.b,
                        brightness: Math.max(128, this.form.brightness),
                        speed: this.form.speed
                    };
                    LEDSimulator.render(container, simPattern, 8, { compact: true });
                }
            }, 50);
        },

        addColor() {
            if (this.form.colors.length < 8) {
                const newPercentage = Math.floor(100 / (this.form.colors.length + 1));
                // Redistribute percentages
                this.form.colors.forEach(c => c.percentage = newPercentage);
                this.form.colors.push({ r: 255, g: 255, b: 255, percentage: newPercentage });
                // Switch to the newly added color tab
                this.activeColorTab = this.form.colors.length - 1;
                this.updateLivePreview();
            }
        },

        removeColor(index) {
            if (this.form.colors.length > 1) {
                this.form.colors.splice(index, 1);
                // If we removed the active tab, switch to the previous one (or 0 if we removed the first one)
                if (this.activeColorTab >= this.form.colors.length) {
                    this.activeColorTab = this.form.colors.length - 1;
                }
                this.normalizePercentages();
                this.updateLivePreview();
            }
        },

        normalizePercentages(changedIndex) {
            if (changedIndex === undefined) {
                // Equal distribution
                const each = Math.floor(100 / this.form.colors.length);
                let remaining = 100 - (each * this.form.colors.length);
                this.form.colors.forEach((c, i) => {
                    c.percentage = each + (i < remaining ? 1 : 0);
                });
            } else {
                // Redistribute remaining percentage among other colors
                const changedPercentage = this.form.colors[changedIndex].percentage;
                const otherColors = this.form.colors.length - 1;
                const remaining = 100 - changedPercentage;

                if (remaining < 0) {
                    this.form.colors[changedIndex].percentage = 100;
                    this.form.colors.forEach((c, i) => {
                        if (i !== changedIndex) c.percentage = 0;
                    });
                } else if (otherColors > 0) {
                    const each = Math.floor(remaining / otherColors);
                    let leftover = remaining - (each * otherColors);

                    this.form.colors.forEach((c, i) => {
                        if (i !== changedIndex) {
                            c.percentage = each + (leftover > 0 ? 1 : 0);
                            if (leftover > 0) leftover--;
                        }
                    });
                }
            }
        },

        openColorPicker(index) {
            this.currentColorIndex = index;
            const color = this.form.colors[index];
            this.tempColorRGB = { r: color.r, g: color.g, b: color.b };
            this.updateTempColorFromRGB();
            this.showColorPicker = true;
        },

        updateTempColorRGB() {
            // Convert hex color to RGB
            const hex = this.tempColor.replace('#', '');
            this.tempColorRGB = {
                r: parseInt(hex.substr(0, 2), 16),
                g: parseInt(hex.substr(2, 2), 16),
                b: parseInt(hex.substr(4, 2), 16)
            };
        },

        updateTempColorFromRGB() {
            // Convert RGB to hex color
            const toHex = (n) => {
                const hex = Math.max(0, Math.min(255, n)).toString(16);
                return hex.length === 1 ? '0' + hex : hex;
            };
            this.tempColor = `#${toHex(this.tempColorRGB.r)}${toHex(this.tempColorRGB.g)}${toHex(this.tempColorRGB.b)}`;
        },

        applyColorPicker() {
            if (this.currentColorIndex !== null) {
                this.form.colors[this.currentColorIndex].r = this.tempColorRGB.r;
                this.form.colors[this.currentColorIndex].g = this.tempColorRGB.g;
                this.form.colors[this.currentColorIndex].b = this.tempColorRGB.b;
            }
            this.showColorPicker = false;
        },

        editPattern(pattern) {
            this.editingPattern = pattern;
            this.activeColorTab = 0;
            this.form = {
                name: pattern.name,
                description: pattern.description,
                type: pattern.type,
                colors: pattern.colors ? JSON.parse(JSON.stringify(pattern.colors)) : [
                    { r: pattern.red || 255, g: pattern.green || 100, b: pattern.blue || 0, percentage: 100 }
                ],
                brightness: pattern.brightness,
                speed: pattern.speed
            };
            this.showCreateModal = true;
            setTimeout(() => this.updateLivePreview(), 200);
        },

        cancelEdit() {
            this.showCreateModal = false;
            this.editingPattern = null;
            this.resetForm();
        },

        async savePattern() {
            const isEditing = !!this.editingPattern;
            const method = isEditing ? 'PUT' : 'POST';
            const url = isEditing
                ? `/api/patterns/${this.editingPattern.patternId}`
                : '/api/patterns';

            // Prepare payload with backward compatibility
            const payload = {
                ...this.form,
                // Keep old format for backward compatibility
                red: this.form.colors[0].r,
                green: this.form.colors[0].g,
                blue: this.form.colors[0].b
            };

            const resp = await fetch(url, {
                method,
                headers: {'Content-Type': 'application/json'},
                credentials: 'same-origin',
                body: JSON.stringify(payload)
            });

            const data = await resp.json();

            if (data.success) {
                const patternId = isEditing ? this.editingPattern.patternId : data.data?.patternId;
                this.showCreateModal = false;
                this.editingPattern = null;
                this.resetForm();
                await this.loadPatterns();

                // If editing an existing pattern, check if any devices use it and offer to update them
                if (isEditing && patternId) {
                    const devicesUsingPattern = this.getDevicesUsingPattern(patternId);
                    if (devicesUsingPattern.length > 0) {
                        this.updatedPatternId = patternId;
                        this.showUpdateDevicesModal = true;
                    } else {
                        NotificationBanner.success('Pattern saved successfully');
                    }
                } else {
                    NotificationBanner.success('Pattern created successfully');
                }
            } else {
                NotificationBanner.error('Error: ' + data.error);
            }
        },

        getDevicesUsingPattern(patternId) {
            const devicesUsingPattern = [];
            for (const device of this.devices) {
                // Check device-level assignment
                if (device.assignedPattern === patternId) {
                    devicesUsingPattern.push({ device, strips: null });
                    continue;
                }
                // Check strip-level assignments
                if (device.ledStrips) {
                    const stripsWithPattern = device.ledStrips.filter(s => s.patternId === patternId);
                    if (stripsWithPattern.length > 0) {
                        devicesUsingPattern.push({ device, strips: stripsWithPattern });
                    }
                }
            }
            return devicesUsingPattern;
        },

        async updateAllDevicesWithPattern() {
            if (!this.updatedPatternId) return;

            this.isUpdatingDevices = true;
            const pattern = this.patterns.find(p => p.patternId === this.updatedPatternId);
            if (!pattern) {
                NotificationBanner.error('Pattern not found');
                this.isUpdatingDevices = false;
                return;
            }

            const devicesToUpdate = this.getDevicesUsingPattern(this.updatedPatternId);

            try {
                for (const { device, strips } of devicesToUpdate) {
                    if (!device.isOnline || !device.isReady) continue;

                    if (strips) {
                        // Update specific strips
                        for (const strip of strips) {
                            await this.sendPatternToStrip(device.deviceId, strip.pin, pattern);
                        }
                    } else {
                        // Update all strips on the device
                        if (device.ledStrips && device.ledStrips.length > 0) {
                            for (const strip of device.ledStrips) {
                                await this.sendPatternToStrip(device.deviceId, strip.pin, pattern);
                            }
                        }
                    }
                }
                NotificationBanner.success('All devices updated successfully!');
            } catch (err) {
                NotificationBanner.error('Error updating devices: ' + err.message);
            } finally {
                this.isUpdatingDevices = false;
                this.showUpdateDevicesModal = false;
                this.updatedPatternId = null;
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

            // Send pattern command
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

            // Send color command
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

            // Send brightness command
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

        skipUpdateDevices() {
            this.showUpdateDevicesModal = false;
            this.updatedPatternId = null;
            NotificationBanner.success('Pattern saved successfully');
        },

        async deletePattern(patternId) {
            if (!confirm('Are you sure you want to delete this pattern?')) return;

            const resp = await fetch(`/api/patterns/${patternId}`, {
                method: 'DELETE',
                credentials: 'same-origin'
            });

            const data = await resp.json();

            if (data.success) {
                this.loadPatterns();
                NotificationBanner.success('Pattern deleted successfully');
            } else {
                NotificationBanner.error('Error: ' + data.error);
            }
        },

        resetForm() {
            this.activeColorTab = 0;
            this.form = {
                name: '',
                description: '',
                type: 'candle',
                colors: [
                    { r: 255, g: 100, b: 0, percentage: 100 }
                ],
                brightness: 128,
                speed: 50
            };
        },

        rgbToHex(r, g, b) {
            const toHex = (n) => {
                const hex = Math.max(0, Math.min(255, n)).toString(16);
                return hex.length === 1 ? '0' + hex : hex;
            };
            return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
        },

        updateColorFromPicker(index, hexValue) {
            const hex = hexValue.replace('#', '');
            this.form.colors[index].r = parseInt(hex.substr(0, 2), 16);
            this.form.colors[index].g = parseInt(hex.substr(2, 2), 16);
            this.form.colors[index].b = parseInt(hex.substr(4, 2), 16);
        },

        editInGlowBlaster(pattern) {
            // Navigate to Glow Blaster with the pattern ID
            window.location.href = `/glowblaster?patternId=${pattern.patternId}`;
        }
    }
}
