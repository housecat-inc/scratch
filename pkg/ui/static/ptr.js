(() => {
  const standalone = window.matchMedia('(display-mode: standalone)').matches || window.navigator.standalone === true;
  if (!standalone) return;

  const threshold = 70;
  const maxPull = 110;
  const drag = 0.5;

  const ind = document.createElement('div');
  ind.id = 'ptr';
  ind.setAttribute('aria-hidden', 'true');
  ind.innerHTML = '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-3.5-7.1"/><path d="M21 4v5h-5"/></svg>';
  const style = document.createElement('style');
  style.textContent = `
    #ptr { position: fixed; top: calc(env(safe-area-inset-top, 0px) + 4px); left: 50%; width: 34px; height: 34px;
           background: #fff; color: #0f172a; border-radius: 9999px; box-shadow: 0 2px 8px rgba(15,23,42,.15);
           display: flex; align-items: center; justify-content: center; z-index: 1000; pointer-events: none;
           transform: translate(-50%, -150%) rotate(0deg); opacity: 0; }
    #ptr.show { opacity: 1; }
    #ptr.snap { transition: transform .2s ease-out, opacity .2s ease-out; }
    #ptr.spin svg { animation: ptr-spin .8s linear infinite; }
    @keyframes ptr-spin { to { transform: rotate(360deg); } }
  `;
  document.head.appendChild(style);
  document.body.appendChild(ind);

  let startY = null;
  let pull = 0;
  let pulling = false;

  const reset = (animate) => {
    ind.classList.toggle('snap', animate);
    ind.classList.remove('show');
    ind.style.transform = 'translate(-50%, -150%) rotate(0deg)';
    pull = 0;
    pulling = false;
    startY = null;
  };

  document.addEventListener('touchstart', (e) => {
    if (window.scrollY > 0 || e.touches.length !== 1) return;
    startY = e.touches[0].clientY;
    pulling = false;
    ind.classList.remove('snap');
  }, { passive: true });

  document.addEventListener('touchmove', (e) => {
    if (startY === null) return;
    if (window.scrollY > 0) { reset(false); return; }
    const dy = e.touches[0].clientY - startY;
    if (dy <= 0) { reset(false); return; }
    pulling = true;
    pull = Math.min(maxPull, dy * drag);
    ind.classList.add('show');
    ind.style.transform = `translate(-50%, ${pull}px) rotate(${pull * 4}deg)`;
  }, { passive: true });

  document.addEventListener('touchend', () => {
    if (!pulling) { startY = null; return; }
    ind.classList.add('snap');
    if (pull >= threshold) {
      ind.classList.add('spin');
      ind.style.transform = `translate(-50%, ${threshold}px) rotate(0deg)`;
      window.location.reload();
    } else {
      reset(true);
    }
    startY = null;
    pulling = false;
  }, { passive: true });

  document.addEventListener('touchcancel', () => reset(true), { passive: true });
})();
