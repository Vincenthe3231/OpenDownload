const pairing = document.querySelector('#pairing');
const start = document.querySelector('#start');
const stop = document.querySelector('#stop');
const state = document.querySelector('#state');
const error = document.querySelector('#error');

function render(status) {
  pairing.hidden = status.active;
  document.querySelector('label').hidden = status.active;
  start.hidden = status.active;
  stop.hidden = !status.active;
  state.textContent = status.active ? 'Capturing only this tab.' : 'Open OpenDownload, select Start capture in Detected streams, then paste its generated code here.';
  error.textContent = status.error || '';
}

async function refresh() {
  render(await browser.runtime.sendMessage({ type: 'status' }));
}

start.addEventListener('click', async () => {
  error.textContent = '';
  try {
    render(await browser.runtime.sendMessage({ type: 'start', code: pairing.value }));
  } catch (reason) {
    error.textContent = reason.message || 'Could not start capture.';
  }
});

stop.addEventListener('click', async () => render(await browser.runtime.sendMessage({ type: 'stop' })));
refresh();
