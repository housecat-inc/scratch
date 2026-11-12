package dist

import "embed"

//go:embed fonts html2canvas.min.js img hx-live.min.js hx-sse.min.js htmx.min.js pill-shape.worklet.mjs ui.css ui.js
var FS embed.FS
