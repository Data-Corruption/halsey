// Server Actions
// Backup modal, stop, restart, and polling functionality

import { blockClicks, unblockClicks } from './ui.js';

/** Open and populate the backups modal */
export function openBackupsModal() {
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

/** Stop the server */
export function stopServer() {
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

/** Restart the server with options from the restart modal */
export function restartServer() {
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

/** Poll for server restart completion */
export function pollForRestart(updateRequested = false) {
    const startTime = Date.now();
    const pollInterval = 3000;
    const timeout = 300000; // 5 minutes

    const check = () => {
        if (Date.now() - startTime > timeout) {
            unblockClicks();
            alert('Restart timed out. Please check logs or try again.');
            return;
        }

        console.log('Polling for restart...', { updateRequested, time: Date.now() - startTime });
        fetch('/settings/restart-status?t=' + Date.now())
            .then(res => res.json())
            .then(data => {
                console.log('Poll response:', data);
                if (data.restarted) {
                    if (updateRequested && !data.updated) {
                        console.warn('Restart detected but not updated.', data);
                        unblockClicks();
                        alert('Restart completed, but the update did not apply. You may already be on the latest version, or the update failed.');
                    } else {
                        console.log('Restart success (updated=' + data.updated + '), reloading...');
                        window.location.reload();
                    }
                } else {
                    setTimeout(check, pollInterval);
                }
            })
            .catch(err => {
                console.error('Poll network error (expected if restarting):', err);
                // Network error during polling - server might be restarting
                setTimeout(check, pollInterval);
            });
    };

    check();
}
