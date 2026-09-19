const pairing = document.querySelector('#pairing');
const start = document.querySelector('#start');
const stop = document.querySelector('#stop');
const state = document.querySelector('#state');
const error = document.querySelector('#error');
const manual = document.querySelector('#manual');
const pairingLabel = document.querySelector('label[for="pairing"]');

let manualMode = false;
let lastStatus = { active: false, mode: 'manual', error: '' };
let requestPending = false;

const popupShell = document.body.firstElementChild instanceof HTMLElement
  ? document.body.firstElementChild
  : document.body;
popupShell.classList.add('popup-shell');

const popupStyle = document.createElement('style');
popupStyle.textContent = `
  :root {
    color-scheme: dark;
    --capture-bg: #090b14;
    --capture-surface: rgba(25, 28, 43, 0.88);
    --capture-surface-raised: rgba(37, 42, 64, 0.8);
    --capture-text: #f8fafc;
    --capture-muted: #b7bfd3;
    --capture-accent: #fb7185;
    --capture-accent-strong: #e11d48;
    --capture-cyan: #38bdf8;
    --capture-success: #4ade80;
    --capture-warning: #fbbf24;
    --capture-danger: #fb7185;
  }

  html, body {
    min-width: 350px;
    min-height: 100%;
    margin: 0;
    background:
      radial-gradient(circle at 8% 4%, rgba(244, 63, 94, 0.25), transparent 34%),
      radial-gradient(circle at 92% 12%, rgba(56, 189, 248, 0.2), transparent 30%),
      var(--capture-bg);
    color: var(--capture-text);
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  }

  body {
    padding: 1px;
  }

  .popup-shell {
    position: relative;
    isolation: isolate;
    overflow: hidden;
    padding: 20px;
    border: 1px solid rgba(255, 255, 255, 0.16);
    border-radius: 18px;
    background: linear-gradient(145deg, rgba(31, 35, 53, 0.94), rgba(13, 16, 28, 0.96));
    box-shadow:
      0 22px 44px rgba(0, 0, 0, 0.38),
      inset 8px 8px 18px rgba(255, 255, 255, 0.035),
      inset -8px -8px 18px rgba(0, 0, 0, 0.32);
  }

  .popup-shell::before {
    position: absolute;
    inset: 0;
    z-index: -1;
    border-radius: inherit;
    background: conic-gradient(from 215deg at 50% 50%, transparent 0deg, rgba(56, 189, 248, 0.44) 55deg, transparent 112deg, rgba(251, 113, 133, 0.54) 190deg, transparent 250deg);
    content: "";
    opacity: 0.75;
    filter: blur(14px);
    transform: scale(1.04);
  }

  .popup-shell::after {
    position: absolute;
    top: 0;
    right: 12%;
    left: 12%;
    height: 1px;
    border-radius: 999px;
    background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.75), rgba(56, 189, 248, 0.8), transparent);
    box-shadow: 0 0 18px rgba(56, 189, 248, 0.55);
    content: "";
  }

  .popup-shell button,
  .popup-shell input {
    font: inherit;
  }

  .popup-shell button {
    border: 1px solid rgba(255, 255, 255, 0.17);
    border-radius: 10px;
    color: var(--capture-text);
    cursor: pointer;
    transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease, background 180ms ease;
  }

  .popup-shell button:hover:not(:disabled) {
    border-color: rgba(125, 211, 252, 0.72);
    box-shadow: 0 0 22px rgba(56, 189, 248, 0.18), inset 2px 2px 5px rgba(255, 255, 255, 0.08), inset -3px -3px 6px rgba(0, 0, 0, 0.24);
    transform: translateY(-1px);
  }

  .popup-shell button:active:not(:disabled) {
    transform: translateY(0) scale(0.98);
  }

  .popup-shell button:focus-visible,
  .popup-shell input:focus-visible {
    outline: 2px solid var(--capture-cyan);
    outline-offset: 3px;
  }

  .popup-shell button:disabled {
    cursor: wait;
    opacity: 0.72;
  }

  #start {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 9px;
    min-height: 42px;
    background: linear-gradient(135deg, var(--capture-accent-strong), #be123c 55%, #7c3aed);
    box-shadow: 0 10px 22px rgba(225, 29, 72, 0.22), inset 2px 2px 5px rgba(255, 255, 255, 0.16), inset -3px -3px 7px rgba(0, 0, 0, 0.25);
  }

  #start.is-loading {
    background: linear-gradient(135deg, #334155, #1e293b 54%, #155e75);
  }

  #stop {
    background: rgba(15, 23, 42, 0.76);
  }

  .button-spinner {
    width: 13px;
    height: 13px;
    border: 2px solid rgba(255, 255, 255, 0.32);
    border-top-color: currentColor;
    border-radius: 50%;
    animation: capture-spin 800ms linear infinite;
  }

  .live-status {
    display: flex;
    align-items: flex-start;
    gap: 9px;
    margin: 14px 0;
    padding: 10px 12px;
    border: 1px solid rgba(125, 211, 252, 0.2);
    border-radius: 12px;
    background: linear-gradient(135deg, rgba(56, 189, 248, 0.11), rgba(30, 41, 59, 0.52));
    color: var(--capture-muted);
    box-shadow: inset 2px 2px 5px rgba(255, 255, 255, 0.04), inset -3px -3px 7px rgba(0, 0, 0, 0.2);
  }

  .live-status[data-state="capturing"] {
    border-color: rgba(74, 222, 128, 0.3);
    background: linear-gradient(135deg, rgba(74, 222, 128, 0.13), rgba(30, 41, 59, 0.52));
  }

  .live-status[data-state="attention"] {
    border-color: rgba(251, 191, 36, 0.36);
    background: linear-gradient(135deg, rgba(251, 191, 36, 0.12), rgba(30, 41, 59, 0.52));
  }

  .live-status-dot {
    flex: 0 0 auto;
    width: 8px;
    height: 8px;
    margin-top: 5px;
    border-radius: 50%;
    background: var(--capture-cyan);
    box-shadow: 0 0 0 4px rgba(56, 189, 248, 0.12), 0 0 14px rgba(56, 189, 248, 0.8);
  }

  .live-status[data-state="capturing"] .live-status-dot {
    background: var(--capture-success);
    box-shadow: 0 0 0 4px rgba(74, 222, 128, 0.12), 0 0 14px rgba(74, 222, 128, 0.8);
  }

  .live-status[data-state="attention"] .live-status-dot {
    background: var(--capture-warning);
    box-shadow: 0 0 0 4px rgba(251, 191, 36, 0.12), 0 0 14px rgba(251, 191, 36, 0.8);
  }

  .recovery-prompt {
    margin-top: 12px;
    padding: 12px;
    border: 1px solid rgba(251, 113, 133, 0.34);
    border-radius: 12px;
    background: linear-gradient(135deg, rgba(225, 29, 72, 0.16), rgba(124, 58, 237, 0.12));
    color: #ffe4e6;
  }

  .recovery-prompt strong,
  .recovery-prompt span {
    display: block;
  }

  .recovery-prompt span {
    margin-top: 4px;
    color: #fecdd3;
    font-size: 12px;
  }

  @keyframes capture-spin {
    to { transform: rotate(360deg); }
  }

  @media (prefers-reduced-motion: reduce) {
    .popup-shell button { transition: none; }
    .button-spinner { animation: none; }
  }
`;
document.head.append(popupStyle);

