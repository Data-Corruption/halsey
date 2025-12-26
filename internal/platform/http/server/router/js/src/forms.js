// Form Handlers
// Generic handlers for toggles, selects, and text inputs with debouncing

import { findStatus, showPending, showSuccess, showError } from './ui.js';
import { postJSON } from './api.js';

/**
 * Generic handler for toggles/checkboxes (immediate POST on change)
 * @param {string|HTMLElement} inputOrId - Input element or ID
 * @param {string} endpoint - POST endpoint
 * @param {string} fieldName - JSON field name (supports nested like "autoExpand.reddit")
 */
export function handleToggle(inputOrId, endpoint, fieldName) {
    const input = typeof inputOrId === 'string'
        ? document.getElementById(inputOrId)
        : inputOrId;
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

            await postJSON(endpoint, body);
            showSuccess(status);
        } catch (e) {
            showError(status, e.message);
        }
    });
}

/**
 * Generic handler for select dropdowns (immediate POST on change)
 * @param {string|HTMLElement} inputOrId - Input element or ID
 * @param {string} endpoint - POST endpoint
 * @param {string} fieldName - JSON field name
 * @param {Function} [onSuccess] - Optional success callback
 */
export function handleSelect(inputOrId, endpoint, fieldName, onSuccess) {
    const input = typeof inputOrId === 'string'
        ? document.getElementById(inputOrId)
        : inputOrId;
    if (!input) return;

    const status = findStatus(input);

    input.addEventListener('change', async () => {
        showPending(status);
        try {
            await postJSON(endpoint, { [fieldName]: input.value });
            showSuccess(status);
            if (onSuccess) onSuccess();
        } catch (e) {
            showError(status, e.message);
        }
    });
}

/**
 * Generic handler for text/number inputs with debouncing
 * @param {string|HTMLElement} inputOrId - Input element or ID
 * @param {string} endpoint - POST endpoint
 * @param {string} fieldName - JSON field name
 * @param {number} [debounceMs=500] - Debounce delay in milliseconds
 * @param {object} [opts] - Options: { skipEmpty, onSuccess }
 */
export function handleTextInput(inputOrId, endpoint, fieldName, debounceMs = 500, opts = {}) {
    const input = typeof inputOrId === 'string'
        ? document.getElementById(inputOrId)
        : inputOrId;
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

                await postJSON(endpoint, { [fieldName]: value }, controller.signal);
                showSuccess(status);
                if (opts.onSuccess) opts.onSuccess();
            } catch (e) {
                if (e.name !== 'AbortError') {
                    showError(status, e.message);
                }
            }
        }, debounceMs);
    });
}

/**
 * Wire a checkbox with a custom body builder
 * @param {HTMLElement} checkbox - Checkbox element
 * @param {string} endpoint - POST endpoint
 * @param {Function} bodyBuilder - Function that takes checkbox.checked and returns body object
 */
export function handleCheckbox(checkbox, endpoint, bodyBuilder) {
    const status = findStatus(checkbox);

    checkbox.addEventListener('change', async () => {
        showPending(status);
        try {
            await postJSON(endpoint, bodyBuilder(checkbox.checked));
            showSuccess(status);
        } catch (e) {
            showError(status, e.message);
        }
    });
}

/**
 * Wire a radio button (no status indicator, just console.error on failure)
 * @param {HTMLElement} radio - Radio element
 * @param {Function} getEndpoint - Function that returns endpoint from radio data
 * @param {Function} bodyBuilder - Function that returns body from radio value
 */
export function handleRadio(radio, getEndpoint, bodyBuilder) {
    radio.addEventListener('change', async () => {
        try {
            await postJSON(getEndpoint(radio), bodyBuilder(radio.value));
        } catch (e) {
            console.error('Failed to update:', e);
        }
    });
}
