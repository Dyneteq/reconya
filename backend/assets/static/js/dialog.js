/* The console's one dialog.
 *
 * The device modal is gone — the drawer replaced it — so this is only used for
 * the things that genuinely interrupt: adding or editing a network, confirming
 * a delete, and About.
 */
(function () {
    'use strict';

    var root, titleEl, bodyEl, footEl;
    var onSubmit = null;

    function ready() {
        root = RC.el('rc-dialog');
        titleEl = RC.el('rc-dialog-title');
        bodyEl = RC.el('rc-dialog-body');
        footEl = RC.el('rc-dialog-foot');
        if (!root) return;

        root.addEventListener('click', function (e) {
            if (e.target.closest('[data-dialog-close]')) closeDialog();
        });

        document.addEventListener('keydown', function (e) {
            if (e.key === 'Escape' && root.classList.contains('is-open')) closeDialog();
        });
    }

    /* openDialog({title, body, confirm, danger, onConfirm})
     *
     * body is trusted HTML — every caller builds it from RC.esc'd values.
     * Omitting `confirm` gives a dismiss-only dialog (About, errors).
     */
    function openDialog(opts) {
        if (!root) ready();
        if (!root) return;

        titleEl.textContent = opts.title || '';
        bodyEl.innerHTML = opts.body || '';
        onSubmit = opts.onConfirm || null;

        var buttons = '<button class="rc-btn" data-dialog-close>' +
            RC.esc(opts.cancel || 'CANCEL') + '</button>';
        if (opts.confirm) {
            buttons += '<button class="rc-btn ' +
                (opts.danger ? 'rc-btn--danger' : 'rc-btn--solid') +
                '" id="rc-dialog-confirm">' + RC.esc(opts.confirm) + '</button>';
        }
        footEl.innerHTML = buttons;

        var confirmBtn = RC.el('rc-dialog-confirm');
        if (confirmBtn) {
            confirmBtn.addEventListener('click', function () {
                if (onSubmit) onSubmit();
            });
        }

        root.classList.add('is-open');

        var firstField = bodyEl.querySelector('input, textarea, select');
        if (firstField) firstField.focus();
    }

    function closeDialog() {
        if (!root) return;
        root.classList.remove('is-open');
        onSubmit = null;
    }

    // Reports a failure without falling back to alert(), which looks nothing
    // like the rest of the console.
    function failDialog(title, message) {
        openDialog({
            title: title,
            body: '<div style="font-size:12px;color:var(--rc-text-3);line-height:1.6">' +
                RC.esc(message) + '</div>',
            cancel: 'CLOSE'
        });
    }

    document.addEventListener('DOMContentLoaded', ready);

    window.openDialog = openDialog;
    window.closeDialog = closeDialog;
    window.failDialog = failDialog;
})();