const liveStatus = document.createElement('div');
liveStatus.className = 'live-status';
liveStatus.setAttribute('role', 'status');
liveStatus.setAttribute('aria-live', 'polite');
const liveStatusDot = document.createElement('span');
liveStatusDot.className = 'live-status-dot';
liveStatusDot.setAttribute('aria-hidden', 'true');
const liveStatusText = document.createElement('span');
liveStatus.append(liveStatusDot, liveStatusText);
state.insertAdjacentElement('afterend', liveStatus);

const recoveryPrompt = document.createElement('div');
recoveryPrompt.className = 'recovery-prompt';
recoveryPrompt.hidden = true;
const recoveryTitle = document.createElement('strong');
recoveryTitle.textContent = 'Automatic capture needs setup';
const recoveryText = document.createElement('span');
recoveryText.textContent = 'Open OpenDownload first, then try again. Manual pairing is available if the native host is not installed yet.';
recoveryPrompt.append(recoveryTitle, recoveryText);
error.insertAdjacentElement('afterend', recoveryPrompt);

error.setAttribute('role', 'alert');

function setStartButton(label, loading) {
  start.replaceChildren();
  if (loading) {
    const spinner = document.createElement('span');
    spinner.className = 'button-spinner';
    spinner.setAttribute('aria-hidden', 'true');
    start.append(spinner);
  }
  const text = document.createElement('span');
  text.className = 'button-label';
  text.textContent = label;
  start.append(text);
  start.classList.toggle('is-loading', loading);
  start.setAttribute('aria-busy', String(loading));
  start.disabled = loading;
}

