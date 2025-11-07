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

// Handle fetch errors
async function handleFetch(url, options = {}) {
    try {
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
