const wrap = document.getElementById('files-wrap');
const editor = CodeMirror(document.getElementById('editor-area'), {
  lineNumbers: true,
  fixedGutter: false,
  lineWrapping: true,
  theme: 'neat',
  keyMap: 'sublime',
  mode: 'text/plain',
});
let currentPath = null;
let savedValue = '';
const saveBtn = document.getElementById('editor-save');
const pathLabel = document.getElementById('editor-path');
const statusLabel = document.getElementById('editor-status');
const backBtn = document.getElementById('back-btn');

function setDirty(dirty) {
  saveBtn.disabled = !dirty || !currentPath;
  statusLabel.textContent = dirty ? '● modified' : '';
}

function urlForPath(path) {
  return '/files/' + (path ? '?path=' + encodeURIComponent(path) : '');
}
function highlightTreeEntry(path) {
  document.querySelectorAll('a[data-file].bg-slate-100').forEach(el => el.classList.remove('bg-slate-100', 'text-slate-900'));
  if (!path) return;
  const a = document.querySelector('a[data-file="' + attrSel(path) + '"]');
  if (a) a.classList.add('bg-slate-100', 'text-slate-900');
}
function showEditor() {
  wrap.classList.add('show-editor');
  setTimeout(() => editor.refresh(), 0);
}
function showTree(opts) {
  wrap.classList.remove('show-editor');
  if (!(opts && opts.fromHistory)) {
    history.pushState(null, '', urlForPath(null));
  }
}

backBtn.addEventListener('click', () => showTree());

editor.on('change', () => {
  if (currentPath === null) return;
  setDirty(editor.getValue() !== savedValue);
});

async function openFile(path, opts) {
  opts = opts || {};
  if (currentPath !== path && savedValue !== editor.getValue() && currentPath !== null) {
    if (!confirm('Discard unsaved changes to ' + currentPath + '?')) return;
  }
  pathLabel.textContent = 'Loading ' + path + '…';
  statusLabel.textContent = '';
  showEditor();
  highlightTreeEntry(path);
  try {
    const res = await fetch('/files/read?path=' + encodeURIComponent(path));
    if (!res.ok) throw new Error(await res.text());
    const mode = res.headers.get('X-CM-Mode') || 'text/plain';
    const content = await res.text();
    currentPath = path;
    savedValue = content;
    editor.setOption('mode', mode);
    editor.setValue(content);
    editor.clearHistory();
    pathLabel.textContent = path;
    setDirty(false);
    editor.refresh();
    if (!opts.fromHistory) {
      history.pushState({path}, '', urlForPath(path));
    }
  } catch (e) {
    pathLabel.textContent = path;
    statusLabel.textContent = 'load error: ' + e.message;
  }
}

window.addEventListener('popstate', () => {
  const path = new URL(location).searchParams.get('path');
  if (path) {
    if (currentPath !== path) openFile(path, {fromHistory: true});
    else showEditor();
  } else {
    showTree({fromHistory: true});
  }
});
window.addEventListener('beforeunload', e => {
  if (currentPath && savedValue !== editor.getValue()) {
    e.preventDefault();
    e.returnValue = '';
  }
});

async function saveFile() {
  if (!currentPath) return;
  const value = editor.getValue();
  saveBtn.disabled = true;
  statusLabel.textContent = 'saving…';
  try {
    const res = await fetch('/files/save?path=' + encodeURIComponent(currentPath), {
      method: 'POST',
      headers: {'Content-Type': 'text/plain'},
      body: value,
    });
    if (!res.ok) throw new Error(await res.text());
    savedValue = value;
    statusLabel.textContent = 'saved';
    setTimeout(() => { if (statusLabel.textContent === 'saved') statusLabel.textContent = ''; }, 1500);
    setDirty(editor.getValue() !== savedValue);
  } catch (e) {
    statusLabel.textContent = 'save error: ' + e.message;
    setDirty(true);
  }
}

saveBtn.addEventListener('click', saveFile);
document.addEventListener('keydown', e => {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault();
    if (!saveBtn.disabled) saveFile();
  }
});

document.body.addEventListener('click', e => {
  const a = e.target.closest('a[data-file]');
  if (!a) return;
  e.preventDefault();
  openFile(a.dataset.file);
});

function attrSel(value) {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}
function waitForTreeLoad(treeChildren) {
  return new Promise(resolve => {
    function done() {
      treeChildren.removeEventListener('htmx:afterSwap', done);
      clearTimeout(timer);
      resolve();
    }
    treeChildren.addEventListener('htmx:afterSwap', done);
    const timer = setTimeout(done, 3000);
  });
}
async function expandToPath(path) {
  const parts = path.split('/');
  parts.pop();
  let prefix = '';
  for (const part of parts) {
    prefix = prefix ? prefix + '/' + part : part;
    const details = document.querySelector('details[data-dir="' + attrSel(prefix) + '"]');
    if (!details) return;
    if (!details.open) {
      const children = details.querySelector('.tree-children');
      const wait = waitForTreeLoad(children);
      details.open = true;
      await wait;
    }
  }
  highlightTreeEntry(path);
  const fileEl = document.querySelector('a[data-file="' + attrSel(path) + '"]');
  if (fileEl) fileEl.scrollIntoView({block: 'center'});
}

const initialPath = new URL(location).searchParams.get('path');
if (initialPath) {
  history.replaceState({path: initialPath}, '', urlForPath(initialPath));
  openFile(initialPath, {fromHistory: true});
  expandToPath(initialPath);
}
