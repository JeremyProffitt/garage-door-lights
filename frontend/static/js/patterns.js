function patternsPage() {
    return {
        patterns: [],
        showCreateModal: false,
        editingPattern: null,
        form: {
            name: '',
            description: '',
            type: 'candle',
            red: 255,
            green: 100,
            blue: 0,
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
                this.patterns = data.data || [];
            }
        },

        editPattern(pattern) {
            this.editingPattern = pattern;
            this.form = {
                name: pattern.name,
                description: pattern.description,
                type: pattern.type,
                red: pattern.red,
                green: pattern.green,
                blue: pattern.blue,
                brightness: pattern.brightness,
                speed: pattern.speed
            };
            this.showCreateModal = true;
        },

        async savePattern() {
            const method = this.editingPattern ? 'PUT' : 'POST';
            const url = this.editingPattern
                ? `/api/patterns/${this.editingPattern.patternId}`
                : '/api/patterns';

            const resp = await fetch(url, {
                method,
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(this.form)
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
                red: 255,
                green: 100,
                blue: 0,
                brightness: 128,
                speed: 50
            };
        }
    }
}
