(() => {
  if (window.scratchUpdatesLoaded) return;
  window.scratchUpdatesLoaded = true;
  let instance;
  let restarting = false;
  let started = 0;

  async function refresh() {
    const button = document.querySelector('[data-app-restart]');
    if (!button) return;
    try {
      const response = await fetch('/app/updates', { cache: 'no-store' });
      if (!response.ok) return;
      const status = await response.json();
      if (restarting && status.instance !== instance) {
        location.reload();
        return;
      }
      instance = status.instance;
      if (!restarting) button.hidden = !status.pending;
    } catch (_) {
    } finally {
      if (restarting && Date.now() - started > 60000) {
        restarting = false;
        button.disabled = false;
        button.textContent = 'Restart unavailable, retry';
      }
    }
  }

  document.addEventListener('click', async (event) => {
    const button = event.target.closest('[data-app-restart]');
    if (!button || restarting || !instance) return;
    restarting = true;
    started = Date.now();
    button.disabled = true;
    button.textContent = 'Restarting…';
    try {
      const response = await fetch('/app/restart', {
        method: 'POST',
        headers: { 'X-Scratch-Instance': instance },
      });
      if (!response.ok) {
        restarting = false;
        button.disabled = false;
        button.textContent = 'Restart failed, retry';
      }
    } catch (_) {
    }
  });

  refresh();
  setInterval(refresh, 3000);
})();
