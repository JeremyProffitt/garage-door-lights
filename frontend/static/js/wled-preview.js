// WLED Preview Interpreter
// Browser-side WLEDb binary interpreter for LED preview
// Parses WLED binary format and renders via LEDSimulator

const WLEDPreview = {
    // Binary format constants (must match wled_types.go)
    MAGIC: [0x57, 0x4C, 0x45, 0x44], // "WLED"
    VERSION: 0x01,
    HEADER_SIZE: 8,
    GLOBAL_SIZE: 4,
    MAX_SEGMENTS: 8,
    MAX_COLORS: 3,

    // Binary offsets
    OFFSET_MAGIC: 0,
    OFFSET_VERSION: 4,
    OFFSET_FLAGS: 5,
    OFFSET_LENGTH: 6,
    OFFSET_BRIGHTNESS: 8,
    OFFSET_TRANSITION: 9,
    OFFSET_SEGMENT_COUNT: 11,
    OFFSET_SEGMENTS_START: 12,

    // Per-segment offsets (relative to segment start)
    SEG_OFFSET_ID: 0,
    SEG_OFFSET_START: 1,
    SEG_OFFSET_STOP: 3,
    SEG_OFFSET_EFFECT_ID: 5,
    SEG_OFFSET_SPEED: 6,
    SEG_OFFSET_INTENSITY: 7,
    SEG_OFFSET_CUSTOM1: 8,
    SEG_OFFSET_CUSTOM2: 9,
    SEG_OFFSET_CUSTOM3: 10,
    SEG_OFFSET_PALETTE_ID: 11,
    SEG_OFFSET_FLAGS: 12,
    SEG_OFFSET_COLOR_CNT: 13,
    SEG_OFFSET_COLOR1: 14,

    // WLED Effect IDs (subset that we support)
    FX_SOLID: 0,
    FX_BREATHE: 2,
    FX_WIPE: 3,
    FX_RAINBOW: 9,
    FX_SPARKLE: 20,
    FX_CHASE: 27,
    FX_SCANNER: 39,
    FX_PRIDE: 46,
    FX_FIRE2012: 49,
    FX_COLORWAVES: 50,
    FX_CANDLE: 71,
    FX_METEOR: 59,
    FX_RIPPLE: 62,
    FX_TWINKLE: 17,
    FX_FIREWORKS: 72,
    FX_PALETTE: 48,

    // Map WLED effect ID to our simulator pattern type
    effectTypeMap: {
        0: 'solid',      // Solid
        2: 'pulse',      // Breathe
        3: 'wipe',       // Wipe
        9: 'rainbow',    // Rainbow
        17: 'sparkle',   // Twinkle
        20: 'sparkle',   // Sparkle
        27: 'chase',     // Chase
        39: 'scanner',   // Scanner
        46: 'gradient',  // Pride/Gradient
        48: 'gradient',  // Palette
        49: 'fire',      // Fire 2012
        50: 'wave',      // Colorwaves
        59: 'meteor',    // Meteor
        62: 'ripple',    // Ripple
        71: 'candle',    // Candle
        72: 'fireworks', // Fireworks
    },

    // Effect names for display
    effectNames: {
        0: 'Solid',
        2: 'Breathe',
        3: 'Wipe',
        9: 'Rainbow',
        17: 'Twinkle',
        20: 'Sparkle',
        27: 'Chase',
        39: 'Scanner',
        46: 'Gradient',
        48: 'Palette',
        49: 'Fire 2012',
        50: 'Colorwaves',
        59: 'Meteor',
        62: 'Ripple',
        71: 'Candle',
        72: 'Fireworks',
    },

    /**
     * Check if binary data is in WLED format
     * @param {Uint8Array} data - Binary data
     * @returns {boolean} True if WLED format
     */
    isWLEDFormat(data) {
        if (!data || data.length < 4) return false;
        return data[0] === this.MAGIC[0] &&
               data[1] === this.MAGIC[1] &&
               data[2] === this.MAGIC[2] &&
               data[3] === this.MAGIC[3];
    },

    /**
     * Parse WLEDb binary header
     * @param {Uint8Array} bytecode - Raw bytecode array
     * @returns {Object|null} Header info or null if invalid
     */
    parseHeader(bytecode) {
        if (!bytecode || bytecode.length < this.HEADER_SIZE + this.GLOBAL_SIZE) {
            console.error('WLEDPreview: Binary too short');
            return null;
        }

        // Check magic bytes "WLED"
        if (!this.isWLEDFormat(bytecode)) {
            console.error('WLEDPreview: Invalid magic bytes');
            return null;
        }

        const version = bytecode[this.OFFSET_VERSION];
        if (version !== this.VERSION) {
            console.warn('WLEDPreview: Unsupported version', version);
        }

        const flags = bytecode[this.OFFSET_FLAGS];
        const length = (bytecode[this.OFFSET_LENGTH] << 8) | bytecode[this.OFFSET_LENGTH + 1];

        return {
            version,
            flags,
            length,
            powerOn: (flags & 0x01) !== 0,
        };
    },

    /**
     * Parse WLEDb binary and extract WLED state
     * @param {Uint8Array} bytecode - Raw bytecode array
     * @returns {Object} WLED state object
     */
    parse(bytecode) {
        const header = this.parseHeader(bytecode);
        if (!header) {
            return this.defaultState();
        }

        const state = {
            on: header.powerOn,
            brightness: bytecode[this.OFFSET_BRIGHTNESS],
            transition: (bytecode[this.OFFSET_TRANSITION] << 8) | bytecode[this.OFFSET_TRANSITION + 1],
            segments: [],
        };

        const segmentCount = bytecode[this.OFFSET_SEGMENT_COUNT];
        if (segmentCount > this.MAX_SEGMENTS) {
            console.warn('WLEDPreview: Too many segments', segmentCount);
        }

        // Parse segments
        let offset = this.OFFSET_SEGMENTS_START;
        for (let i = 0; i < Math.min(segmentCount, this.MAX_SEGMENTS); i++) {
            if (offset + this.SEG_OFFSET_COLOR_CNT >= bytecode.length) {
                console.warn('WLEDPreview: Truncated segment data at segment', i);
                break;
            }

            const segment = {
                id: bytecode[offset + this.SEG_OFFSET_ID],
                start: (bytecode[offset + this.SEG_OFFSET_START] << 8) | bytecode[offset + this.SEG_OFFSET_START + 1],
                stop: (bytecode[offset + this.SEG_OFFSET_STOP] << 8) | bytecode[offset + this.SEG_OFFSET_STOP + 1],
                effectId: bytecode[offset + this.SEG_OFFSET_EFFECT_ID],
                speed: bytecode[offset + this.SEG_OFFSET_SPEED],
                intensity: bytecode[offset + this.SEG_OFFSET_INTENSITY],
                custom1: bytecode[offset + this.SEG_OFFSET_CUSTOM1],
                custom2: bytecode[offset + this.SEG_OFFSET_CUSTOM2],
                custom3: bytecode[offset + this.SEG_OFFSET_CUSTOM3],
                paletteId: bytecode[offset + this.SEG_OFFSET_PALETTE_ID],
                flags: bytecode[offset + this.SEG_OFFSET_FLAGS],
                reverse: (bytecode[offset + this.SEG_OFFSET_FLAGS] & 0x01) !== 0,
                mirror: (bytecode[offset + this.SEG_OFFSET_FLAGS] & 0x02) !== 0,
                on: (bytecode[offset + this.SEG_OFFSET_FLAGS] & 0x04) !== 0,
                colors: [],
            };

            // Parse colors
            const colorCount = Math.min(bytecode[offset + this.SEG_OFFSET_COLOR_CNT], this.MAX_COLORS);
            let colorOffset = offset + this.SEG_OFFSET_COLOR1;
            for (let c = 0; c < colorCount; c++) {
                if (colorOffset + 2 < bytecode.length) {
                    segment.colors.push([
                        bytecode[colorOffset],
                        bytecode[colorOffset + 1],
                        bytecode[colorOffset + 2],
                    ]);
                }
                colorOffset += 3;
            }

            state.segments.push(segment);

            // Move to next segment (skip checksum byte)
            offset = colorOffset + 1;
        }

        return state;
    },

    /**
     * Convert WLED state to simulator-compatible pattern
     * @param {Object} state - WLED state object
     * @param {number} segmentIndex - Segment index to render (default: 0)
     * @returns {Object} Pattern for LEDSimulator
     */
    toSimulatorPattern(state, segmentIndex = 0) {
        if (!state.segments || state.segments.length === 0) {
            return this.defaultPattern();
        }

        const segment = state.segments[Math.min(segmentIndex, state.segments.length - 1)];
        const effectType = this.effectTypeMap[segment.effectId] || 'solid';

        // Get primary color
        let red = 255, green = 255, blue = 255;
        if (segment.colors && segment.colors.length > 0) {
            [red, green, blue] = segment.colors[0];
        }

        // Build pattern object
        const pattern = {
            type: effectType,
            red,
            green,
            blue,
            brightness: state.brightness,
            speed: segment.speed,
        };

        // Add palette if multiple colors
        if (segment.colors && segment.colors.length > 1) {
            pattern.palette = segment.colors.map(([r, g, b]) => ({ r, g, b }));
        }

        // Map effect-specific parameters
        switch (segment.effectId) {
            case this.FX_FIRE2012:
                pattern.cooling = segment.intensity;
                pattern.sparking = segment.custom1;
                break;
            case this.FX_SCANNER:
                pattern.headSize = segment.intensity;
                pattern.tailLen = segment.custom1;
                break;
            case this.FX_SPARKLE:
            case this.FX_TWINKLE:
                pattern.density = segment.intensity;
                break;
            case this.FX_COLORWAVES:
                pattern.waveCount = Math.max(1, Math.floor(segment.intensity / 25));
                break;
            case this.FX_METEOR:
                pattern.tailLen = segment.intensity;
                break;
        }

        // Handle reverse
        if (segment.reverse) {
            pattern.direction = 1;
        }

        return pattern;
    },

    /**
     * Get default WLED state
     * @returns {Object} Default state
     */
    defaultState() {
        return {
            on: true,
            brightness: 200,
            transition: 0,
            segments: [{
                id: 0,
                start: 0,
                stop: 8,
                effectId: 0,
                speed: 128,
                intensity: 128,
                custom1: 0,
                custom2: 0,
                custom3: 0,
                paletteId: 0,
                reverse: false,
                mirror: false,
                on: true,
                colors: [[255, 255, 255]],
            }],
        };
    },

    /**
     * Get default pattern when binary is invalid
     * @returns {Object} Default pattern configuration
     */
    defaultPattern() {
        return {
            type: 'solid',
            red: 255,
            green: 255,
            blue: 255,
            brightness: 200,
            speed: 128,
        };
    },

    /**
     * Convert bytecode (array or base64) to Uint8Array
     * @param {Array|Uint8Array|string} bytecode - Bytecode in various formats
     * @returns {Uint8Array} Normalized bytecode array
     */
    normalizeBytecode(bytecode) {
        if (bytecode instanceof Uint8Array) {
            return bytecode;
        }
        if (Array.isArray(bytecode)) {
            return new Uint8Array(bytecode);
        }
        if (typeof bytecode === 'string') {
            // Assume base64
            try {
                const binary = atob(bytecode);
                const bytes = new Uint8Array(binary.length);
                for (let i = 0; i < binary.length; i++) {
                    bytes[i] = binary.charCodeAt(i);
                }
                return bytes;
            } catch (e) {
                console.error('WLEDPreview: Failed to decode base64:', e);
                return new Uint8Array(0);
            }
        }
        return new Uint8Array(0);
    },

    /**
     * Render bytecode to an LED strip preview
     * @param {HTMLElement} container - Container element
     * @param {Array|Uint8Array|string} bytecode - Compiled bytecode
     * @param {number} ledCount - Number of LEDs to display (default: 8)
     */
    render(container, bytecode, ledCount = 8) {
        if (!container) {
            console.error('WLEDPreview: No container provided');
            return;
        }

        const bytes = this.normalizeBytecode(bytecode);
        const state = this.parse(bytes);
        const pattern = this.toSimulatorPattern(state);

        // Use LEDSimulator to render the pattern
        if (typeof LEDSimulator !== 'undefined') {
            LEDSimulator.render(container, pattern, ledCount);
        } else {
            console.error('WLEDPreview: LEDSimulator not available');
            container.innerHTML = '<p style="color: #ef4444;">LED Simulator not loaded</p>';
        }
    },

    /**
     * Render multi-segment pattern with visual separation
     * @param {HTMLElement} container - Container element
     * @param {Array|Uint8Array|string} bytecode - Compiled bytecode
     */
    renderMultiSegment(container, bytecode) {
        if (!container) {
            console.error('WLEDPreview: No container provided');
            return;
        }

        const bytes = this.normalizeBytecode(bytecode);
        const state = this.parse(bytes);

        // Clear container
        container.innerHTML = '';

        if (!state.segments || state.segments.length === 0) {
            this.render(container, bytecode, 8);
            return;
        }

        // Render each segment
        state.segments.forEach((segment, index) => {
            const segmentDiv = document.createElement('div');
            segmentDiv.className = 'segment-preview mb-2';

            const label = document.createElement('div');
            label.className = 'text-xs text-zinc-500 mb-1';
            const effectName = this.effectNames[segment.effectId] || `Effect ${segment.effectId}`;
            label.textContent = `Segment ${index}: LEDs ${segment.start}-${segment.stop} (${effectName})`;
            segmentDiv.appendChild(label);

            const ledContainer = document.createElement('div');
            segmentDiv.appendChild(ledContainer);
            container.appendChild(segmentDiv);

            const pattern = this.toSimulatorPattern(state, index);
            const ledCount = segment.stop - segment.start;

            if (typeof LEDSimulator !== 'undefined') {
                LEDSimulator.render(ledContainer, pattern, ledCount);
            }
        });
    },

    /**
     * Get human-readable description of bytecode
     * @param {Array|Uint8Array|string} bytecode - Compiled bytecode
     * @returns {string} Description of the pattern
     */
    describe(bytecode) {
        const bytes = this.normalizeBytecode(bytecode);
        const state = this.parse(bytes);

        if (!state.segments || state.segments.length === 0) {
            return 'Invalid WLED pattern';
        }

        const descriptions = state.segments.map((seg, i) => {
            const effectName = this.effectNames[seg.effectId] || `Effect ${seg.effectId}`;
            const color = seg.colors && seg.colors[0]
                ? `#${seg.colors[0][0].toString(16).padStart(2, '0')}${seg.colors[0][1].toString(16).padStart(2, '0')}${seg.colors[0][2].toString(16).padStart(2, '0')}`
                : '#ffffff';
            return `Segment ${i}: ${effectName} (${color}, speed: ${seg.speed})`;
        });

        return `Brightness: ${state.brightness}\n${descriptions.join('\n')}`;
    },

    /**
     * Validate WLED bytecode format
     * @param {Array|Uint8Array|string} bytecode - Bytecode to validate
     * @returns {Object} Validation result { valid: boolean, errors: string[] }
     */
    validate(bytecode) {
        const bytes = this.normalizeBytecode(bytecode);
        const errors = [];

        if (bytes.length < this.HEADER_SIZE + this.GLOBAL_SIZE) {
            errors.push('Binary too short');
            return { valid: false, errors };
        }

        // Check magic
        if (!this.isWLEDFormat(bytes)) {
            errors.push('Invalid magic bytes (expected "WLED")');
        }

        // Check version
        if (bytes[this.OFFSET_VERSION] !== this.VERSION) {
            errors.push(`Unsupported version ${bytes[this.OFFSET_VERSION]} (expected ${this.VERSION})`);
        }

        // Check segment count
        const segmentCount = bytes[this.OFFSET_SEGMENT_COUNT];
        if (segmentCount === 0) {
            errors.push('No segments defined');
        }
        if (segmentCount > this.MAX_SEGMENTS) {
            errors.push(`Too many segments: ${segmentCount} (max ${this.MAX_SEGMENTS})`);
        }

        return {
            valid: errors.length === 0,
            errors
        };
    },

    /**
     * Parse WLED JSON string and convert to state object
     * @param {string} jsonStr - WLED JSON string
     * @returns {Object} WLED state object
     */
    parseJSON(jsonStr) {
        try {
            const json = JSON.parse(jsonStr);
            return {
                on: json.on !== false,
                brightness: json.bri || 200,
                transition: json.transition || 0,
                segments: (json.seg || []).map((seg, i) => ({
                    id: seg.id || i,
                    start: seg.start || 0,
                    stop: seg.stop || 8,
                    effectId: seg.fx || 0,
                    speed: seg.sx || 128,
                    intensity: seg.ix || 128,
                    custom1: seg.c1 || 0,
                    custom2: seg.c2 || 0,
                    custom3: seg.c3 || 0,
                    paletteId: seg.pal || 0,
                    reverse: seg.rev || false,
                    mirror: seg.mi || false,
                    on: seg.on !== false,
                    colors: (seg.col || [[255, 255, 255]]),
                })),
            };
        } catch (e) {
            console.error('WLEDPreview: Failed to parse JSON:', e);
            return this.defaultState();
        }
    },

    /**
     * Render from WLED JSON string
     * @param {HTMLElement} container - Container element
     * @param {string} jsonStr - WLED JSON string
     * @param {number} ledCount - Number of LEDs (default: 8)
     */
    renderFromJSON(container, jsonStr, ledCount = 8) {
        const state = this.parseJSON(jsonStr);
        const pattern = this.toSimulatorPattern(state);

        if (typeof LEDSimulator !== 'undefined') {
            LEDSimulator.render(container, pattern, ledCount);
        } else {
            console.error('WLEDPreview: LEDSimulator not available');
        }
    },
};

