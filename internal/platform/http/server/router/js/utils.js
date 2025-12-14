// theme management -----------------------------------------------------------

const LIGHT_THEME = 'nord';
const DARK_THEME = 'night';
const THEME_KEY = 'HALSEY_THEME';

(function () {
    // Get current theme, defaulting to system preference
    window.getTheme = function () {
        return localStorage.getItem(THEME_KEY) ||
            (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? DARK_THEME : LIGHT_THEME);
    };

    // Check if current theme is dark
    window.isDarkTheme = function () {
        return getTheme() === DARK_THEME;
    };

    // Set theme and update UI
    window.setTheme = function (theme) {
        localStorage.setItem(THEME_KEY, theme);
        document.documentElement.setAttribute('data-theme', theme);
        updateThemeToggle();
    };

    // Toggle between light and dark themes
    window.toggleTheme = function () {
        setTheme(isDarkTheme() ? LIGHT_THEME : DARK_THEME);
    };

    // Update the theme toggle checkbox state
    function updateThemeToggle() {
        const toggle = document.getElementById('theme-toggle');
        if (toggle) {
            toggle.checked = isDarkTheme();
        }
    }

    // Apply theme immediately on page load (before DOM ready)
    const loadTheme = getTheme();
    document.documentElement.setAttribute('data-theme', loadTheme);
    localStorage.setItem(THEME_KEY, loadTheme);

    // Setup toggle after DOM is loaded
    document.addEventListener('DOMContentLoaded', function () {
        updateThemeToggle();
    });
})();

// click blocker --------------------------------------------------------------

(function () {
    window.blockClicks = function () {
        const blocker = document.getElementById('click-blocker');
        if (blocker) blocker.classList.remove('hidden');
    };

    window.unblockClicks = function () {
        const blocker = document.getElementById('click-blocker');
        if (blocker) blocker.classList.add('hidden');
    };
})();

// settings -------------------------------------------------------------------

function openBackupsModal() {
    const modal = document.getElementById('backups-modal');
    const loading = document.getElementById('backups-loading');
    const content = document.getElementById('backups-content');
    const empty = document.getElementById('backups-empty');
    const error = document.getElementById('backups-error');
    const errorMessage = document.getElementById('backups-error-message');

    // Reset state
    loading.classList.remove('hidden');
    content.classList.add('hidden');
    empty.classList.add('hidden');
    error.classList.add('hidden');
    content.innerHTML = '';

    // Show modal
    modal.showModal();

    // Fetch backups
    fetch('/settings/backups')
        .then(res => {
            if (!res.ok) throw new Error(`HTTP ${res.status}`);
            return res.json();
        })
        .then(backups => {
            loading.classList.add('hidden');

            if (!backups || backups.length === 0) {
                empty.classList.remove('hidden');
                return;
            }

            // Build backup items
            backups.forEach(backup => {
                const item = document.createElement('div');
                item.className = 'flex items-center justify-between bg-base-200/50 rounded-lg p-3';

                const info = document.createElement('div');
                info.className = 'flex-1';

                const name = document.createElement('div');
                name.className = 'font-medium text-base-content';
                name.textContent = backup.guildName;

                const lastRun = document.createElement('div');
                lastRun.className = 'text-xs text-base-content/50';
                if (backup.lastRun) {
                    const date = new Date(backup.lastRun);
                    lastRun.textContent = 'Last backup: ' + date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
                } else {
                    lastRun.textContent = 'No backup run yet';
                }

                info.appendChild(name);
                info.appendChild(lastRun);

                const downloadBtn = document.createElement('a');

                if (backup.lastRun) {
                    downloadBtn.className = 'btn btn-sm btn-primary';
                    downloadBtn.href = backup.downloadLink;
                } else {
                    downloadBtn.className = 'btn btn-sm btn-primary btn-disabled';
                }
                downloadBtn.innerHTML = `
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                    </svg>
                    Download
                `;

                item.appendChild(info);
                item.appendChild(downloadBtn);
                content.appendChild(item);
            });

            content.classList.remove('hidden');
        })
        .catch(err => {
            loading.classList.add('hidden');
            error.classList.remove('hidden');
            errorMessage.textContent = err.message || 'Failed to load backups.';
        });
}

