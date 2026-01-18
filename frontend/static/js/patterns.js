function patternsPage() {
    return {
        // Data
        patterns: [],
        effects: [],
        palettes: [],
        isLoading: true,
        isSaving: false,

        // Modal state
        showModal: false,
        showPreviewModal: false,
        editingPattern: null,
        previewPattern: null,

        // Form data
        form: {
            name: '',
            description: '',
            effectId: 71, // Candle by default
            paletteId: 0, // Default palette
            colors: [{ r: 255, g: 100, b: 0, percentage: 100 }],
            brightness: 200,
            speed: 128,
            intensity: 128,
            custom1: 128,
            custom2: 128,
            custom3: 128
        },

        // Computed
        get selectedEffect() {
            return this.effects.find(e => e.id === parseInt(this.form.effectId));
        },

        async init() {
            this.initPalettes();
            await Promise.all([
                this.loadEffects(),
                this.loadPatterns()
            ]);
            this.isLoading = false;
        },

        initPalettes() {
            // WLED palette list - common palettes
            this.palettes = [
                { id: 0, name: 'Default', description: 'Uses segment colors' },
                { id: 6, name: 'Party', description: 'Vibrant party colors' },
                { id: 7, name: 'Cloud', description: 'Soft blue and white cloud tones' },
                { id: 8, name: 'Lava', description: 'Hot red and orange lava colors' },
                { id: 9, name: 'Ocean', description: 'Deep blue ocean tones' },
                { id: 10, name: 'Forest', description: 'Natural green forest colors' },
                { id: 11, name: 'Rainbow', description: 'Full rainbow spectrum' },
                { id: 13, name: 'Sunset', description: 'Warm sunset gradient' },
                { id: 35, name: 'Fire', description: 'Fire colors palette' },
                { id: 36, name: 'Icefire', description: 'Cold ice meets hot fire' },
                { id: 39, name: 'Autumn', description: 'Fall season colors' },
                { id: 50, name: 'Aurora', description: 'Northern lights colors' }
            ];
        },

        async loadEffects() {
            try {
                const resp = await fetch('/api/effects', { credentials: 'same-origin' });
                const data = await resp.json();
                if (data.success) {
                    this.effects = data.data || [];
                }
            } catch (err) {
                console.error('Failed to load effects:', err);
                // Fallback to basic effects
                this.effects = [
                    { id: 0, name: 'Solid', description: 'Static solid color', minColors: 1, maxColors: 1 },
                    { id: 71, name: 'Candle', description: 'Flickering candle flame', hasSpeed: true, hasIntensity: true, minColors: 1, maxColors: 2 },
                    { id: 9, name: 'Rainbow', description: 'Moving rainbow gradient', hasSpeed: true, minColors: 0, maxColors: 0 }
                ];
            }
        },

        async loadPatterns() {
            try {
                const resp = await fetch('/api/patterns', { credentials: 'same-origin' });
                const data = await resp.json();
                if (data.success) {
                    this.patterns = (data.data || []).map(p => {
                        // Ensure colors array exists
                        if (!p.colors && p.red !== undefined) {
                            p.colors = [{ r: p.red, g: p.green, b: p.blue, percentage: 100 }];
                        }
                        return p;
                    });
                }
            } catch (err) {
                console.error('Failed to load patterns:', err);
            }
        },

        getEffectName(pattern) {
            // Try to find effect by ID in metadata
            if (pattern.metadata?.effectId) {
                const effect = this.effects.find(e => e.id === parseInt(pattern.metadata.effectId));
                if (effect) return effect.name;
            }
            // Map old type to effect name
            const typeMap = {
                'candle': 'Candle',
                'solid': 'Solid',
                'pulse': 'Breathe',
                'wave': 'Colorwaves',
                'rainbow': 'Rainbow',
                'fire': 'Fire 2012'
            };
            return typeMap[pattern.type] || pattern.type || 'Unknown';
        },

        openCreateModal() {
            this.editingPattern = null;
            this.resetForm();
            this.showModal = true;
            this.$nextTick(() => this.updatePreview());
        },

        editPattern(pattern) {
            this.editingPattern = pattern;

            // Map old type to effect ID
            const typeToEffectId = {
                'candle': 71,
                'solid': 0,
                'pulse': 2,
                'wave': 50,
                'rainbow': 9,
                'fire': 49
            };

            const effectId = pattern.metadata?.effectId
                ? parseInt(pattern.metadata.effectId)
                : (typeToEffectId[pattern.type] || 71);

            this.form = {
                name: pattern.name || '',
                description: pattern.description || '',
                effectId: effectId,
                paletteId: pattern.metadata?.paletteId ? parseInt(pattern.metadata.paletteId) : 0,
                colors: pattern.colors
                    ? JSON.parse(JSON.stringify(pattern.colors))
                    : [{ r: pattern.red || 255, g: pattern.green || 100, b: pattern.blue || 0, percentage: 100 }],
                brightness: pattern.brightness || 200,
                speed: pattern.metadata?.speed ? parseInt(pattern.metadata.speed) : (pattern.speed || 128),
                intensity: pattern.metadata?.intensity ? parseInt(pattern.metadata.intensity) : 128,
                custom1: pattern.metadata?.custom1 ? parseInt(pattern.metadata.custom1) : 128,
                custom2: pattern.metadata?.custom2 ? parseInt(pattern.metadata.custom2) : 128,
                custom3: pattern.metadata?.custom3 ? parseInt(pattern.metadata.custom3) : 128
            };

            this.showModal = true;
            this.$nextTick(() => this.updatePreview());
        },

        closeModal() {
            this.showModal = false;
            this.editingPattern = null;
        },

        resetForm() {
            this.form = {
                name: '',
                description: '',
                effectId: 71,
                paletteId: 0,
                colors: [{ r: 255, g: 100, b: 0, percentage: 100 }],
                brightness: 200,
                speed: 128,
                intensity: 128,
                custom1: 128,
                custom2: 128,
                custom3: 128
            };
        },

        onEffectChange() {
            const effect = this.selectedEffect;
            if (!effect) return;

            // Ensure minimum colors
            while (this.form.colors.length < effect.minColors) {
                this.addColor();
            }
            // Trim excess colors
            while (this.form.colors.length > Math.max(1, effect.maxColors)) {
                this.form.colors.pop();
            }

            this.updatePreview();
        },

        addColor() {
            const maxColors = this.selectedEffect?.maxColors || 3;
            if (this.form.colors.length < maxColors) {
                const newPercentage = Math.floor(100 / (this.form.colors.length + 1));
                this.form.colors.forEach(c => c.percentage = newPercentage);
                this.form.colors.push({ r: 255, g: 255, b: 255, percentage: newPercentage });
                this.updatePreview();
            }
        },

        removeColor(index) {
            if (this.form.colors.length > 1) {
                this.form.colors.splice(index, 1);
                this.normalizePercentages();
                this.updatePreview();
            }
        },

        normalizePercentages(changedIndex) {
            if (changedIndex === undefined) {
                const each = Math.floor(100 / this.form.colors.length);
                let remaining = 100 - (each * this.form.colors.length);
                this.form.colors.forEach((c, i) => {
                    c.percentage = each + (i < remaining ? 1 : 0);
                });
            } else {
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
            this.updatePreview();
        },

        updateColorFromPicker(index, hexValue) {
            const hex = hexValue.replace('#', '');
            this.form.colors[index].r = parseInt(hex.substr(0, 2), 16);
            this.form.colors[index].g = parseInt(hex.substr(2, 2), 16);
            this.form.colors[index].b = parseInt(hex.substr(4, 2), 16);
            this.updatePreview();
        },

        rgbToHex(r, g, b) {
            const toHex = (n) => {
                const hex = Math.max(0, Math.min(255, n)).toString(16);
                return hex.length === 1 ? '0' + hex : hex;
            };
            return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
        },

        updatePreview() {
            setTimeout(() => {
                const container = document.getElementById('livePreviewContainer');
                if (!container) return;

                const color = this.form.colors[0] || { r: 255, g: 100, b: 0 };
                const simPattern = {
                    type: this.getTypeFromEffectId(this.form.effectId),
                    red: color.r,
                    green: color.g,
                    blue: color.b,
                    brightness: Math.max(128, this.form.brightness),
                    speed: Math.max(10, Math.round(255 - this.form.speed) / 2.55) // Convert 0-255 to speed
                };

                if (typeof LEDSimulator !== 'undefined') {
                    LEDSimulator.render(container, simPattern, 8, { compact: true });
                }
            }, 50);
        },

        getTypeFromEffectId(effectId) {
            // Map WLED effect IDs to valid backend pattern types
            // Valid types: candle, solid, pulse, wave, rainbow, fire
            const effectToType = {
                0: 'solid',      // Solid
                1: 'pulse',      // Blink
                2: 'pulse',      // Breathe
                3: 'wave',       // Wipe (movement effect)
                9: 'rainbow',    // Rainbow
                12: 'wave',      // Fade
                13: 'wave',      // Theater Chase
                17: 'solid',     // Twinkle (point-based)
                20: 'solid',     // Sparkle (point-based)
                28: 'wave',      // Chase (movement effect)
                39: 'wave',      // Scanner (movement effect)
                40: 'wave',      // Larson Scanner
                41: 'wave',      // Comet
                42: 'fire',      // Fireworks
                46: 'solid',     // Gradient
                49: 'fire',      // Fire 2012
                50: 'wave',      // Colorwaves
                59: 'wave',      // Meteor (movement effect)
                71: 'candle',    // Candle
                79: 'wave',      // Ripple
                89: 'fire',      // Starburst
                91: 'wave',      // Bouncing Balls
                92: 'wave'       // Sinelon
            };
            return effectToType[parseInt(effectId)] || 'candle';
        },

        async savePattern() {
            if (!this.form.name.trim()) {
                NotificationBanner.error('Pattern name is required');
                return;
            }

            this.isSaving = true;

            try {
                const isEditing = !!this.editingPattern;
                const method = isEditing ? 'PUT' : 'POST';
                const url = isEditing
                    ? `/api/patterns/${this.editingPattern.patternId}`
                    : '/api/patterns';

                // Build WLED JSON
                const wledJson = this.buildWLEDJson();

                // Build payload
                const payload = {
                    name: this.form.name.trim(),
                    description: this.form.description.trim(),
                    type: this.getTypeFromEffectId(this.form.effectId),
                    colors: this.form.colors,
                    red: this.form.colors[0].r,
                    green: this.form.colors[0].g,
                    blue: this.form.colors[0].b,
                    brightness: this.form.brightness,
                    speed: Math.round((255 - this.form.speed) / 2.55), // Convert to ms-like value
                    metadata: {
                        effectId: String(this.form.effectId),
                        paletteId: String(this.form.paletteId),
                        speed: String(this.form.speed),
                        intensity: String(this.form.intensity),
                        custom1: String(this.form.custom1),
                        custom2: String(this.form.custom2),
                        custom3: String(this.form.custom3)
                    },
                    wledState: wledJson,
                    formatVersion: 2
                };

                const resp = await fetch(url, {
                    method,
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify(payload)
                });

                const data = await resp.json();

                if (data.success) {
                    this.showModal = false;
                    this.editingPattern = null;
                    await this.loadPatterns();
                    NotificationBanner.success(isEditing ? 'Pattern updated!' : 'Pattern created!');
                } else {
                    NotificationBanner.error('Error: ' + (data.error || 'Failed to save'));
                }
            } catch (err) {
                console.error('Save error:', err);
                NotificationBanner.error('Failed to save pattern');
            } finally {
                this.isSaving = false;
            }
        },

        buildWLEDJson() {
            const colors = this.form.colors.map(c => [
                Math.max(0, Math.min(255, c.r)),
                Math.max(0, Math.min(255, c.g)),
                Math.max(0, Math.min(255, c.b))
            ]);
            // Clamp all values to valid 0-255 range
            const clamp = (val) => Math.max(0, Math.min(255, val || 0));

            // Build segment with all parameters
            const segment = {
                id: 0,
                start: 0,
                stop: 8,
                fx: parseInt(this.form.effectId) || 0,
                sx: clamp(this.form.speed),
                ix: clamp(this.form.intensity),
                pal: parseInt(this.form.paletteId) || 0,
                col: colors,
                on: true
            };

            // Add custom parameters if the effect uses them
            const effect = this.selectedEffect;
            if (effect?.hasCustom1) {
                segment.c1 = clamp(this.form.custom1);
            }
            if (effect?.hasCustom2) {
                segment.c2 = clamp(this.form.custom2);
            }
            if (effect?.hasCustom3) {
                segment.c3 = clamp(this.form.custom3);
            }

            return JSON.stringify({
                on: true,
                bri: clamp(this.form.brightness),
                seg: [segment]
            });
        },

        async simulatePattern(pattern) {
            this.previewPattern = pattern;
            this.showPreviewModal = true;

            setTimeout(() => {
                const container = document.getElementById('previewContainer');
                if (!container) return;

                // Use WLED bytecode preview if available
                if (pattern.wledBinary || pattern.bytecode) {
                    const bytecode = pattern.wledBinary || pattern.bytecode;
                    if (typeof WLEDPreview !== 'undefined') {
                        WLEDPreview.render(container, bytecode, 12);
                    }
                } else {
                    // Standard LEDSimulator
                    const color = pattern.colors?.[0] || { r: pattern.red || 255, g: pattern.green || 100, b: pattern.blue || 0 };
                    const simPattern = {
                        type: pattern.type,
                        red: color.r,
                        green: color.g,
                        blue: color.b,
                        brightness: pattern.brightness || 200,
                        speed: pattern.speed || 50
                    };
                    if (typeof LEDSimulator !== 'undefined') {
                        LEDSimulator.render(container, simPattern, 12);
                    }
                }
            }, 100);
        },

        async deletePattern(patternId) {
            if (!confirm('Are you sure you want to delete this pattern?')) return;

            try {
                const resp = await fetch(`/api/patterns/${patternId}`, {
                    method: 'DELETE',
                    credentials: 'same-origin'
                });
                const data = await resp.json();

                if (data.success) {
                    await this.loadPatterns();
                    NotificationBanner.success('Pattern deleted');
                } else {
                    NotificationBanner.error('Error: ' + (data.error || 'Failed to delete'));
                }
            } catch (err) {
                NotificationBanner.error('Failed to delete pattern');
            }
        },

        editInGlowBlaster(pattern) {
            window.location.href = `/glowblaster?patternId=${pattern.patternId}`;
        }
    };
}