// PatternPreview for WLED format
const PatternPreview = {
    /**
     * Render bytecode
     * @param {HTMLElement} container - Container element
     * @param {Array|Uint8Array|string} bytecode - Compiled bytecode
     * @param {number} ledCount - Number of LEDs to display
     */
    render(container, bytecode, ledCount = 8) {
        let bytes;
        if (typeof bytecode === 'string') {
            // Could be base64 or JSON
            if (bytecode.trim().startsWith('{')) {
                // WLED JSON
                WLEDPreview.renderFromJSON(container, bytecode, ledCount);
                return;
            }
            // Assume base64
            bytes = WLEDPreview.normalizeBytecode(bytecode);
        } else {
            bytes = WLEDPreview.normalizeBytecode(bytecode);
        }

        WLEDPreview.render(container, bytes, ledCount);
    },

    /**
     * Describe bytecode
     * @param {Array|Uint8Array|string} bytecode - Compiled bytecode
     * @returns {string} Description
     */
    describe(bytecode) {
        return WLEDPreview.describe(bytecode);
    },

    /**
     * Validate bytecode
     * @param {Array|Uint8Array|string} bytecode - Bytecode to validate
     * @returns {Object} Validation result
     */
    validate(bytecode) {
        return WLEDPreview.validate(bytecode);
    },
};
