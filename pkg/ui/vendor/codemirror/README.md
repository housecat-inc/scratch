# cm-sql vendored bundle

`../../static/cm-sql.bundle.js` is a self-contained IIFE bundle of CodeMirror 6,
the `codemirror-languageserver` client, and the `@dprint/formatter` loader,
exposed on `window.CMSQL` and consumed by `../../static/sql-editor.js`. It is
committed (like `htmx.min.js`) so the SQL editor loads offline with no CDN
dependency.

`../../static/lax-sql.wasm` is the dprint lax-sql plugin
(`https://plugins.dprint.dev/bartlomieju/lax-sql-0.3.0.wasm`), fetched at
runtime by the Fmt action to format queries.

To rebuild after changing `vendor.js` or bumping versions:

```sh
cd $(mktemp -d)
cp /path/to/repo/pkg/ui/vendor/cm-sql/{package.json,vendor.js} .
npm install
npx esbuild vendor.js --bundle --format=iife --minify --target=es2020 \
  --legal-comments=none --outfile=cm-sql.bundle.js
cp cm-sql.bundle.js /path/to/repo/pkg/ui/static/cm-sql.bundle.js
```