function updateLiveStatus(status) {
  const automatic = status.active && status.mode === 'automatic';
  const manualCapture = status.active && status.mode === 'manual';
  let message = 'Ready to connect to OpenDownload.';
  let stateName = 'ready';

  if (requestPending) {
    message = 'Connecting to OpenDownload native host...';
    stateName = 'connecting';
  } else if (automatic) {
    message = 'Live capture is active for this tab.';
    stateName = 'capturing';
  } else if (manualCapture) {
    message = 'Manual capture is active for this tab.';
    stateName = 'capturing';
  } else if (status.error) {
    message = 'Automatic capture needs attention.';
    stateName = 'attention';
  } else if (manualMode) {
    message = 'Pairing mode is ready for a code from OpenDownload.';
  }

  liveStatus.dataset.state = stateName;
  liveStatusText.textContent = message;
}

function updateBusyState() {
  setStartButton(
    requestPending ? 'Connecting...' : manualMode ? 'Start manual capture' : 'Start automatic capture',
    requestPending,
  );
  stop.disabled = requestPending;
  updateLiveStatus(lastStatus);
}

function render(status) {
  lastStatus = status;
  const automatic = status.active && status.mode === 'automatic';
  const manualCapture = status.active && status.mode === 'manual';
  const showPairing = manualMode && !status.active;

  pairing.hidden = !showPairing;
  pairingLabel.hidden = !showPairing;
  manual.hidden = status.active || manualMode || !status.error;
  start.hidden = status.active;
  stop.hidden = !status.active;
  if (automatic) {
    state.textContent = 'Capturing this tab through OpenDownload.';
  } else if (manualCapture) {
    state.textContent = 'Capturing this tab with manual pairing.';
  } else if (manualMode) {
    state.textContent = 'Generate a pairing code in OpenDownload, paste it here, then start capture.';
  } else {
    state.textContent = 'Open OpenDownload, then start capture for this tab. Manual pairing is available if automatic connection needs repair.';
  }
  error.textContent = status.error || '';
  recoveryPrompt.hidden = !status.error || !/timed out/i.test(status.error);
  updateLiveStatus(status);
  updateBusyState();
}

async function refresh() {
  try {
    render(await browser.runtime.sendMessage({ type: 'status' }));
  } catch (reason) {
    showConnectionError(reason);
  }
}

function showConnectionError(reason) {
  const message = reason && reason.message ? reason.message : '';
  requestPending = false;
  if (message.includes('Receiving end does not exist')) {
    render({ ...lastStatus, error: 'The add on background is unavailable. Open about:debugging, reload OpenDownload Capture, then close and reopen this panel.' });
    return;
  }
  render({ ...lastStatus, error: message || 'Could not communicate with the OpenDownload add on.' });
}

manual.addEventListener('click', () => {
  manualMode = true;
  pairing.value = '';
  render(lastStatus);
  pairing.focus();
});

start.addEventListener('click', async () => {
  error.textContent = '';
  requestPending = true;
  render({ ...lastStatus, error: '' });
  try {
    render(await browser.runtime.sendMessage({ type: 'start', code: manualMode ? pairing.value : '', manual: manualMode }));
  } catch (reason) {
    showConnectionError(reason);
  } finally {
    requestPending = false;
    updateBusyState();
  }
});

stop.addEventListener('click', async () => {
  requestPending = true;
  updateBusyState();
  try {
    render(await browser.runtime.sendMessage({ type: 'stop' }));
    manualMode = false;
  } catch (reason) {
    showConnectionError(reason);
  } finally {
    requestPending = false;
    updateBusyState();
  }
});
refresh();
