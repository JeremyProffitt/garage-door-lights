function patternsPage() {
    return {
        patterns: [],
        showCreateModal: false,
        showColorPicker: false,
        showRGBHelp: false,
        editingPattern: null,
        currentColorIndex: null,
        tempColor: '#ff6400',
        tempColorRGB: { r: 255, g: 100, b: 0 },
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
            this.loadPatterns();
        },

        async loadPatterns() {
            const resp = await fetch('/api/patterns');
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

        addColor() {
            if (this.form.colors.length < 8) {
                const newPercentage = Math.floor(100 / (this.form.colors.length + 1));
                // Redistribute percentages
                this.form.colors.forEach(c => c.percentage = newPercentage);
                this.form.colors.push({ r: 255, g: 255, b: 255, percentage: newPercentage });
            }
        },

        removeColor(index) {
            if (this.form.colors.length > 1) {
                this.form.colors.splice(index, 1);
                this.normalizePercentages();
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
        },

        cancelEdit() {
            this.showCreateModal = false;
            this.editingPattern = null;
            this.resetForm();
        },

        async savePattern() {
            const method = this.editingPattern ? 'PUT' : 'POST';
            const url = this.editingPattern
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
                body: JSON.stringify(payload)
            });

            const data = await resp.json();

            if (data.success) {
                this.showCreateModal = false;
                this.editingPattern = null;
                this.resetForm();
                this.loadPatterns();
            } else {
                alert('Error: ' + data.error);
            }
        },

        async deletePattern(patternId) {
            if (!confirm('Are you sure you want to delete this pattern?')) return;

            const resp = await fetch(`/api/patterns/${patternId}`, {
                method: 'DELETE'
            });

            const data = await resp.json();

            if (data.success) {
                this.loadPatterns();
            } else {
                alert('Error: ' + data.error);
            }
        },

        resetForm() {
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
        }
    }
}
