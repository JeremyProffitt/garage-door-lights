// LCL Preview Interpreter
// Browser-side bytecode interpreter for LED Control Language
// Parses compiled bytecode and renders via LEDSimulator

const LCLPreview = {
    // Bytecode constants (must match lcl_compiler.go)
    MAGIC: [0x4C, 0x43, 0x4C], // "LCL"
    VERSION: 0x01,
    HEADER_SIZE: 8,

    // Opcodes
    OP_PUSH_COLOR: 0x05,
    OP_PALETTE: 0x44,
    OP_SET_PATTERN: 0x50,
    OP_SET_PARAM: 0x51,
    OP_HALT: 0x67,

    // Effect types
    EFFECT_SOLID: 0x01,
    EFFECT_PULSE: 0x02,
    EFFECT_SPARKLE: 0x04,
    EFFECT_GRADIENT: 0x05,
    EFFECT_FIRE: 0x07,
    EFFECT_CANDLE: 0x08,
    EFFECT_WAVE: 0x09,
    EFFECT_RAINBOW: 0x0A,
    EFFECT_METEOR: 0x0B,
    EFFECT_BREATHING: 0x0C,

    // Parameter IDs (must match lcl_compiler.go)
    PARAM_RED: 0x01,
    PARAM_GREEN: 0x02,
    PARAM_BLUE: 0x03,
    PARAM_BRIGHTNESS: 0x04,
    PARAM_SPEED: 0x05,
    PARAM_COOLING: 0x06,
    PARAM_SPARKING: 0x07,
    PARAM_DIRECTION: 0x08,
    PARAM_WAVE_COUNT: 0x09,
    PARAM_HEAD_SIZE: 0x0A,
    PARAM_TAIL_LEN: 0x0B,
    PARAM_DENSITY: 0x0C,

    // Map effect type ID to LEDSimulator pattern type
    effectTypeMap: {
        0x01: 'solid',
        0x02: 'pulse',
        0x04: 'sparkle',
        0x05: 'gradient',
        0x07: 'fire',
        0x08: 'candle',
        0x09: 'wave',
        0x0A: 'rainbow',
        0x0B: 'meteor',
        0x0C: 'pulse' // breathing -> pulse approximation
    },

    /**
     * Parse bytecode header
     * @param {Uint8Array} bytecode - Raw bytecode array
     * @returns {Object|null} Header info or null if invalid
     */
    parseHeader(bytecode) {
        if (!bytecode || bytecode.length < this.HEADER_SIZE) {
            console.error('LCLPreview: Bytecode too short');
            return null;
        }

        // Check magic bytes "LCL"
        if (bytecode[0] !== this.MAGIC[0] ||
            bytecode[1] !== this.MAGIC[1] ||
            bytecode[2] !== this.MAGIC[2]) {
            console.error('LCLPreview: Invalid magic bytes');
            return null;
        }

        const version = bytecode[3];
        const length = (bytecode[4] << 8) | bytecode[5];
        const checksum = bytecode[6];
        const flags = bytecode[7];

        // Verify checksum
        let calcChecksum = 0;
        for (let i = this.HEADER_SIZE; i < bytecode.length; i++) {
            calcChecksum ^= bytecode[i];
        }
        if (calcChecksum !== checksum) {
            console.warn('LCLPreview: Checksum mismatch (expected', checksum, 'got', calcChecksum, ')');
            // Continue anyway for preview purposes
        }

        return {
            version,
            length,
            checksum,
            flags,
            codeStart: this.HEADER_SIZE
        };
    },

    /**
     * Execute bytecode and extract pattern configuration
     * @param {Uint8Array} bytecode - Raw bytecode array
     * @returns {Object} Pattern configuration for LEDSimulator
     */
    execute(bytecode) {
        const header = this.parseHeader(bytecode);
        if (!header) {
            return this.defaultPattern();
        }

        // Default pattern state
        const pattern = {
            type: 'solid',
            red: 255,
            green: 147,
            blue: 41,
            brightness: 128,
            speed: 50,
            palette: null  // Custom color palette
        };

        // Color stack for palette operations
        const colorStack = [];

        // Execute bytecode instructions
        let pc = header.codeStart;
        while (pc < bytecode.length) {
            const opcode = bytecode[pc];

            switch (opcode) {
                case this.OP_PUSH_COLOR:
                    // Push RGB color onto stack (3 bytes follow)
                    if (pc + 3 < bytecode.length) {
                        const r = bytecode[pc + 1];
                        const g = bytecode[pc + 2];
                        const b = bytecode[pc + 3];
                        colorStack.push({ r, g, b });
                        pc += 4;
                    } else {
                        pc++;
                    }
                    break;

                case this.OP_PALETTE:
                    // Pop N colors from stack and set as palette (1 byte follows: count)
                    if (pc + 1 < bytecode.length) {
                        const count = bytecode[pc + 1];
                        // Colors are pushed in order, so take the last N from stack
                        if (colorStack.length >= count) {
                            pattern.palette = colorStack.splice(-count, count);
                        } else {
                            // Use all available colors if not enough
                            pattern.palette = [...colorStack];
                            colorStack.length = 0;
                        }
                        pc += 2;
                    } else {
                        pc++;
                    }
                    break;

                case this.OP_SET_PATTERN:
                    if (pc + 1 < bytecode.length) {
                        const effectType = bytecode[pc + 1];
                        pattern.type = this.effectTypeMap[effectType] || 'solid';
                        pc += 2;
                    } else {
                        pc++;
                    }
                    break;

                case this.OP_SET_PARAM:
                    if (pc + 2 < bytecode.length) {
                        const paramId = bytecode[pc + 1];
                        const value = bytecode[pc + 2];
                        this.applyParam(pattern, paramId, value);
                        pc += 3;
                    } else {
                        pc++;
                    }
                    break;

                case this.OP_HALT:
                    return pattern;

                default:
                    // Unknown opcode, skip it
                    pc++;
                    break;
            }
        }

        return pattern;
    },

    /**
     * Apply a parameter value to the pattern
     * @param {Object} pattern - Pattern object to modify
     * @param {number} paramId - Parameter ID
     * @param {number} value - Parameter value (0-255)
     */
    applyParam(pattern, paramId, value) {
        switch (paramId) {
            case this.PARAM_RED:
                pattern.red = value;
                break;
            case this.PARAM_GREEN:
                pattern.green = value;
                break;
            case this.PARAM_BLUE:
                pattern.blue = value;
                break;
            case this.PARAM_BRIGHTNESS:
                pattern.brightness = value;
                break;
            case this.PARAM_SPEED:
                pattern.speed = value;
                break;
            case this.PARAM_COOLING:
                pattern.cooling = value;
                break;
            case this.PARAM_SPARKING:
                pattern.sparking = value;
                break;
            case this.PARAM_DIRECTION:
                pattern.direction = value;
                break;
            case this.PARAM_WAVE_COUNT:
                pattern.waveCount = value;
                break;
            case this.PARAM_HEAD_SIZE:
                pattern.headSize = value;
                break;
            case this.PARAM_TAIL_LEN:
                pattern.tailLen = value;
                break;
            case this.PARAM_DENSITY:
                pattern.density = value;
                break;
        }
    },

    /**
     * Get default pattern when bytecode is invalid
     * @returns {Object} Default pattern configuration
     */
    defaultPattern() {
        return {
            type: 'candle',
            red: 255,
            green: 147,
            blue: 41,
            brightness: 128,
            speed: 50
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
                console.error('LCLPreview: Failed to decode base64:', e);
                return new Uint8Array(0);
            }
        }
        return new Uint8Array(0);
    },

    /**
     * Render bytecode to an LED strip preview
     * @param {HTMLElement} container - Container element
     * @param {Array|Uint8Array|string} bytecode - Compiled bytecode
     * @param {number} ledCount - Number of LEDs to display (default: 12)
     */
    render(container, bytecode, ledCount = 12) {
        if (!container) {
            console.error('LCLPreview: No container provided');
            return;
        }

        const bytes = this.normalizeBytecode(bytecode);
        const pattern = this.execute(bytes);

        // Use LEDSimulator to render the pattern
        if (typeof LEDSimulator !== 'undefined') {
            LEDSimulator.render(container, pattern, ledCount);
        } else {
            console.error('LCLPreview: LEDSimulator not available');
            container.innerHTML = '<p style="color: #ef4444;">LED Simulator not loaded</p>';
        }
    },

    /**
     * Get human-readable description of bytecode
     * @param {Array|Uint8Array|string} bytecode - Compiled bytecode
     * @returns {string} Description of the pattern
     */
    describe(bytecode) {
        const bytes = this.normalizeBytecode(bytecode);
        const pattern = this.execute(bytes);

        const colorHex = '#' +
            pattern.red.toString(16).padStart(2, '0') +
            pattern.green.toString(16).padStart(2, '0') +
            pattern.blue.toString(16).padStart(2, '0');

        let desc = `${pattern.type} pattern (${colorHex}, brightness: ${pattern.brightness}, speed: ${pattern.speed})`;

        // Add palette info if present
        if (pattern.palette && pattern.palette.length > 0) {
            const paletteColors = pattern.palette.map(c =>
                '#' + c.r.toString(16).padStart(2, '0') +
                      c.g.toString(16).padStart(2, '0') +
                      c.b.toString(16).padStart(2, '0')
            ).join(', ');
            desc += ` [palette: ${paletteColors}]`;
        }

        return desc;
    },

    /**
     * Validate bytecode format
     * @param {Array|Uint8Array|string} bytecode - Bytecode to validate
     * @returns {Object} Validation result { valid: boolean, errors: string[] }
     */
    validate(bytecode) {
        const bytes = this.normalizeBytecode(bytecode);
        const errors = [];

        if (bytes.length < this.HEADER_SIZE) {
            errors.push('Bytecode too short');
            return { valid: false, errors };
        }

        // Check magic
        if (bytes[0] !== this.MAGIC[0] ||
            bytes[1] !== this.MAGIC[1] ||
            bytes[2] !== this.MAGIC[2]) {
            errors.push('Invalid magic bytes (expected "LCL")');
        }

        // Check version
        if (bytes[3] !== this.VERSION) {
            errors.push(`Unsupported version ${bytes[3]} (expected ${this.VERSION})`);
        }

        // Check length
        const declaredLength = (bytes[4] << 8) | bytes[5];
        const actualLength = bytes.length - this.HEADER_SIZE;
        if (declaredLength !== actualLength) {
            errors.push(`Length mismatch: header says ${declaredLength}, actual is ${actualLength}`);
        }

        // Verify checksum
        let checksum = 0;
        for (let i = this.HEADER_SIZE; i < bytes.length; i++) {
            checksum ^= bytes[i];
        }
        if (checksum !== bytes[6]) {
            errors.push(`Checksum mismatch: expected ${bytes[6]}, calculated ${checksum}`);
        }

        return {
            valid: errors.length === 0,
            errors
        };
    }
};
