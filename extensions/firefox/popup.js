const pairing = document.querySelector('#pairing');
const start = document.querySelector('#start');
const stop = document.querySelector('#stop');
const state = document.querySelector('#state');
const error = document.querySelector('#error');

function render(status) {
  pairing.hidden = false;
  document.querySelector('label').hidden = false;
  start.hidden = false;
  stop.hidden = !status.active;
  start.textContent = status.active ? 'Replace pairing code' : 'Start capture';
  state.textContent = status.active ? 'Paste the fresh pairing code from OpenDownload below, then select Replace pairing code.' : 'Open OpenDownload, select Generate pairing code in Detected streams, then paste its generated code here.';
  error.textContent = status.error || '';
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
  if (message.includes('Receiving end does not exist')) {
    error.textContent = 'The add on background is unavailable. Open about:debugging, reload OpenDownload Capture, then close and reopen this panel.';
    return;
  }
  error.textContent = message || 'Could not communicate with the OpenDownload add on.';
}

start.addEventListener('click', async () => {
  error.textContent = '';
  try {
    render(await browser.runtime.sendMessage({ type: 'start', code: pairing.value }));
  } catch (reason) {
    showConnectionError(reason);
  }
});

stop.addEventListener('click', async () => {
  try {
    render(await browser.runtime.sendMessage({ type: 'stop' }));
  } catch (reason) {
    showConnectionError(reason);
  }
});
refresh();
