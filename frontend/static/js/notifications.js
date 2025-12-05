// Notification Banner System
const NotificationBanner = {
    container: null,
    timeout: null,
    countdownInterval: null,
    countdown: 15,

    init() {
        // Create container if it doesn't exist
        if (!this.container) {
            this.container = document.createElement('div');
            this.container.id = 'notification-container';
            document.body.prepend(this.container);
        }
    },

    show(message, type = 'info', duration = 15) {
        this.init();
        this.clear();

        this.countdown = duration;

        const banner = document.createElement('div');
        banner.className = `notification-banner ${type}`;
        banner.innerHTML = `
            <div class="notification-content">
                <span>${message}</span>
            </div>
            <div class="notification-countdown">
                <span class="countdown-text">${this.countdown}s</span>
                <div class="countdown-spinner"></div>
                <button class="notification-close" onclick="NotificationBanner.dismiss()">&times;</button>
            </div>
        `;

        this.container.appendChild(banner);

        // Start countdown
        const countdownText = banner.querySelector('.countdown-text');
        this.countdownInterval = setInterval(() => {
            this.countdown--;
            if (countdownText) {
                countdownText.textContent = `${this.countdown}s`;
            }
            if (this.countdown <= 0) {
                this.dismiss();
            }
        }, 1000);

        // Auto-dismiss after duration
        this.timeout = setTimeout(() => {
            this.dismiss();
        }, duration * 1000);
    },

    dismiss() {
        if (this.countdownInterval) {
            clearInterval(this.countdownInterval);
            this.countdownInterval = null;
        }
        if (this.timeout) {
            clearTimeout(this.timeout);
            this.timeout = null;
        }

        const banner = this.container?.querySelector('.notification-banner');
        if (banner) {
            banner.classList.add('hiding');
            setTimeout(() => {
                banner.remove();
            }, 300);
        }
    },

    clear() {
        if (this.countdownInterval) {
            clearInterval(this.countdownInterval);
            this.countdownInterval = null;
        }
        if (this.timeout) {
            clearTimeout(this.timeout);
            this.timeout = null;
        }
        if (this.container) {
            this.container.innerHTML = '';
        }
    },

    success(message, duration = 15) {
        this.show(message, 'success', duration);
    },

    error(message, duration = 15) {
        this.show(message, 'error', duration);
    },

    info(message, duration = 15) {
        this.show(message, 'info', duration);
    },

    warning(message, duration = 15) {
        this.show(message, 'warning', duration);
    }
};

// Make it globally available
window.NotificationBanner = NotificationBanner;
