// LED Simulator Utility
// Simulates WS2812 LED behavior based on pattern type

const LEDSimulator = {
    // Track active intervals per container to prevent memory leaks
    _activeIntervals: new Map(),

    /**
     * Create and render an LED strip
     * @param {HTMLElement} container - Container element to render LEDs into
     * @param {Object} pattern - Pattern object with type, red, green, blue, brightness, speed
     * @param {number} ledCount - Number of LEDs to display (default: 8)
     * @param {Object} options - Optional settings { compact: boolean }
     */
    render(container, pattern, ledCount = 8, options = {}) {
        if (!container) return;

        // Clear any existing intervals for this container
        this.clearIntervals(container);

        container.innerHTML = '';
        container.className = 'led-strip' + (options.compact ? ' compact' : '');

        const leds = [];
        for (let i = 0; i < ledCount; i++) {
            const led = document.createElement('div');
            led.className = 'led';
            led.dataset.index = i;
            container.appendChild(led);
            leds.push(led);
        }

        this.animate(leds, pattern, container);
        return leds;
    },

    /**
     * Clear all intervals for a container
     * @param {HTMLElement} container - Container to clear intervals for
     */
    clearIntervals(container) {
        const intervals = this._activeIntervals.get(container);
        if (intervals) {
            intervals.forEach(id => clearInterval(id));
            this._activeIntervals.delete(container);
        }
    },

    /**
     * Register an interval for a container
     * @param {HTMLElement} container - Container to track interval for
     * @param {number} intervalId - Interval ID to track
     */
    trackInterval(container, intervalId) {
        if (!this._activeIntervals.has(container)) {
            this._activeIntervals.set(container, []);
        }
        this._activeIntervals.get(container).push(intervalId);
    },

    /**
     * Animate LEDs based on pattern type
     * @param {Array<HTMLElement>} leds - Array of LED elements
     * @param {Object} pattern - Pattern object
     * @param {HTMLElement} container - Container element for interval tracking
     */
    animate(leds, pattern, container) {
        const brightness = (pattern.brightness || 128) / 255;
        const speed = pattern.speed || 50;

        switch (pattern.type) {
            case 'candle':
                this.animateCandle(leds, pattern, brightness, container);
                break;
            case 'solid':
                this.animateSolid(leds, pattern, brightness);
                break;
            case 'pulse':
                this.animatePulse(leds, pattern, brightness, speed, container);
                break;
            case 'wave':
                this.animateWave(leds, pattern, brightness, speed, container);
                break;
            case 'rainbow':
                this.animateRainbow(leds, brightness, speed, container);
                break;
            case 'fire':
                this.animateFire(leds, brightness, container);
                break;
            default:
                this.animateSolid(leds, pattern, brightness);
        }
    },

    animateSolid(leds, pattern, brightness) {
        const r = Math.round((pattern.red || 0) * brightness);
        const g = Math.round((pattern.green || 0) * brightness);
        const b = Math.round((pattern.blue || 0) * brightness);
        const color = `rgb(${r}, ${g}, ${b})`;

        leds.forEach(led => {
            led.style.backgroundColor = color;
            led.style.color = color;
        });
    },

    animateCandle(leds, pattern, brightness, container) {
        const baseR = Math.round((pattern.red || 255) * brightness);
        const baseG = Math.round((pattern.green || 147) * brightness);
        const baseB = Math.round((pattern.blue || 41) * brightness);

        leds.forEach((led, i) => {
            // Set initial color immediately
            const initialColor = `rgb(${baseR}, ${baseG}, ${baseB})`;
            led.style.backgroundColor = initialColor;
            led.style.color = initialColor;

            const intervalId = setInterval(() => {
                const flicker = 0.8 + Math.random() * 0.2;
                const r = Math.round(baseR * flicker);
                const g = Math.round(baseG * flicker);
                const b = Math.round(baseB * flicker);
                const color = `rgb(${r}, ${g}, ${b})`;
                led.style.backgroundColor = color;
                led.style.color = color;
            }, 50 + Math.random() * 100);
            if (container) this.trackInterval(container, intervalId);
        });
    },

    animatePulse(leds, pattern, brightness, speed, container) {
        const baseR = pattern.red || 0;
        const baseG = pattern.green || 0;
        const baseB = pattern.blue || 0;
        let phase = 0;

        // Set initial color immediately
        const initialColor = `rgb(${Math.round(baseR * brightness)}, ${Math.round(baseG * brightness)}, ${Math.round(baseB * brightness)})`;
        leds.forEach(led => {
            led.style.backgroundColor = initialColor;
            led.style.color = initialColor;
        });

        const intervalId = setInterval(() => {
            phase += (speed / 50) * 0.1;
            const pulseBrightness = brightness * (0.3 + 0.7 * (Math.sin(phase) * 0.5 + 0.5));
            const r = Math.round(baseR * pulseBrightness);
            const g = Math.round(baseG * pulseBrightness);
            const b = Math.round(baseB * pulseBrightness);
            const color = `rgb(${r}, ${g}, ${b})`;

            leds.forEach(led => {
                led.style.backgroundColor = color;
                led.style.color = color;
            });
        }, 50);
        if (container) this.trackInterval(container, intervalId);
    },

    animateWave(leds, pattern, brightness, speed, container) {
        const baseR = pattern.red || 0;
        const baseG = pattern.green || 0;
        const baseB = pattern.blue || 0;
        let offset = 0;

        // Set initial colors immediately
        leds.forEach((led, i) => {
            const phase = (i / leds.length) * Math.PI * 2;
            const waveBrightness = brightness * (0.3 + 0.7 * (Math.sin(phase) * 0.5 + 0.5));
            const r = Math.round(baseR * waveBrightness);
            const g = Math.round(baseG * waveBrightness);
            const b = Math.round(baseB * waveBrightness);
            const color = `rgb(${r}, ${g}, ${b})`;
            led.style.backgroundColor = color;
            led.style.color = color;
        });

        const intervalId = setInterval(() => {
            offset += (speed / 50) * 0.2;

            leds.forEach((led, i) => {
                const phase = (i / leds.length) * Math.PI * 2 + offset;
                const waveBrightness = brightness * (0.3 + 0.7 * (Math.sin(phase) * 0.5 + 0.5));
                const r = Math.round(baseR * waveBrightness);
                const g = Math.round(baseG * waveBrightness);
                const b = Math.round(baseB * waveBrightness);
                const color = `rgb(${r}, ${g}, ${b})`;
                led.style.backgroundColor = color;
                led.style.color = color;
            });
        }, 50);
        if (container) this.trackInterval(container, intervalId);
    },

    animateRainbow(leds, brightness, speed, container) {
        let hueOffset = 0;

        // Set initial colors immediately
        leds.forEach((led, i) => {
            const hue = ((i / leds.length) * 360) % 360;
            const rgb = this.hslToRgb(hue / 360, 1, 0.5);
            const r = Math.round(rgb.r * brightness);
            const g = Math.round(rgb.g * brightness);
            const b = Math.round(rgb.b * brightness);
            const color = `rgb(${r}, ${g}, ${b})`;
            led.style.backgroundColor = color;
            led.style.color = color;
        });

        const intervalId = setInterval(() => {
            hueOffset += (speed / 50) * 2;

            leds.forEach((led, i) => {
                const hue = ((i / leds.length) * 360 + hueOffset) % 360;
                const rgb = this.hslToRgb(hue / 360, 1, 0.5);
                const r = Math.round(rgb.r * brightness);
                const g = Math.round(rgb.g * brightness);
                const b = Math.round(rgb.b * brightness);
                const color = `rgb(${r}, ${g}, ${b})`;
                led.style.backgroundColor = color;
                led.style.color = color;
            });
        }, 50);
        if (container) this.trackInterval(container, intervalId);
    },

    animateFire(leds, brightness, container) {
        leds.forEach((led, i) => {
            // Set initial color immediately
            const initialR = Math.round(255 * brightness * 0.8);
            const initialG = Math.round(140 * brightness * 0.5);
            const initialColor = `rgb(${initialR}, ${initialG}, 0)`;
            led.style.backgroundColor = initialColor;
            led.style.color = initialColor;

            const intervalId = setInterval(() => {
                const heat = 0.6 + Math.random() * 0.4;
                const r = Math.round(255 * brightness * heat);
                const g = Math.round(heat > 0.7 ? 140 * brightness * (heat - 0.3) : 0);
                const b = 0;
                const color = `rgb(${r}, ${g}, ${b})`;
                led.style.backgroundColor = color;
                led.style.color = color;
            }, 50 + Math.random() * 100);
            if (container) this.trackInterval(container, intervalId);
        });
    },

    hslToRgb(h, s, l) {
        let r, g, b;

        if (s === 0) {
            r = g = b = l;
        } else {
            const hue2rgb = (p, q, t) => {
                if (t < 0) t += 1;
                if (t > 1) t -= 1;
                if (t < 1/6) return p + (q - p) * 6 * t;
                if (t < 1/2) return q;
                if (t < 2/3) return p + (q - p) * (2/3 - t) * 6;
                return p;
            };

            const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
            const p = 2 * l - q;
            r = hue2rgb(p, q, h + 1/3);
            g = hue2rgb(p, q, h);
            b = hue2rgb(p, q, h - 1/3);
        }

        return {
            r: Math.round(r * 255),
            g: Math.round(g * 255),
            b: Math.round(b * 255)
        };
    },

    /**
     * Stop all animations for a set of LEDs
     * @param {Array<HTMLElement>} leds - Array of LED elements
     */
    stop(leds) {
        // Clear all intervals
        // Note: In a production app, you'd want to track interval IDs
        // For this simple simulator, we'll just set them to black
        if (leds && leds.length) {
            leds.forEach(led => {
                led.style.backgroundColor = 'rgb(20, 20, 20)';
                led.style.color = 'rgb(20, 20, 20)';
            });
        }
    }
};
