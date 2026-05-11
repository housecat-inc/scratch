const CACHE = 'cc-shell-v1';
const SHELL = ['/', '/setup', '/code/', '/files/', '/manifest.webmanifest', '/static/icon.svg', '/static/icon-180.png', '/static/icon-192.png', '/static/icon-512.png'];

self.addEventListener('install', event => {
  event.waitUntil(caches.open(CACHE).then(cache => cache.addAll(SHELL)).catch(() => {}));
  self.skipWaiting();
});

self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys().then(keys => Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k))))
  );
  self.clients.claim();
});

self.addEventListener('fetch', event => {
  const req = event.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== location.origin) return;
  event.respondWith(
    fetch(req).then(res => {
      if (res && res.ok && (url.pathname === '/' || url.pathname.startsWith('/static/') || url.pathname === '/manifest.webmanifest' || url.pathname === '/setup' || url.pathname === '/code/' || url.pathname === '/files/')) {
        const copy = res.clone();
        caches.open(CACHE).then(cache => cache.put(req, copy)).catch(() => {});
      }
      return res;
    }).catch(() => caches.match(req).then(hit => hit || caches.match('/')))
  );
});
