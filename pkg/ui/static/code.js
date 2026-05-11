function expandAllBlocks(btn, evt) {
  evt.preventDefault(); evt.stopPropagation();
  const file = btn.closest('details');
  const hunks = Array.from(file.querySelectorAll('details'));
  if (hunks.length === 0) return;
  const allOpen = hunks.every(d => d.open);
  if (allOpen) {
    hunks.forEach(d => d.open = false);
  } else {
    file.open = true;
    hunks.forEach(d => d.open = true);
  }
}
function expandWholeFile(btn, evt) {
  evt.preventDefault(); evt.stopPropagation();
  const file = btn.closest('details');
  const hunks = Array.from(file.querySelectorAll('details'));
  if (hunks.length === 0) return;
  const allOpen = hunks.every(d => d.open);
  if (allOpen) {
    hunks.forEach(d => d.open = false);
    file.querySelectorAll('[id^="prev-lines-"]').forEach(c => { c.innerHTML = ''; });
    file.querySelectorAll('[id^="prev-buttons-"]').forEach(span => {
      const tmpl = document.getElementById('initial-prev-' + span.id.substring('prev-buttons-'.length));
      if (tmpl) {
        span.innerHTML = tmpl.innerHTML;
        if (window.htmx) htmx.process(span);
      }
    });
  } else {
    file.open = true;
    hunks.forEach(d => d.open = true);
    file.querySelectorAll('button[data-expand-all="true"]').forEach(b => b.click());
  }
}
function markFileDone(btn, evt) {
  evt.preventDefault(); evt.stopPropagation();
  const file = btn.closest('details');
  const done = !file.classList.contains('opacity-60');
  file.classList.toggle('opacity-60', done);
  btn.classList.toggle('text-green-600', done);
  btn.classList.toggle('text-slate-400', !done);
  file.open = !done;
}
function markHunkDone(btn, evt) {
  evt.preventDefault(); evt.stopPropagation();
  const hunk = btn.closest('details');
  const done = !hunk.classList.contains('opacity-60');
  hunk.classList.toggle('opacity-60', done);
  btn.classList.toggle('text-green-600', done);
  btn.classList.toggle('text-slate-400', !done);
  hunk.open = !done;
}
document.addEventListener('toggle', function(evt) {
  const file = evt.target;
  if (!(file instanceof HTMLDetailsElement)) return;
  if (!file.classList.contains('group')) return;
  if (!file.open) return;
  const firstHunk = file.querySelector('details');
  if (firstHunk) firstHunk.open = true;
}, true);
function highlightNew(root) {
  if (!window.hljs) return;
  (root || document).querySelectorAll('pre code:not([data-highlighted])').forEach(el => {
    try { hljs.highlightElement(el); } catch (_) {}
  });
}
window.addEventListener('load', () => highlightNew());
document.body.addEventListener('htmx:afterSwap', evt => highlightNew(evt.detail.target));
document.body.addEventListener('htmx:oobAfterSwap', evt => highlightNew(evt.detail.target));
function handlePrevClick(btn, event) {
  const span = btn.closest('[id^="prev-buttons-"]');
  if (!span) return;
  const key = span.id.substring('prev-buttons-'.length);
  const lines = document.getElementById('prev-lines-' + key);
  if (!lines || lines.children.length === 0) return;
  event.preventDefault();
  lines.innerHTML = '';
  const tmpl = document.getElementById('initial-prev-' + key);
  if (tmpl) {
    span.innerHTML = tmpl.innerHTML;
    if (window.htmx) htmx.process(span);
  }
}
