const pairing = document.querySelector('#pairing');
const start = document.querySelector('#start');
const stop = document.querySelector('#stop');
const state = document.querySelector('#state');
const error = document.querySelector('#error');

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
  pairing.hidden = false;
  document.querySelector('label').hidden = false;
  start.hidden = false;
  stop.hidden = !status.active;
  start.textContent = status.active ? 'Replace pairing code' : 'Start capture';
  state.textContent = status.active
    ? 'Paste the fresh pairing code from OpenDownload below, then select Replace pairing code.'
    : 'Open OpenDownload, select Generate pairing code in Detected streams, then paste its generated code here.';
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

start.addEventListener('click', async () => {
  error.textContent = '';
  try {
    render(await sendMessage({ type: 'start', code: pairing.value }));
  } catch (reason) {
    showConnectionError(reason);
  }
});

stop.addEventListener('click', async () => {
  try {
    render(await sendMessage({ type: 'stop' }));
  } catch (reason) {
    showConnectionError(reason);
  }
});

void refresh();
