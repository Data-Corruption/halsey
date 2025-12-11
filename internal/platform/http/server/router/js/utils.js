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

// admin panel functions ------------------------------------------------------

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