function stopServer() {
    if (!confirm('Are you sure you want to stop the server? You will lose access to this page.')) {
        return;
    }
    blockClicks();
    fetch('/settings/stop', { method: 'POST' })
        .then(response => {
            if (response.ok) {
                alert('Server is shutting down...');
            } else {
                throw new Error('Failed to stop server');
            }
        })
        .catch(err => {
            unblockClicks();
            alert('Error: ' + err.message);
        });
}

function restartServer() {
    const registerCommands = document.getElementById('restart-register-commands').checked;
    const updateRequested = document.getElementById('restart-update').checked;

    // Close the modal
    document.getElementById('restart-modal').close();

    blockClicks();
    fetch('/settings/restart', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            register_commands: registerCommands,
            update: updateRequested
        })
    })
        .then(response => {
            if (response.ok || response.status === 202) {
                // Server is restarting, poll for it to come back
                setTimeout(() => pollForRestart(updateRequested), 3000);
            } else {
                throw new Error('Failed to restart server');
            }
        })
        .catch(err => {
            unblockClicks();
            alert('Error: ' + err.message);
        });
}

function pollForRestart(updateRequested = false) {
    const startTime = Date.now();
    const pollInterval = 3000;
    const timeout = 300000; // 5 minutes

    const check = () => {
        if (Date.now() - startTime > timeout) {
            unblockClicks();
            alert('Restart timed out. Please check logs or try again.');
            return;
        }

        fetch('/settings/restart-status')
            .then(res => res.json())
            .then(data => {
                if (data.restarted) {
                    if (updateRequested && !data.updated) {
                        unblockClicks();
                        alert('Restart completed, but the update did not apply. You may already be on the latest version, or the update failed.');
                    } else {
                        window.location.reload();
                    }
                } else {
                    setTimeout(check, pollInterval);
                }
            })
            .catch(() => {
                // Network error during polling - server might be restarting
                setTimeout(check, pollInterval);
            });
    };

    check();
}

// settings status helpers ----------------------------------------------------

