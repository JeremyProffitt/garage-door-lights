function glowBlasterPage() {
    return {
        // Data
        conversations: [],
        glowBlasterPatterns: [],
        devices: [],
        activeConversation: null,
        currentMessages: [],
        currentLCL: '',
        currentBytecode: null,
        userMessage: '',
        isLoading: false,
        totalTokens: 0,
        selectedModel: 'claude-sonnet-4-20250514',

        // Device modal
        showDeviceModal: false,
        selectedDeviceId: '',
        selectedDevice: null,
        selectedStripPin: null,

        async init() {
            await Promise.all([
                this.loadConversations(),
                this.loadGlowBlasterPatterns(),
                this.loadDevices()
            ]);
        },

        async loadConversations() {
            try {
                const resp = await fetch('/api/glowblaster/conversations', {
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success) {
                    this.conversations = (data.data || []).sort((a, b) =>
                        new Date(b.updatedAt) - new Date(a.updatedAt)
                    );
                }
            } catch (err) {
                console.error('Failed to load conversations:', err);
            }
        },

        async loadGlowBlasterPatterns() {
            try {
                const resp = await fetch('/api/glowblaster/patterns', {
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success) {
                    this.glowBlasterPatterns = data.data || [];
                }
            } catch (err) {
                console.error('Failed to load patterns:', err);
            }
        },

        async loadDevices() {
            try {
                const resp = await fetch('/api/devices', {
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success) {
                    this.devices = (data.data || []).filter(d => d.isOnline && d.ledStrips?.length > 0);
                }
            } catch (err) {
                console.error('Failed to load devices:', err);
            }
        },

        async startNewConversation() {
            try {
                const resp = await fetch('/api/glowblaster/conversations', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify({
                        title: 'New Pattern',
                        model: this.selectedModel
                    })
                });
                const data = await resp.json();
                if (data.success) {
                    await this.loadConversations();
                    await this.loadConversation(data.data.conversationId);
                }
            } catch (err) {
                NotificationBanner.error('Failed to create conversation');
            }
        },

        async loadConversation(conversationId) {
            console.log('[LoadConversation] Loading:', conversationId);
            try {
                const resp = await fetch(`/api/glowblaster/conversations/${conversationId}`, {
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                console.log('[LoadConversation] Response:', { success: data.success, hasData: !!data.data });

                if (data.success) {
                    this.activeConversation = data.data;
                    this.currentMessages = data.data.messages || [];
                    this.currentLCL = data.data.currentLcl || '';
                    this.currentBytecode = null; // Reset bytecode so updatePreview recompiles
                    this.totalTokens = data.data.totalTokens || 0;
                    this.selectedModel = data.data.model || 'claude-sonnet-4-20250514';

                    console.log('[LoadConversation] State after load:', {
                        currentLCL: this.currentLCL ? `${this.currentLCL.substring(0, 100)}...` : '(empty)',
                        currentBytecode: this.currentBytecode
                    });

                    if (this.currentLCL) {
                        console.log('[LoadConversation] Has LCL, calling updatePreview...');
                        await this.updatePreview();
                        console.log('[LoadConversation] After updatePreview, bytecode:', this.currentBytecode ? 'SET' : 'NULL');
                    } else {
                        console.log('[LoadConversation] No LCL, clearing preview');
                        this.clearPreview();
                    }

                    this.$nextTick(() => this.scrollToBottom());
                }
            } catch (err) {
                console.error('[LoadConversation] Error:', err);
                NotificationBanner.error('Failed to load conversation');
            }
        },

        async deleteConversation(conversationId) {
            if (!confirm('Delete this conversation?')) return;

            try {
                const resp = await fetch(`/api/glowblaster/conversations/${conversationId}`, {
                    method: 'DELETE',
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success) {
                    if (this.activeConversation?.conversationId === conversationId) {
                        this.activeConversation = null;
                        this.currentMessages = [];
                        this.currentLCL = '';
                        this.clearPreview();
                    }
                    await this.loadConversations();
                    NotificationBanner.success('Conversation deleted');
                }
            } catch (err) {
                NotificationBanner.error('Failed to delete conversation');
            }
        },

        async sendMessage() {
            if (!this.userMessage.trim() || this.isLoading) return;

            // Create conversation if none active
            if (!this.activeConversation) {
                await this.startNewConversation();
                if (!this.activeConversation) return;
            }

            const message = this.userMessage.trim();
            this.userMessage = '';
            this.isLoading = true;

            // Add user message immediately for UI feedback
            this.currentMessages.push({
                role: 'user',
                content: message,
                timestamp: new Date().toISOString()
            });
            this.scrollToBottom();

            try {
                const resp = await fetch(
                    `/api/glowblaster/conversations/${this.activeConversation.conversationId}/chat`,
                    {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        credentials: 'same-origin',
                        body: JSON.stringify({
                            message,
                            model: this.selectedModel
                        })
                    }
                );
                const data = await resp.json();

                if (data.success) {
                    // Add assistant response
                    this.currentMessages.push({
                        role: 'assistant',
                        content: data.data.message,
                        timestamp: new Date().toISOString()
                    });

                    // Update LCL if provided
                    if (data.data.lcl) {
                        this.currentLCL = data.data.lcl;
                        this.currentBytecode = data.data.bytecode;
                        this.updatePreview();
                    }

                    this.totalTokens = data.data.totalTokens || this.totalTokens;

                    // Refresh conversation list (title may have changed)
                    await this.loadConversations();
                } else {
                    NotificationBanner.error('Error: ' + (data.error || 'Unknown error'));
                    // Remove the user message we added
                    this.currentMessages.pop();
                }
            } catch (err) {
                NotificationBanner.error('Failed to send message');
                // Remove the user message we added
                this.currentMessages.pop();
            } finally {
                this.isLoading = false;
                this.scrollToBottom();
            }
        },

        async updateConversationModel() {
            if (!this.activeConversation) return;
            // Model will be sent with next message
        },

        async updatePreview() {
            console.log('[UpdatePreview] Starting...', {
                hasBytecode: !!this.currentBytecode,
                hasLCL: !!this.currentLCL
            });

            const container = document.getElementById('glowblasterPreview');
            if (!container) {
                console.warn('[UpdatePreview] No preview container found!');
                return;
            }

            // Clear existing content
            container.innerHTML = '';

            if (this.currentBytecode && this.currentBytecode.length > 0) {
                console.log('[UpdatePreview] Using existing bytecode');
                // Use LCL preview interpreter
                if (typeof LCLPreview !== 'undefined') {
                    LCLPreview.render(container, this.currentBytecode, 12);
                }
            } else if (this.currentLCL) {
                console.log('[UpdatePreview] Compiling LCL:', this.currentLCL.substring(0, 100));
                // Compile LCL and render
                try {
                    const resp = await fetch('/api/glowblaster/compile', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        credentials: 'same-origin',
                        body: JSON.stringify({ lcl: this.currentLCL })
                    });
                    const data = await resp.json();
                    console.log('[UpdatePreview] Compile response:', JSON.stringify(data, null, 2));

                    if (data.success && data.data) {
                        if (data.data.success && data.data.bytecode) {
                            console.log('[UpdatePreview] Compile SUCCESS, bytecode length:', data.data.bytecode.length);
                            this.currentBytecode = data.data.bytecode;
                            if (typeof LCLPreview !== 'undefined') {
                                LCLPreview.render(container, data.data.bytecode, 12);
                            }
                        } else if (data.data.errors && data.data.errors.length > 0) {
                            console.error('[UpdatePreview] Compile FAILED with errors:', data.data.errors);
                            container.innerHTML = '<div class="compile-error">Compile error: ' + data.data.errors.join(', ') + '</div>';
                        } else {
                            console.warn('[UpdatePreview] Compile returned no bytecode and no errors:', data.data);
                        }
                    } else {
                        console.warn('[UpdatePreview] Compile response missing success or data:', data);
                    }
                } catch (err) {
                    console.error('[UpdatePreview] Fetch failed:', err);
                }
            } else {
                console.log('[UpdatePreview] No bytecode or LCL to preview');
            }
        },

        clearPreview() {
            const container = document.getElementById('glowblasterPreview');
            if (container) {
                container.innerHTML = '<div class="no-preview"><p>Pattern preview will appear here</p></div>';
            }
        },

        async saveAsPattern() {
            const name = prompt('Pattern name:', this.activeConversation?.title || 'My Pattern');
            if (!name) return;

            try {
                const resp = await fetch('/api/glowblaster/patterns', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify({
                        name,
                        conversationId: this.activeConversation?.conversationId,
                        lcl: this.currentLCL
                    })
                });
                const data = await resp.json();
                if (data.success) {
                    NotificationBanner.success('Pattern saved!');
                    await this.loadGlowBlasterPatterns();
                } else {
                    NotificationBanner.error('Error: ' + (data.error || 'Failed to save'));
                }
            } catch (err) {
                NotificationBanner.error('Failed to save pattern');
            }
        },

        async deletePattern(patternId, patternName) {
            if (!confirm(`Delete pattern "${patternName}"?`)) return;

            try {
                const resp = await fetch(`/api/glowblaster/patterns/${patternId}`, {
                    method: 'DELETE',
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success) {
                    NotificationBanner.success('Pattern deleted');
                    await this.loadGlowBlasterPatterns();
                } else {
                    NotificationBanner.error('Error: ' + (data.error || 'Failed to delete'));
                }
            } catch (err) {
                NotificationBanner.error('Failed to delete pattern');
            }
        },

        async compactConversation() {
            if (!this.activeConversation) return;
            if (!confirm('Compact conversation? This will summarize older messages to save tokens.')) return;

            try {
                const resp = await fetch(
                    `/api/glowblaster/conversations/${this.activeConversation.conversationId}/compact`,
                    {
                        method: 'POST',
                        credentials: 'same-origin'
                    }
                );
                const data = await resp.json();
                if (data.success) {
                    await this.loadConversation(this.activeConversation.conversationId);
                    NotificationBanner.success('Conversation compacted');
                }
            } catch (err) {
                NotificationBanner.error('Failed to compact conversation');
            }
        },

        loadPatternToConversation(pattern) {
            // Create new conversation with this pattern's LCL
            this.currentLCL = pattern.lclSpec || pattern.intentLayer || '';
            this.updatePreview();
            NotificationBanner.info(`Loaded pattern: ${pattern.name}`);
        },

        onDeviceSelect() {
            this.selectedDevice = this.devices.find(d => d.deviceId === this.selectedDeviceId);
            if (this.selectedDevice?.ledStrips?.length > 0) {
                this.selectedStripPin = this.selectedDevice.ledStrips[0].pin;
            }
        },

        async sendToDevice() {
            console.log('[SendToDevice] State check:', {
                selectedDeviceId: this.selectedDeviceId,
                currentBytecode: this.currentBytecode ? `${this.currentBytecode.substring(0, 20)}...` : null,
                currentLCL: this.currentLCL ? `${this.currentLCL.substring(0, 50)}...` : null,
                selectedStripPin: this.selectedStripPin
            });

            if (!this.selectedDeviceId || !this.currentBytecode) {
                console.error('[SendToDevice] FAILED - Missing:', {
                    hasDeviceId: !!this.selectedDeviceId,
                    hasBytecode: !!this.currentBytecode
                });
                NotificationBanner.error('Select a device and ensure pattern is compiled');
                return;
            }

            try {
                // Bytecode is already base64-encoded from Go backend
                const base64Bytecode = this.currentBytecode;

                const resp = await fetch('/api/particle/command', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify({
                        deviceId: this.selectedDeviceId,
                        command: 'setBytecode',
                        argument: `${this.selectedStripPin},${base64Bytecode}`
                    })
                });
                const data = await resp.json();
                if (data.success) {
                    NotificationBanner.success('Pattern sent to device!');
                    this.showDeviceModal = false;
                } else {
                    NotificationBanner.error('Failed to send: ' + (data.error || 'Unknown error'));
                }
            } catch (err) {
                NotificationBanner.error('Failed to send pattern to device');
            }
        },

        formatMessage(content) {
            // Escape HTML first
            let escaped = content
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;');

            // Convert LCL code blocks to styled HTML
            escaped = escaped.replace(/```lcl\n([\s\S]*?)```/g, '<pre class="lcl-code">$1</pre>');
            escaped = escaped.replace(/```yaml\n([\s\S]*?)```/g, '<pre class="lcl-code">$1</pre>');
            escaped = escaped.replace(/```([\s\S]*?)```/g, '<pre>$1</pre>');

            // Convert inline code
            escaped = escaped.replace(/`([^`]+)`/g, '<code>$1</code>');

            // Convert bold
            escaped = escaped.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');

            // Convert line breaks
            escaped = escaped.replace(/\n/g, '<br>');

            return escaped;
        },

        formatDate(dateStr) {
            if (!dateStr) return '';
            const date = new Date(dateStr);
            const now = new Date();
            const diff = now - date;

            if (diff < 60000) return 'Just now';
            if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
            if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
            if (diff < 604800000) return Math.floor(diff / 86400000) + 'd ago';

            return date.toLocaleDateString();
        },

        scrollToBottom() {
            this.$nextTick(() => {
                const el = this.$refs.chatMessages;
                if (el) el.scrollTop = el.scrollHeight;
            });
        }
    };
}
