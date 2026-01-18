// Settings Wiring
// DOMContentLoaded initialization for all settings controls

import { findStatus, showPending, showSuccess, showError } from './ui.js';
import { postJSON, deleteRequest } from './api.js';
import { handleToggle, handleSelect, handleTextInput, handleCheckbox, handleRadio } from './forms.js';

/** Show restart required notice */
function showRestartNotice() {
    const notice = document.getElementById('restart-required-notice');
    if (notice) notice.classList.remove('hidden');
}

/** Wire up user settings (Settings tab) */
function wireUserSettings() {
    handleToggle('backup-opt-out', '/settings/user', 'backupOptOut');
    handleToggle('ai-chat-opt-out', '/settings/user', 'aiChatOptOut');
    handleToggle('auto-expand-reddit', '/settings/user', 'autoExpand.reddit');
    handleToggle('auto-expand-youtube-shorts', '/settings/user', 'autoExpand.youTubeShorts');
    handleToggle('auto-expand-redgifs', '/settings/user', 'autoExpand.redGifs');
}

/** Wire up admin settings (Admin tab) */
function wireAdminSettings() {
    handleSelect('admin-log-level', '/settings/admin', 'logLevel', showRestartNotice);
    handleTextInput('admin-host', '/settings/admin', 'host', 500, { onSuccess: showRestartNotice });
    handleTextInput('admin-port', '/settings/admin', 'port', 500, { onSuccess: showRestartNotice });
    handleTextInput('admin-proxy-port', '/settings/admin', 'proxyPort', 500, { onSuccess: showRestartNotice });
    handleTextInput('admin-bot-token', '/settings/admin', 'botToken', 500, { skipEmpty: true, onSuccess: showRestartNotice });
    handleTextInput('admin-ollama-url', '/settings/admin', 'ollamaURL', 500, { onSuccess: showRestartNotice });

    // Disable Auto-Expand (server-wide)
    handleToggle('admin-disable-autoexpand-reddit', '/settings/admin', 'disableAutoExpand.reddit');
    handleToggle('admin-disable-autoexpand-youtube-shorts', '/settings/admin', 'disableAutoExpand.youTubeShorts');
    handleToggle('admin-disable-autoexpand-redgifs', '/settings/admin', 'disableAutoExpand.redGifs');

    // yt-dlp Update button (one-shot action, not a toggle)
    const ytdlpBtn = document.getElementById('admin-update-yt-dlp');
    if (ytdlpBtn) {
        const status = ytdlpBtn.parentElement.querySelector('.status');
        ytdlpBtn.addEventListener('click', async () => {
            ytdlpBtn.disabled = true;
            showPending(status);
            try {
                const res = await fetch('/settings/update-yt-dlp', { method: 'GET' });
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text || `HTTP ${res.status}`);
                }
                showSuccess(status);
            } catch (e) {
                showError(status, e.message);
            } finally {
                ytdlpBtn.disabled = false;
            }
        });
    }
}

/** Wire up guild-specific settings */
function wireGuildSettings() {
    // Only select the collapse container divs, not child elements with data-guild-id
    document.querySelectorAll('.collapse[data-guild-id]').forEach(collapse => {
        const guildId = collapse.dataset.guildId;
        if (!guildId) return;

        const endpoint = `/settings/guild/${guildId}`;

        // Backup Password
        handleTextInput(`guild-${guildId}-backup-password`, endpoint, 'backupPassword', 500, { skipEmpty: true });

        // Synctube URL
        handleTextInput(`guild-${guildId}-synctube`, endpoint, 'synctubeURL', 500);

        // Toggle settings
        handleToggle(`guild-${guildId}-backup`, endpoint, 'backupEnabled');
        handleToggle(`guild-${guildId}-antirot`, endpoint, 'antiRotEnabled');
        handleToggle(`guild-${guildId}-aichat`, endpoint, 'aiChatEnabled');
    });
}