(function () {
    // Show a loading spinner on the status element
    window.showPending = function (statusEl) {
        if (!statusEl) return;
        statusEl.className = 'status loading loading-spinner loading-xs';
        statusEl.textContent = '';
        statusEl.dataset.errorMessage = '';
        statusEl.onclick = null;
    };

    // Show a green circle that auto-hides after 2 seconds
    window.showSuccess = function (statusEl) {
        if (!statusEl) return;
        statusEl.className = 'status status-success';
        statusEl.dataset.errorMessage = '';
        statusEl.onclick = null;
        setTimeout(() => {
            if (statusEl.classList.contains('status-success')) {
                statusEl.className = 'status hidden';
            }
        }, 2000);
    };

    // Show a red circle that opens error modal when clicked
    window.showError = function (statusEl, message) {
        if (!statusEl) return;
        statusEl.className = 'status status-error cursor-pointer';
        statusEl.dataset.errorMessage = message || 'An unknown error occurred.';
        statusEl.onclick = () => {
            const modal = document.getElementById('error-modal');
            const msgEl = document.getElementById('error-modal-message');
            if (modal && msgEl) {
                msgEl.textContent = statusEl.dataset.errorMessage;
                modal.showModal();
            }
        };
    };

    // Find the status element relative to the input
    function findStatus(input) {
        // For inline toggles (inside label), find sibling status span
        const label = input.closest('label');
        if (label) {
            const status = label.querySelector('.status');
            if (status) return status;
        }
        // For inputs with wrapper divs, find sibling
        const wrapper = input.closest('.flex');
        if (wrapper) {
            const status = wrapper.querySelector('.status');
            if (status) return status;
        }
        // Fallback: search in parent form-control
        const formControl = input.closest('.form-control');
        if (formControl) {
            return formControl.querySelector('.status');
        }
        return null;
    }

    // Generic handler for toggles/checkboxes (immediate POST on change)
    window.handleToggle = function (inputId, endpoint, fieldName) {
        const input = document.getElementById(inputId);
        if (!input) return;
        const status = findStatus(input);

        input.addEventListener('change', async () => {
            showPending(status);
            try {
                const body = {};
                // Support nested fields like "autoExpand.reddit"
                const parts = fieldName.split('.');
                if (parts.length === 2) {
                    body[parts[0]] = { [parts[1]]: input.checked };
                } else {
                    body[fieldName] = input.checked;
                }

                const res = await fetch(endpoint, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text || `HTTP ${res.status}`);
                }
                showSuccess(status);
            } catch (e) {
                showError(status, e.message);
            }
        });
    };

    // Generic handler for select dropdowns (immediate POST on change)
    window.handleSelect = function (inputId, endpoint, fieldName, onSuccess) {
        const input = document.getElementById(inputId);
        if (!input) return;
        const status = findStatus(input);

        input.addEventListener('change', async () => {
            showPending(status);
            try {
                const res = await fetch(endpoint, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ [fieldName]: input.value })
                });
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text || `HTTP ${res.status}`);
                }
                showSuccess(status);
                if (onSuccess) onSuccess();
            } catch (e) {
                showError(status, e.message);
            }
        });
    };

    // Generic handler for text/number inputs with debouncing
    window.handleTextInput = function (inputId, endpoint, fieldName, debounceMs = 500, opts = {}) {
        const input = document.getElementById(inputId);
        if (!input) return;
        const status = findStatus(input);

        let timeout = null;
        let controller = null;

        input.addEventListener('input', () => {
            clearTimeout(timeout);
            if (controller) controller.abort();

            timeout = setTimeout(async () => {
                // Skip empty values for optional fields like bot token
                if (opts.skipEmpty && !input.value.trim()) return;

                controller = new AbortController();
                showPending(status);
                try {
                    let value = input.value;
                    // Parse as int for number inputs
                    if (input.type === 'number') {
                        value = parseInt(value, 10);
                        if (isNaN(value)) {
                            throw new Error('Invalid number');
                        }
                    }

                    const res = await fetch(endpoint, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ [fieldName]: value }),
                        signal: controller.signal
                    });
                    if (!res.ok) {
                        const text = await res.text();
                        throw new Error(text || `HTTP ${res.status}`);
                    }
                    showSuccess(status);
                    if (opts.onSuccess) opts.onSuccess();
                } catch (e) {
                    if (e.name !== 'AbortError') {
                        showError(status, e.message);
                    }
                }
            }, debounceMs);
        });
    };

    // Show restart required notice
    function showRestartNotice() {
        const notice = document.getElementById('restart-required-notice');
        if (notice) notice.classList.remove('hidden');
    }

    // Wire up all settings on DOMContentLoaded
    document.addEventListener('DOMContentLoaded', function () {
        // User settings (Settings tab)
        handleToggle('backup-opt-out', '/settings/user', 'backupOptOut');
        handleToggle('ai-chat-opt-out', '/settings/user', 'aiChatOptOut');
        handleToggle('auto-expand-reddit', '/settings/user', 'autoExpand.reddit');
        handleToggle('auto-expand-youtube-shorts', '/settings/user', 'autoExpand.youTubeShorts');
        handleToggle('auto-expand-redgifs', '/settings/user', 'autoExpand.redGifs');

        // Admin settings (Admin tab) - these show restart notice on change
        handleSelect('admin-log-level', '/settings/admin', 'logLevel', showRestartNotice);
        handleTextInput('admin-host', '/settings/admin', 'host', 500, { onSuccess: showRestartNotice });
        handleTextInput('admin-port', '/settings/admin', 'port', 500, { onSuccess: showRestartNotice });
        handleTextInput('admin-proxy-port', '/settings/admin', 'proxyPort', 500, { onSuccess: showRestartNotice });
        handleTextInput('admin-bot-token', '/settings/admin', 'botToken', 500, { skipEmpty: true, onSuccess: showRestartNotice });
        handleTextInput('admin-system-prompt', '/settings/admin', 'systemPrompt', 500, { skipEmpty: true, onSuccess: showRestartNotice });

        // Guild settings - wire up dynamically found guilds
        wireGuildSettings();
    });

    // Wire up guild-specific settings
    function wireGuildSettings() {
        // Find all guild collapses
        document.querySelectorAll('[data-guild-id]').forEach(collapse => {
            const guildId = collapse.dataset.guildId;
            if (!guildId) return;

            const endpoint = `/settings/guild/${guildId}`;

            // Backup Password
            handleTextInputById(`guild-${guildId}-backup-password`, endpoint, 'backupPassword', 500, { skipEmpty: true });

            // Synctube URL
            handleTextInputById(`guild-${guildId}-synctube`, endpoint, 'synctubeURL', 500);

            // System Prompt
            handleTextInputById(`guild-${guildId}-system-prompt`, endpoint, 'systemPrompt', 500);

            // Toggle settings
            handleToggleById(`guild-${guildId}-backup`, endpoint, 'backupEnabled');
            handleToggleById(`guild-${guildId}-antirot`, endpoint, 'antiRotEnabled');
            handleToggleById(`guild-${guildId}-aichat`, endpoint, 'aiChatEnabled');
        });

        // Wire up channel backup checkboxes
        document.querySelectorAll('.channel-backup').forEach(checkbox => {
            const channelId = checkbox.dataset.channelId;
            if (!channelId) return;

            const status = findStatus(checkbox);
            checkbox.addEventListener('change', async () => {
                showPending(status);
                try {
                    const res = await fetch(`/settings/channel/${channelId}`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ backupEnabled: checkbox.checked })
                    });
                    if (!res.ok) {
                        const text = await res.text();
                        throw new Error(text || `HTTP ${res.status}`);
                    }
                    showSuccess(status);
                } catch (e) {
                    showError(status, e.message);
                }
            });
        });

        // Wire up fav channel radio buttons
        document.querySelectorAll('.guild-fav-channel').forEach(radio => {
            radio.addEventListener('change', async () => {
                const guildId = radio.dataset.guildId;
                const channelId = radio.value;
                try {
                    const res = await fetch(`/settings/guild/${guildId}`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ favChannelID: channelId })
                    });
                    if (!res.ok) {
                        throw new Error(await res.text() || `HTTP ${res.status}`);
                    }
                } catch (e) {
                    console.error('Failed to update fav channel:', e);
                }
            });
        });

        // Wire up bot channel radio buttons
        document.querySelectorAll('.guild-bot-channel').forEach(radio => {
            radio.addEventListener('change', async () => {
                const guildId = radio.dataset.guildId;
                const channelId = radio.value;
                try {
                    const res = await fetch(`/settings/guild/${guildId}`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ botChannelID: channelId })
                    });
                    if (!res.ok) {
                        throw new Error(await res.text() || `HTTP ${res.status}`);
                    }
                } catch (e) {
                    console.error('Failed to update bot channel:', e);
                }
            });
        });
    }

    // Handle toggle by ID (for dynamically generated IDs)
    function handleToggleById(inputId, endpoint, fieldName) {
        const input = document.getElementById(inputId);
        if (!input) return;
        const status = findStatus(input);

        input.addEventListener('change', async () => {
            showPending(status);
            try {
                const res = await fetch(endpoint, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ [fieldName]: input.checked })
                });
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text || `HTTP ${res.status}`);
                }
                showSuccess(status);
            } catch (e) {
                showError(status, e.message);
            }
        });
    }

    // Handle text input by ID (for dynamically generated IDs)
    function handleTextInputById(inputId, endpoint, fieldName, debounceMs = 500) {
        const input = document.getElementById(inputId);
        if (!input) return;
        const status = findStatus(input);

        let timeout = null;
        let controller = null;

        input.addEventListener('input', () => {
            clearTimeout(timeout);
            if (controller) controller.abort();

            timeout = setTimeout(async () => {
                controller = new AbortController();
                showPending(status);
                try {
                    const res = await fetch(endpoint, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ [fieldName]: input.value }),
                        signal: controller.signal
                    });
                    if (!res.ok) {
                        const text = await res.text();
                        throw new Error(text || `HTTP ${res.status}`);
                    }
                    showSuccess(status);
                } catch (e) {
                    if (e.name !== 'AbortError') {
                        showError(status, e.message);
                    }
                }
            }, debounceMs);
        });
    }
})();

