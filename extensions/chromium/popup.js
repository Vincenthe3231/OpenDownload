const pairing = document.querySelector('#pairing');
const start = document.querySelector('#start');
const stop = document.querySelector('#stop');
const state = document.querySelector('#state');
const error = document.querySelector('#error');
const manual = document.querySelector('#manual');
const pairingLabel = document.querySelector('label[for="pairing"]');

let manualMode = false;
let lastStatus = { active: false, mode: 'manual', error: '' };

function sendMessage(message) {
  return new Promise((resolve, reject) => {
    chrome.runtime.sendMessage(message, (response) => {
      const runtimeError = chrome.runtime.lastError;
      if (runtimeError) {
        reject(new Error(runtimeError.message));
        return;
      }
      resolve(response);
    });
  });
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
    start.textContent = 'Start manual capture';
    state.textContent = 'Generate a pairing code in OpenDownload, paste it here, then start capture.';
  } else {
    start.textContent = 'Start automatic capture';
    state.textContent = 'Open OpenDownload, then start capture for this tab. Manual pairing is available if automatic connection needs repair.';
  }
  error.textContent = status.error || '';
}

async function refresh() {
  try {
    render(await sendMessage({ type: 'status' }));
  } catch (reason) {
    showConnectionError(reason);
  }
}

function showConnectionError(reason) {
  const message = reason && reason.message ? reason.message : '';
  if (message.includes('Receiving end does not exist')) {
    error.textContent = 'The extension background is unavailable. Reload OpenDownload Capture in the extensions page, then close and reopen this panel.';
    return;
  }
  error.textContent = message || 'Could not communicate with the OpenDownload extension.';
}

manual.addEventListener('click', () => {
  manualMode = true;
  pairing.value = '';
  render(lastStatus);
  pairing.focus();
});

start.addEventListener('click', async () => {
  error.textContent = '';
  try {
    render(await sendMessage({ type: 'start', code: manualMode ? pairing.value : '', manual: manualMode }));
  } catch (reason) {
    showConnectionError(reason);
  }
});

stop.addEventListener('click', async () => {
  try {
    render(await sendMessage({ type: 'stop' }));
    manualMode = false;
  } catch (reason) {
    showConnectionError(reason);
  }
});

void refresh();
