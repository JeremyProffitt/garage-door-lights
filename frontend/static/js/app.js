// Global app utilities

// Format dates
function formatDate(dateString) {
    return new Date(dateString).toLocaleString();
}

// Show toast notifications
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);

    setTimeout(() => {
        toast.remove();
    }, 3000);
}

// Get token from cookie
function getTokenFromCookie() {
    const match = document.cookie.match(/(?:^|;\s*)token=([^;]*)/);
    return match ? match[1] : null;
}

// Handle fetch errors
async function handleFetch(url, options = {}) {
    try {
        // Add Authorization header if token exists
        const token = getTokenFromCookie();
        if (token && !options.headers) {
            options.headers = {};
        }
        if (token) {
            options.headers['Authorization'] = `Bearer ${token}`;
        }

        // Ensure Content-Type is set for POST/PUT requests
        if ((options.method === 'POST' || options.method === 'PUT') && !options.headers['Content-Type']) {
            options.headers['Content-Type'] = 'application/json';
        }

        const response = await fetch(url, options);
        const data = await response.json();

        if (!data.success) {
            throw new Error(data.error || 'Request failed');
        }

        return data;
    } catch (error) {
        showToast(error.message, 'error');
        throw error;
    }
}