/** Wire up channel settings (backup and AI chat checkboxes) */
function wireChannelSettings() {
    // Channel backup checkboxes
    document.querySelectorAll('.channel-backup').forEach(checkbox => {
        const channelId = checkbox.dataset.channelId;
        if (!channelId) return;
        handleCheckbox(checkbox, `/settings/channel/${channelId}`,
            checked => ({ backupEnabled: checked }));
    });

    // Channel AI chat checkboxes
    document.querySelectorAll('.channel-aichat').forEach(checkbox => {
        const channelId = checkbox.dataset.channelId;
        if (!channelId) return;
        handleCheckbox(checkbox, `/settings/channel/${channelId}`,
            checked => ({ aiChat: checked }));
    });

    // Fav channel radio buttons
    document.querySelectorAll('.guild-fav-channel').forEach(radio => {
        handleRadio(radio,
            r => `/settings/guild/${r.dataset.guildId}`,
            value => ({ favChannelID: value }));
    });

    // Bot channel radio buttons
    document.querySelectorAll('.guild-bot-channel').forEach(radio => {
        handleRadio(radio,
            r => `/settings/guild/${r.dataset.guildId}`,
            value => ({ botChannelID: value }));
    });
}

/** Wire up delete guild button and confirmation modal */
function wireDeleteGuild() {
    const modal = document.getElementById('delete-guild-modal');
    const nameDisplay = document.getElementById('delete-guild-name');
    const confirmInput = document.getElementById('delete-guild-confirm-input');
    const confirmBtn = document.getElementById('delete-guild-confirm-btn');
    const guildIdInput = document.getElementById('delete-guild-id');

    if (!modal || !confirmBtn) return;

    // Wire up all delete buttons
    document.querySelectorAll('.delete-guild-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const guildId = btn.dataset.guildId;
            const guildName = btn.dataset.guildName;

            // Set up modal
            nameDisplay.textContent = guildName;
            guildIdInput.value = guildId;
            confirmInput.value = '';
            confirmBtn.disabled = true;

            // Show modal
            modal.showModal();
        });
    });

    // Enable/disable confirm button based on input matching guild name
    confirmInput.addEventListener('input', () => {
        const expectedName = nameDisplay.textContent;
        confirmBtn.disabled = confirmInput.value !== expectedName;
    });

    // Handle confirm button click
    confirmBtn.addEventListener('click', async () => {
        const guildId = guildIdInput.value;
        if (!guildId) return;

        confirmBtn.disabled = true;
        confirmBtn.innerHTML = '<span class="loading loading-spinner loading-sm"></span> Deleting...';

        try {
            await deleteRequest(`/settings/guild/${guildId}`);

            // Close modal and reload page to reflect changes
            modal.close();
            window.location.reload();
        } catch (e) {
            confirmBtn.disabled = false;
            confirmBtn.textContent = 'Delete Guild';

            // Show error modal
            const errorModal = document.getElementById('error-modal');
            const errorMsg = document.getElementById('error-modal-message');
            if (errorModal && errorMsg) {
                errorMsg.textContent = e.message || 'Failed to delete guild.';
                errorModal.showModal();
            }
        }
    });

    // Reset modal state when closed
    modal.addEventListener('close', () => {
        confirmInput.value = '';
        confirmBtn.disabled = true;
        confirmBtn.textContent = 'Delete Guild';
    });
}

/** Wire up user permission management (admin panel) */
function wireUserManagement() {
    // Admin checkboxes
    document.querySelectorAll('.user-admin').forEach(checkbox => {
        const userId = checkbox.dataset.userId;
        if (!userId) return;
        handleCheckbox(checkbox, `/settings/user/${userId}`,
            checked => ({ isAdmin: checked }));
    });

    // Backup access checkboxes
    document.querySelectorAll('.user-backup').forEach(checkbox => {
        const userId = checkbox.dataset.userId;
        if (!userId) return;
        handleCheckbox(checkbox, `/settings/user/${userId}`,
            checked => ({ backupAccess: checked }));
    });

    // AI access checkboxes
    document.querySelectorAll('.user-ai').forEach(checkbox => {
        const userId = checkbox.dataset.userId;
        if (!userId) return;
        handleCheckbox(checkbox, `/settings/user/${userId}`,
            checked => ({ aiAccess: checked }));
    });
}

/** Initialize all settings on DOMContentLoaded */
export function initSettings() {
    wireUserSettings();
    wireAdminSettings();
    wireGuildSettings();
    wireChannelSettings();
    wireDeleteGuild();
    wireUserManagement();
}
