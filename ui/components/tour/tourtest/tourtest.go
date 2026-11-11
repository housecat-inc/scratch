// Package tourtest reads the tour's overlay on a page a browser test drives.
// The kit's tour tests and the app's first-run tour tests share it.
package tourtest

// Hole is the hole tour.ts cuts in the overlay's dim, or null when there
// is none: its box, from the path's own geometry, and its corner radii as
// [x, y], top left first and then clockwise. drawHole writes the viewport and
// then the hole, from its top edge clockwise, so the hole is what follows the
// first Z and its arcs run top right, bottom right, bottom left, top left.
const Hole = `() => {
	const overlay = document.querySelector("[data-slot=tour-overlay]");
	const d = overlay?.querySelector("[data-slot=tour-overlay-dim]")?.getAttribute("d") ?? "";
	const cut = d.slice(d.indexOf("Z") + 1);
	if (!cut) return null;
	const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
	path.setAttribute("d", cut);
	overlay.appendChild(path);
	const box = path.getBBox();
	path.remove();
	const arcs = Array.from(cut.matchAll(/A(\S+) (\S+) /g), (m) => [Number(m[1]), Number(m[2])]);
	return { bottom: box.y + box.height, left: box.x, radii: [arcs[3], arcs[0], arcs[1], arcs[2]], right: box.x + box.width, top: box.y };
}`
