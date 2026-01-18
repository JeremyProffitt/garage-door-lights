function glowBlasterPage() {
    return {
        // Data
        conversations: [],
        glowBlasterPatterns: [],
        devices: [],
        activeConversation: null,
        currentMessages: [],
        currentWLED: '',      // WLED JSON state
        currentBytecode: null,
        userMessage: '',
        isLoading: false,
        totalTokens: 0,
        selectedModel: 'claude-sonnet-4-20250514',
        models: [],

        // Device modal
        showDeviceModal: false,
        selectedDeviceId: '',
        selectedDevice: null,
        selectedStripPin: null,

        // View Conversation modal
        showViewModal: false,
        conversationJson: '',

        // Debug/Prompt viewing
        lastDebugInfo: null,
        showPromptModal: false,
        promptJson: '',
        promptReadable: '',
        promptViewMode: 'readable',

        // Conversation view
        conversationReadable: '',
        conversationViewMode: 'readable',

        // Pattern editing state
        editingPatternId: null,
        editingPatternName: null,

        async init() {
            await Promise.all([
                this.loadModels(),
                this.loadConversations(),
                this.loadGlowBlasterPatterns(),
                this.loadDevices()
            ]);

            // Check for patternId URL parameter (editing existing pattern)
            const urlParams = new URLSearchParams(window.location.search);
            const patternId = urlParams.get('patternId');
            if (patternId) {
                await this.loadPatternForEditing(patternId);
                // Clean up URL
                window.history.replaceState({}, document.title, window.location.pathname);
            }
        },

        async loadModels() {
            try {
                const resp = await fetch('/api/glowblaster/models', {
                    credentials: 'same-origin'
                });
                const data = await resp.json();
                if (data.success && data.data) {
                    // Convert map to array of objects for easier iteration
                    this.models = Object.entries(data.data).map(([family, id]) => {
                        // Extract version from ID (e.g., "3-5" -> "3.5", "4" -> "4")
                        // claude-3-5-sonnet-20241022 -> 3.5
                        let version = '';
                        const match = id.match(/claude-(\d+(?:-\d+)?)/);
                        if (match) {
                            version = match[1].replace('-', '.');
                        }

                        let displayName = id;
                        if (family === 'opus') displayName = `Opus ${version} (Best)`;
                        else if (family === 'sonnet') displayName = `Sonnet ${version} (Balanced)`;
                        else if (family === 'haiku') displayName = `Haiku ${version} (Fast)`;
                        
                        return { id: id, name: displayName, family: family };
                    }).sort((a, b) => {
                        // Sort order: opus, sonnet, haiku
                        const order = { 'opus': 1, 'sonnet': 2, 'haiku': 3 };
                        return (order[a.family] || 99) - (order[b.family] || 99);
                    });
                    
                    // Set default model if current selected is not in list (or if it's the hardcoded default)
                    const sonnetModel = this.models.find(m => m.family === 'sonnet');
                    if (sonnetModel) {
                        this.selectedModel = sonnetModel.id;
                    } else if (this.models.length > 0) {
                        this.selectedModel = this.models[0].id;
                    }
                }
            } catch (err) {
                console.error('Failed to load models:', err);
                // Fallback models if API fails
                this.models = [
                    { id: 'claude-3-5-sonnet-20241022', name: 'Sonnet 3.5', family: 'sonnet' }
                ];
            }
        },

        async loadPatternForEditing(patternId) {
            // Find the pattern in our loaded patterns
            const pattern = this.glowBlasterPatterns.find(p => p.patternId === patternId);
            if (!pattern) {
                NotificationBanner.error('Pattern not found');
                return;
            }

            // Set editing state
            this.editingPatternId = patternId;
            this.editingPatternName = pattern.name;

            // Load pattern WLED data
            this.currentWLED = pattern.wledState || '';

            if (this.currentWLED) {
                await this.updatePreview();
                NotificationBanner.info(`Editing pattern: ${pattern.name}`);
            } else {
                NotificationBanner.warning('Pattern has no WLED data to edit');
            }
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
            // Clear any editing state when starting fresh
            this.clearEditingState();

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
                    this.currentWLED = data.data.currentWled || '';
                    this.currentBytecode = null; // Reset bytecode so updatePreview recompiles
                    this.totalTokens = data.data.totalTokens || 0;
                    this.selectedModel = data.data.model || 'claude-sonnet-4-20250514';

                    console.log('[LoadConversation] State after load:', {
                        currentWLED: this.currentWLED ? `${this.currentWLED.substring(0, 100)}...` : '(empty)',
                        currentBytecode: this.currentBytecode
                    });

                    if (this.currentWLED) {
                        console.log('[LoadConversation] Has WLED JSON, calling updatePreview...');
                        await this.updatePreview();
                        console.log('[LoadConversation] After updatePreview, bytecode:', this.currentBytecode ? 'SET' : 'NULL');
                    } else {
                        console.log('[LoadConversation] No WLED data, clearing preview');
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
                        this.currentWLED = '';
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

                    // Update WLED JSON if provided
                    if (data.data.wled) {
                        this.currentWLED = data.data.wled;
                        this.currentBytecode = data.data.wledBinary || data.data.bytecode;
                        this.updatePreview();
                    }

                    this.totalTokens = data.data.totalTokens || this.totalTokens;

                    // Store debug info
                    if (data.data.debug) {
                        this.lastDebugInfo = data.data.debug;
                    }

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
                hasWLED: !!this.currentWLED
            });

            const container = document.getElementById('glowblasterPreview');
            if (!container) {
                console.warn('[UpdatePreview] No preview container found!');
                return;
            }

            // Clear existing content
            container.innerHTML = '';

            // If we have bytecode, use WLEDPreview to render
            if (this.currentBytecode && this.currentBytecode.length > 0) {
                console.log('[UpdatePreview] Using existing bytecode');
                if (typeof WLEDPreview !== 'undefined') {
                    WLEDPreview.render(container, this.currentBytecode, 12);
                }
            }
            // WLED JSON - compile to binary first
            else if (this.currentWLED) {
                console.log('[UpdatePreview] Compiling WLED JSON:', this.currentWLED.substring(0, 100));
                try {
                    const resp = await fetch('/api/glowblaster/compile', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        credentials: 'same-origin',
                        body: JSON.stringify({ lcl: this.currentWLED })
                    });
                    const data = await resp.json();
                    console.log('[UpdatePreview] WLED Compile response:', JSON.stringify(data, null, 2));

                    if (data.success && data.data) {
                        if (data.data.success && data.data.bytecode) {
                            console.log('[UpdatePreview] WLED Compile SUCCESS, bytecode length:', data.data.bytecode.length);
                            this.currentBytecode = data.data.bytecode;
                            if (typeof WLEDPreview !== 'undefined') {
                                WLEDPreview.render(container, data.data.bytecode, 12);
                            }
                        } else if (data.data.errors && data.data.errors.length > 0) {
                            console.error('[UpdatePreview] WLED Compile FAILED with errors:', data.data.errors);
                            container.innerHTML = '<div class="compile-error">Compile error: ' + data.data.errors.join(', ') + '</div>';
                        } else {
                            console.warn('[UpdatePreview] WLED Compile returned no bytecode and no errors:', data.data);
                        }
                    } else {
                        console.warn('[UpdatePreview] WLED Compile response missing success or data:', data);
                    }
                } catch (err) {
                    console.error('[UpdatePreview] WLED Compile fetch failed:', err);
                }
            } else {
                console.log('[UpdatePreview] No bytecode or WLED to preview');
            }
        },

        clearPreview() {
            const container = document.getElementById('glowblasterPreview');
            if (container) {
                container.innerHTML = '<div class="no-preview"><p>Pattern preview will appear here</p></div>';
            }
        },

        async saveAsNewPattern() {
            const name = prompt('Pattern name:', this.activeConversation?.title || 'My Pattern');
            if (!name) return;

            try {
                // Build request body with WLED data
                const body = {
                    name,
                    conversationId: this.activeConversation?.conversationId
                };
                if (this.currentWLED) {
                    body.wled = this.currentWLED;
                }

                const resp = await fetch('/api/glowblaster/patterns', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify(body)
                });
                const data = await resp.json();
                if (data.success) {
                    // Set editing state to the new pattern
                    this.editingPatternId = data.data?.patternId;
                    this.editingPatternName = name;
                    NotificationBanner.success('Pattern saved!');
                    await this.loadGlowBlasterPatterns();
                } else {
                    NotificationBanner.error('Error: ' + (data.error || 'Failed to save'));
                }
            } catch (err) {
                NotificationBanner.error('Failed to save pattern');
            }
        },

        async updateExistingPattern() {
            if (!this.editingPatternId) return;

            try {
                // Build request body with WLED data
                const body = {};
                if (this.currentWLED) {
                    body.wled = this.currentWLED;
                }

                const resp = await fetch(`/api/glowblaster/patterns/${this.editingPatternId}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify(body)
                });
                const data = await resp.json();
                if (data.success) {
                    NotificationBanner.success(`Pattern "${this.editingPatternName}" updated!`);
                    await this.loadGlowBlasterPatterns();
                } else {
                    NotificationBanner.error('Error: ' + (data.error || 'Failed to update'));
                }
            } catch (err) {
                NotificationBanner.error('Failed to update pattern');
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

        async loadPatternToConversation(pattern) {
            // Set editing state for this pattern
            this.editingPatternId = pattern.patternId;
            this.editingPatternName = pattern.name;

            // Check if pattern has an associated conversation
            if (pattern.conversationId) {
                // Try to load the existing conversation
                try {
                    const resp = await fetch(`/api/glowblaster/conversations/${pattern.conversationId}`, {
                        credentials: 'same-origin'
                    });
                    const data = await resp.json();

                    if (data.success && data.data) {
                        // Conversation exists, load it
                        this.activeConversation = data.data;
                        this.currentMessages = data.data.messages || [];
                        this.currentWLED = data.data.currentWled || pattern.wledState || '';
                        this.currentBytecode = null;
                        this.totalTokens = data.data.totalTokens || 0;
                        this.selectedModel = data.data.model || 'claude-sonnet-4-20250514';

                        if (this.currentWLED) {
                            await this.updatePreview();
                        }

                        NotificationBanner.info(`Loaded conversation for: ${pattern.name}`);
                        this.$nextTick(() => this.scrollToBottom());
                        return;
                    }
                } catch (err) {
                    console.log('Conversation not found, will create new one');
                }
            }

            // No conversation found - create a new one with the pattern loaded
            await this.startNewConversation();

            // Re-set editing state (startNewConversation clears it)
            this.editingPatternId = pattern.patternId;
            this.editingPatternName = pattern.name;

            // Load the pattern WLED data
            this.currentWLED = pattern.wledState || '';

            if (this.currentWLED) {
                await this.updatePreview();

                // Add a system message showing the loaded WLED pattern
                this.currentMessages.push({
                    role: 'assistant',
                    content: `I've loaded the pattern "${pattern.name}" for editing. Here's the current WLED configuration:\n\n\`\`\`json\n${this.currentWLED}\n\`\`\`\n\nHow would you like to modify it?`,
                    timestamp: new Date().toISOString()
                });
                this.scrollToBottom();
            }

            NotificationBanner.info(`Loaded pattern: ${pattern.name}`);
        },

        clearEditingState() {
            this.editingPatternId = null;
            this.editingPatternName = null;
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
                currentWLED: this.currentWLED ? `${this.currentWLED.substring(0, 50)}...` : null,
                selectedStripPin: this.selectedStripPin
            });

            if (!this.selectedDeviceId || (!this.currentBytecode && !this.currentWLED)) {
                console.error('[SendToDevice] FAILED - Missing:', {
                    hasDeviceId: !!this.selectedDeviceId,
                    hasBytecode: !!this.currentBytecode,
                    hasWLED: !!this.currentWLED
                });
                NotificationBanner.error('Select a device and ensure pattern is compiled');
                return;
            }

            try {
                // Get the selected strip's LED count
                const selectedStrip = this.selectedDevice?.ledStrips?.find(s => s.pin === this.selectedStripPin);
                const ledCount = selectedStrip?.ledCount || 8;
                console.log('[SendToDevice] Selected strip LED count:', ledCount);

                let bytecodeToSend = this.currentBytecode;

                // If we have WLED JSON, recompile with the correct LED count
                if (this.currentWLED) {
                    console.log('[SendToDevice] Recompiling WLED JSON with ledCount:', ledCount);
                    try {
                        const wledJson = JSON.parse(this.currentWLED);
                        // Update all segment stop values to match device LED count
                        if (wledJson.seg && Array.isArray(wledJson.seg)) {
                            wledJson.seg.forEach(seg => {
                                seg.stop = ledCount;
                            });
                        }
                        const updatedWledState = JSON.stringify(wledJson);
                        console.log('[SendToDevice] Updated WLED JSON:', updatedWledState);

                        const compileResp = await fetch('/api/glowblaster/compile', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            credentials: 'same-origin',
                            body: JSON.stringify({ lcl: updatedWledState })
                        });
                        const compileData = await compileResp.json();
                        console.log('[SendToDevice] Compile response:', compileData);

                        if (compileData.success && compileData.data?.bytecode) {
                            bytecodeToSend = compileData.data.bytecode;
                            console.log('[SendToDevice] Using recompiled bytecode');
                        } else {
                            console.error('[SendToDevice] Compile failed:', compileData.data?.errors);
                            NotificationBanner.error('Failed to compile pattern: ' + (compileData.data?.errors?.join(', ') || 'Unknown error'));
                            return;
                        }
                    } catch (parseErr) {
                        console.error('[SendToDevice] Failed to parse/update WLED JSON:', parseErr);
                        // Fall through to use existing bytecode
                    }
                }

                if (!bytecodeToSend) {
                    NotificationBanner.error('No bytecode available to send');
                    return;
                }

                const resp = await fetch('/api/particle/command', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify({
                        deviceId: this.selectedDeviceId,
                        command: 'setBytecode',
                        argument: `${this.selectedStripPin},${bytecodeToSend}`
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

            // Convert WLED JSON code blocks to styled HTML
            escaped = escaped.replace(/```json\n([\s\S]*?)```/g, '<pre class="wled-code">$1</pre>');
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

        viewConversation() {
            if (!this.activeConversation) return;
            // Prepare JSON
            const exportData = {
                conversationId: this.activeConversation.conversationId,
                title: this.activeConversation.title,
                model: this.activeConversation.model,
                createdAt: this.activeConversation.createdAt,
                updatedAt: this.activeConversation.updatedAt,
                messages: this.currentMessages
            };
            this.conversationJson = JSON.stringify(exportData, null, 2);
            this.conversationReadable = this.formatConversationReadable(exportData);
            this.conversationViewMode = 'readable';
            this.showViewModal = true;
        },

        formatConversationReadable(data) {
            let html = '';
            html += `<div style="margin-bottom: 1rem; padding-bottom: 1rem; border-bottom: 1px solid #e5e7eb;">`;
            html += `<strong>Title:</strong> ${this.escapeHtml(data.title || 'Untitled')}<br>`;
            html += `<strong>Model:</strong> ${this.escapeHtml(data.model || 'Unknown')}<br>`;
            html += `<strong>Created:</strong> ${data.createdAt ? new Date(data.createdAt).toLocaleString() : 'Unknown'}<br>`;
            html += `<strong>Updated:</strong> ${data.updatedAt ? new Date(data.updatedAt).toLocaleString() : 'Unknown'}<br>`;
            html += `<strong>ID:</strong> <code style="font-size: 0.8rem;">${data.conversationId || ''}</code>`;
            html += `</div>`;

            if (data.messages && data.messages.length > 0) {
                html += `<div style="margin-bottom: 0.5rem;"><strong>Messages (${data.messages.length}):</strong></div>`;
                data.messages.forEach((msg, idx) => {
                    const isUser = msg.role === 'user';
                    const bgColor = isUser ? '#dbeafe' : '#f3f4f6';
                    const label = isUser ? 'User' : 'Assistant';
                    html += `<div style="margin-bottom: 0.75rem; padding: 0.75rem; background: ${bgColor}; border-radius: 8px;">`;
                    html += `<div style="font-weight: 600; font-size: 0.8rem; color: #6b7280; margin-bottom: 0.25rem;">${label}</div>`;
                    html += `<div style="white-space: pre-wrap;">${this.escapeHtml(msg.content)}</div>`;
                    html += `</div>`;
                });
            } else {
                html += `<div style="color: #9ca3af;">No messages</div>`;
            }
            return html;
        },

        getConversationReadableText() {
            if (!this.activeConversation) return '';
            const data = {
                conversationId: this.activeConversation.conversationId,
                title: this.activeConversation.title,
                model: this.activeConversation.model,
                createdAt: this.activeConversation.createdAt,
                updatedAt: this.activeConversation.updatedAt,
                messages: this.currentMessages
            };
            let text = '';
            text += `Title: ${data.title || 'Untitled'}\n`;
            text += `Model: ${data.model || 'Unknown'}\n`;
            text += `Created: ${data.createdAt ? new Date(data.createdAt).toLocaleString() : 'Unknown'}\n`;
            text += `Updated: ${data.updatedAt ? new Date(data.updatedAt).toLocaleString() : 'Unknown'}\n`;
            text += `ID: ${data.conversationId || ''}\n`;
            text += `\n${'='.repeat(50)}\n\n`;

            if (data.messages && data.messages.length > 0) {
                data.messages.forEach((msg, idx) => {
                    const label = msg.role === 'user' ? 'USER' : 'ASSISTANT';
                    text += `[${label}]\n${msg.content}\n\n`;
                });
            }
            return text;
        },

        copyCurrentConversationView() {
            const text = this.conversationViewMode === 'raw'
                ? this.conversationJson
                : this.getConversationReadableText();
            this.copyToClipboard(text);
        },

        downloadCurrentConversationView() {
            if (!this.activeConversation) return;
            const isRaw = this.conversationViewMode === 'raw';
            const content = isRaw ? this.conversationJson : this.getConversationReadableText();
            const ext = isRaw ? 'json' : 'txt';
            const mimeType = isRaw ? 'application/json' : 'text/plain';

            const blob = new Blob([content], { type: mimeType });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `glowblaster-conversation-${this.activeConversation.conversationId.substring(0, 8)}.${ext}`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        },

        viewPrompt() {
            if (!this.lastDebugInfo) {
                NotificationBanner.info('No prompt data available. Send a message first.');
                return;
            }
            this.promptJson = JSON.stringify(this.lastDebugInfo, null, 2);
            this.promptReadable = this.formatPromptReadable(this.lastDebugInfo);
            this.promptViewMode = 'readable';
            this.showPromptModal = true;
        },

        formatPromptReadable(data) {
            let html = '';

            // System prompt section
            if (data.systemPrompt) {
                html += `<div style="margin-bottom: 1rem;">`;
                html += `<div style="font-weight: 600; color: #059669; margin-bottom: 0.5rem; font-size: 1rem;">SYSTEM PROMPT</div>`;
                html += `<div style="background: #ecfdf5; padding: 1rem; border-radius: 8px; border-left: 4px solid #059669; white-space: pre-wrap; font-family: system-ui;">${this.escapeHtml(data.systemPrompt)}</div>`;
                html += `</div>`;
            }

            // Messages section
            if (data.messages && data.messages.length > 0) {
                html += `<div style="font-weight: 600; color: #374151; margin-bottom: 0.5rem; font-size: 1rem;">MESSAGES (${data.messages.length})</div>`;
                data.messages.forEach((msg, idx) => {
                    const isUser = msg.role === 'user';
                    const bgColor = isUser ? '#dbeafe' : '#f3f4f6';
                    const borderColor = isUser ? '#3b82f6' : '#9ca3af';
                    const label = isUser ? 'USER' : 'ASSISTANT';
                    html += `<div style="margin-bottom: 0.75rem; padding: 0.75rem; background: ${bgColor}; border-radius: 8px; border-left: 4px solid ${borderColor};">`;
                    html += `<div style="font-weight: 600; font-size: 0.8rem; color: #6b7280; margin-bottom: 0.25rem;">${label}</div>`;
                    html += `<div style="white-space: pre-wrap;">${this.escapeHtml(msg.content)}</div>`;
                    html += `</div>`;
                });
            }

            return html;
        },

        getPromptReadableText() {
            if (!this.lastDebugInfo) return '';
            const data = this.lastDebugInfo;
            let text = '';

            if (data.systemPrompt) {
                text += `${'='.repeat(50)}\nSYSTEM PROMPT\n${'='.repeat(50)}\n\n`;
                text += data.systemPrompt;
                text += `\n\n`;
            }

            if (data.messages && data.messages.length > 0) {
                text += `${'='.repeat(50)}\nMESSAGES (${data.messages.length})\n${'='.repeat(50)}\n\n`;
                data.messages.forEach((msg, idx) => {
                    const label = msg.role === 'user' ? 'USER' : 'ASSISTANT';
                    text += `[${label}]\n${msg.content}\n\n`;
                });
            }

            return text;
        },

        copyCurrentPromptView() {
            const text = this.promptViewMode === 'raw'
                ? this.promptJson
                : this.getPromptReadableText();
            this.copyToClipboard(text);
        },

        downloadCurrentPromptView() {
            const isRaw = this.promptViewMode === 'raw';
            const content = isRaw ? this.promptJson : this.getPromptReadableText();
            const ext = isRaw ? 'json' : 'txt';
            const mimeType = isRaw ? 'application/json' : 'text/plain';

            const blob = new Blob([content], { type: mimeType });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `glowblaster-prompt-${new Date().toISOString().slice(0, 10)}.${ext}`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        },

        escapeHtml(text) {
            if (!text) return '';
            return text
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#039;');
        },

        formatWLEDDisplay(wledJson) {
            if (!wledJson) return '';
            try {
                // Parse and pretty print with 2-space indent
                const parsed = JSON.parse(wledJson);
                return JSON.stringify(parsed, null, 2);
            } catch (e) {
                // If it's already a string but not valid JSON, return as-is
                return wledJson;
            }
        },

        copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(() => {
                NotificationBanner.success('Copied to clipboard');
            }).catch(err => {
                console.error('Failed to copy:', err);
                NotificationBanner.error('Failed to copy');
            });
        },

        scrollToBottom() {
            this.$nextTick(() => {
                const el = this.$refs.chatMessages;
                if (el) el.scrollTop = el.scrollHeight;
            });
        }
    };
}
