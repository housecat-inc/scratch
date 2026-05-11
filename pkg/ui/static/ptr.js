(() => {
  const standalone = window.matchMedia('(display-mode: standalone)').matches || window.navigator.standalone === true;
  if (!standalone) return;

  const threshold = 110;
  const drag = 0.5;

  const ind = document.createElement('div');
  ind.id = 'ptr';
  ind.setAttribute('aria-hidden', 'true');
  ind.innerHTML = '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-3.5-7.1"/><path d="M21 4v5h-5"/></svg>';
  const style = document.createElement('style');
  style.textContent = `
    #ptr { position: fixed; top: calc(env(safe-area-inset-top, 0px) + 12px); left: 50%; width: 34px; height: 34px;
           background: #fff; color: #0f172a; border-radius: 9999px; box-shadow: 0 2px 8px rgba(15,23,42,.15);
           display: flex; align-items: center; justify-content: center; z-index: 1000; pointer-events: none;
           transform: translateX(-50%); opacity: 0; }
    #ptr.fade { transition: opacity .2s ease-out; }
    #ptr.spin svg { animation: ptr-spin .8s linear infinite; transform-origin: 50% 50%; }
    @keyframes ptr-spin { to { transform: rotate(360deg); } }
  `;
  document.head.appendChild(style);
  document.body.appendChild(ind);

  const arrow = ind.querySelector('svg');

  let startY = null;
  let pull = 0;
  let pulling = false;

  const reset = (animate) => {
    ind.classList.toggle('fade', animate);
    ind.classList.remove('spin');
    ind.style.opacity = '0';
    arrow.style.transform = 'rotate(0deg)';
    pull = 0;
    pulling = false;
    startY = null;
  };

  document.addEventListener('touchstart', (e) => {
    if (window.scrollY > 0 || e.touches.length !== 1) return;
    startY = e.touches[0].clientY;
    pulling = false;
    ind.classList.remove('fade');
  }, { passive: true });

  document.addEventListener('touchmove', (e) => {
    if (startY === null) return;
    if (window.scrollY > 0) { reset(false); return; }
    const dy = e.touches[0].clientY - startY;
    if (dy <= 0) { reset(false); return; }
    pulling = true;
    pull = dy * drag;
    const progress = Math.min(1, pull / threshold);
    ind.style.opacity = String(progress);
    arrow.style.transform = `rotate(${progress * 270}deg)`;
  }, { passive: true });

  document.addEventListener('touchend', () => {
    if (!pulling) { startY = null; return; }
    if (pull >= threshold) {
      ind.classList.remove('fade');
      ind.style.opacity = '1';
      ind.classList.add('spin');
      window.location.reload();
    } else {
      reset(true);
    }
    startY = null;
    pulling = false;
  }, { passive: true });

  document.addEventListener('touchcancel', () => reset(true), { passive: true });
})();
