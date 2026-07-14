# Vendored JavaScript

The editor and its supporting libraries are vendored into `pkg/ui/static` and
served from the `//go:embed` filesystem, so the SQL and Files tools work with no
runtime CDN dependency. (Tailwind's browser build is still loaded from a CDN in
`layout.templ`.)

## CodeMirror 6 bundle (built here)

`cm.js` bundles CodeMirror 6, its language grammars, the
`codemirror-languageserver` client, and the `@dprint/formatter` loader into one
self-contained IIFE, `../../static/cm.bundle.js`. Both tools load the single
bundle (it caches across pages) and read the pieces off two globals:

| Global | Used by | Provides |
| --- | --- | --- |
| `window.CMSQL` | SQL tool (`sql-editor.js`) | editor primitives, `sql`, language-server client, dprint formatter |
| `window.CMFILES` | Files tool (`files.js`) | `create()` editor with per-language grammars (Go, JS/TS, HTML, CSS, JSON, Markdown, Python, XML, YAML, shell, SQL) |

`package.json` pins every dependency. To rebuild after editing `cm.js` or
bumping versions:

```sh
cd $(mktemp -d)
cp /path/to/repo/pkg/ui/codemirror/{package.json,cm.js} .
npm install
npx esbuild cm.js --bundle --format=iife --minify --target=es2020 \
  --legal-comments=none --outfile=cm.bundle.js
cp cm.bundle.js /path/to/repo/pkg/ui/static/cm.bundle.js
```

## Pre-minified libraries (dropped in as-is)

Committed directly to `pkg/ui/static` from their upstream releases — no build:

- `htmx.min.js` — htmx 2.0.10
- `htmx-ext-sse.min.js` — htmx server-sent-events extension
- `html2canvas.min.js` — html2canvas-pro 1.5.8
- `lax-sql.wasm` — dprint lax-sql plugin 0.3.0 (vendored from
  `https://plugins.dprint.dev/bartlomieju/lax-sql-0.3.0.wasm`); the SQL Fmt action
  loads it at runtime from `/static/lax-sql.wasm`
