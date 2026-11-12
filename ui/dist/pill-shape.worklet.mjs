/*!
* @klinking/squircle — MIT License — Copyright (c) 2026 Chris Klink
* https://squircle.klink.ing/ · https://github.com/klink-ing/squircle
*/
const e = `--squircle-pill`, t = `${e}-arc-setback`, n = `${e}-edge-setback`, r = `${e}-border-style`, i = `${e}-continuity`, a = `${e}-side`, o = `${e}-border-width`, s = `${e}-border-color`, c = `${e}-outline-width`, l = `${e}-outline-offset`, u = `${e}-outline-color`, d = `${e}-outline-style`, f = `${e}-ring-width`, p = `${e}-ring-color`, m = `${e}-ring-offset-width`, h = `${e}-ring-offset-color`, g = `${e}-inset-ring-width`, _ = `${e}-inset-ring-color`, v = `${e}-box-shadow`, y = `${e}-resolved-shadow`, b = `${e}-paint-zoom`, x = `${e}-reach`, S = `${e}-background-inset`, C = `${e}-max-radius`, w = [
	x,
	o,
	s,
	r,
	c,
	l,
	u,
	d,
	f,
	p,
	m,
	h,
	g,
	_,
	v,
	y,
	b,
	`color`
], T = .5;
function E(e, t) {
	let n = [], r = 0, i = ``;
	for (let a of e) {
		if (a === `(`) r++;
		else if (a === `)`) r--;
		else if (r === 0 && (t === `,` ? a === `,` : /\s/.test(a))) {
			i.trim() && n.push(i.trim()), i = ``;
			continue;
		}
		i += a;
	}
	return i.trim() && n.push(i.trim()), n;
}
function ee(e) {
	let t = /^(-?(?:\d+\.?\d*|\.\d+))(px|rem|em)?$/.exec(e);
	if (!t) return null;
	let n = Number(t[1]);
	return t[2] ? t[2] === `px` ? n : n * 16 : n === 0 ? 0 : null;
}
function D(e, t) {
	if (!/currentcolor/i.test(e)) return e;
	let n = t?.get(`color`)?.toString().trim() || `#000`;
	return e.replace(/currentcolor/gi, n);
}
function O(e) {
	let t = e.trim().toLowerCase();
	return !(t === `transparent` || t === `#0000` || t === `#00000000` || /^rgba\(.*,\s*0\)$/.test(t) || /\/\s*0%?\s*\)$/.test(t));
}
const k = 1024, A = /* @__PURE__ */ new Map(), j = /* @__PURE__ */ new Map(), M = 1024, N = /* @__PURE__ */ new Map(), P = /* @__PURE__ */ new Map(), F = 1024, I = /* @__PURE__ */ new Map(), L = /* @__PURE__ */ new Map(), R = /* @__PURE__ */ new Map(), z = (e, t, n, r) => (e.size >= t && e.delete(e.keys().next().value), e.set(n, r), r), te = Math.PI / 2, B = /* @__PURE__ */ new Map(), V = /* @__PURE__ */ new Float64Array(513), H = /* @__PURE__ */ new Float64Array(513);
for (let e = 0; e <= 512; e++) {
	let t = e / 512;
	V[e] = Math.log(1 - t), H[e] = Math.log(1 - t * t);
}
const U = (e, t) => t === 0 ? 1 : Math.exp(t * e);
function W(e, t, n, r = e ** t) {
	return Math.max(0, 1 - e * (1 - 2 * r / (t + 1) + r * r / (2 * t + 1)) / n);
}
function G(e, t) {
	let n = 0;
	t && (t[0] = 0);
	for (let r = 0; r < 128; r++) {
		let i = r === 0 ? 9 * e[0] + 19 * e[1] - 5 * e[2] + e[3] : r === 127 ? e[125] - 5 * e[126] + 19 * e[127] + 9 * e[128] : 13 * (e[r] + e[r + 1]) - e[r - 1] - e[r + 2];
		n += i * .0003255208333333333, t && (t[r + 1] = n);
	}
	return n;
}
function K(e, t, n) {
	let r = Math.min(Math.floor(n / 4), 127), i = n / 4 - r;
	if (i === 0) return e[r];
	let a = i * i, o = a * i, s = 1 / 128;
	return (2 * o - 3 * a + 1) * e[r] + (o - 2 * a + i) * s * t[r] + (3 * a - 2 * o) * e[r + 1] + (o - a) * s * t[r + 1];
}
const q = (e) => Array(e).fill(.5), J = () => q(129), ne = J(), re = J(), Y = () => ({
	cos: J(),
	sin: J(),
	dcos: J(),
	dsin: J()
}), X = [Y(), Y()], Z = .03, ie = {
	2: 2.25,
	3: 3.25
}, ae = /* @__PURE__ */ new Map(), Q = 1024, $ = class {
	static get inputProperties() {
		return [
			t,
			n,
			i,
			a,
			C,
			S
		];
	}
	resolveDash(e, t) {
		let n = e?.trim() || `solid`;
		return n === `none` || n === `hidden` ? null : n === `dashed` ? [t * 3, t * 2] : n === `dotted` ? [t, t * 2] : [];
	}
	resolveSetbacks(e) {
		let r = e?.get(t)?.toString().trim() ?? ``, i = /^([+-]?(?:\d*\.)?\d+(?:e[+-]?\d+)?)(deg|rad|grad|turn)?$/i.exec(r), a = i?.[2]?.toLowerCase(), o = i ? Number(i[1]) * (a === `rad` ? 180 / Math.PI : a === `grad` ? .9 : a === `turn` ? 360 : 1) : 45, s = Math.min(Math.max(o * Math.PI / 180, 0), te), c = Number.parseFloat(e?.get(n)?.toString() ?? ``), l = Number.isFinite(c) ? Math.max(c, 0) : .75, u = this.resolveContinuity(e), d = `${s},${l},${u}`, f = B.get(d);
		if (f) return f;
		let p = (e, t) => {
			let n = this.samples(t, u), r = this.capMetrics(1, e, n.lambda, this.fresnel(e, n));
			return r.junction - r.radius;
		}, m = 1;
		if (s > 0 && l > 0) {
			let e = 1, t = 2;
			for (; t < 128 && p(s, t) < l;) e = t, t *= 2;
			if (t = Math.min(t, 128), p(s, t) < l) m = t;
			else {
				for (let n = 0; n < 32; n++) {
					let n = (e + t) / 2;
					p(s, n) < l ? e = n : t = n;
				}
				m = (e + t) / 2;
			}
		}
		return z(B, M, d, {
			beta: s,
			exponent: m
		});
	}
	resolveAngle(e) {
		return this.resolveSetbacks(e).beta;
	}
	resolveExponent(e) {
		return this.resolveSetbacks(e).exponent;
	}
	resolveContinuity(e) {
		let t = Number.parseFloat(e?.get(i)?.toString() ?? ``);
		return Number.isFinite(t) && t < 2.5 ? 2 : 3;
	}
	resolveSide(e) {
		let t = e?.get(a)?.toString().trim();
		return t === `t` || t === `r` || t === `b` || t === `l` ? t : `auto`;
	}
	resolveMaxRadius(e) {
		let t = Number.parseFloat(e?.get(C)?.toString() ?? ``);
		return Number.isFinite(t) ? Math.max(0, t) : 1 / 0;
	}
	profile(e, t) {
		let n = `${e},${t}`, r = I.get(n);
		if (r) return r;
		let i = this.samples(e, t), a = /* @__PURE__ */ new Float64Array(513);
		for (let n = 0; n <= 512; n++) a[n] = this.remainingAt(i, e, t, n);
		return z(I, 64, n, {
			remaining: a,
			lambda: i.lambda,
			samples: i
		});
	}
	remainingAt(e, t, n, r) {
		if (r % 4 == 0) return e.remaining[r / 4];
		if (n === 2) return U(V[r], t);
		if (t < 3) {
			let n = r / 512;
			return t <= 1 ? 1 - n : W(n, 4 / (t - 1), 1 / e.lambda);
		}
		return K(e.remaining, e.slope, r);
	}
	samples(e, t) {
		let n = `${e},${t}`, r = L.get(n);
		if (r) return r;
		let i = J(), a = J(), o = e;
		if (t === 2) for (let t = 0; t <= 128; t++) i[t] = U(V[t * 4], e);
		else if (e < 3) {
			let t = e > 1 ? 4 / (e - 1) : 1 / 0, n = 1 - 2 / (t + 1) + 1 / (2 * t + 1);
			for (let r = 0; r <= 128; r++) {
				let o = r / 128, s = e > 1 ? o ** t : 0;
				i[r] = e > 1 ? W(o, t, n, s) : 1 - o, a[r] = -((1 - s) ** 2) / n;
			}
			i[128] = 0, o = 1 / n;
		} else {
			let t = Math.max(e - 1, 1);
			for (let e = 0; e <= 128; e++) a[e] = U(H[e * 4], t);
			let n = G(a, i);
			for (let e = 0; e <= 128; e++) i[e] = 1 - i[e] / n, a[e] = -a[e] / n;
			o = 1 / n;
		}
		return z(L, 64, n, {
			remaining: i,
			slope: a,
			lambda: o
		});
	}
	fresnel(e, t) {
		let n = Y(), { totalCos: r, totalSin: i } = this.integrals(e, t, n);
		return {
			...n,
			totalCos: r,
			totalSin: i
		};
	}
	integrals(e, { remaining: t }, n) {
		let r = n ? n.dcos : ne, i = n ? n.dsin : re;
		for (let n = 0; n <= 128; n++) {
			let a = e * t[n];
			r[n] = Math.cos(a), i[n] = Math.sin(a);
		}
		return {
			totalCos: G(r, n ? n.cos : null),
			totalSin: G(i, n ? n.sin : null)
		};
	}
	junctionRatio(e, t, n, r) {
		let i = this.samples(t, n), a = this.integrals(e, i, r);
		return this.capMetrics(1, e, i.lambda, a).junction;
	}
	capMetrics(e, t, n, { totalCos: r, totalSin: i }) {
		let a = e / (Math.cos(t) + n * t * i);
		return {
			radius: a,
			junction: a * (1 - Math.sin(t) + n * t * r)
		};
	}
	fitEasing(e, t, n, r, i = 3) {
		if (n <= 0) return {
			beta: 0,
			exponent: r
		};
		if (t <= e) return {
			beta: n,
			exponent: 1
		};
		if (r <= 1) return {
			beta: n,
			exponent: r
		};
		let a = t / e, o = `${n},${r},${i}`, s = this.fitsAsAsked(n, r, i);
		if (a >= s) return {
			beta: n,
			exponent: r
		};
		let c = `${a},${o}`, l = N.get(c);
		if (l) return l;
		let u = r - 1, d = (e, t) => this.junctionRatio(n, 1 + e, i, t), f = i === 3 ? 2 : 0, p = P.get(o) ?? z(P, 16, o, this.fitTable(u, s, f, d)), m = 1e-4 * u, { segment: h, offset: g } = this.fitGuess(p, a), _ = p.offsets[h], v = p.offsets[h + 1], y = p.ratios[h] - a, b = p.ratios[h + 1] - a, x = g - m / 2, S = null, C = X[0];
		for (let e = 0; e < 40; e++) {
			(!(x > _ && x < v) || e > 3) && (x = (_ + v) / 2);
			let t = d(x, C) - a;
			t <= 0 ? (_ = x, y = t, [S, C] = [C, S ?? X[1]]) : (v = x, b = t);
			let n = -y * (v - _) / (b - y);
			if (_ > 0 && (n <= m || v - _ <= m)) break;
			x = _ + n - m / 2;
		}
		let w = {
			beta: n,
			exponent: 1 + _
		};
		return S && this.keepTransition(w.beta, w.exponent, i, S), z(N, M, c, w);
	}
	fitsAsAsked(e, t, n) {
		let r = `${e},${t},${n}`;
		return j.get(r) ?? z(j, M, r, this.junctionRatio(e, t, n));
	}
	keepTransition(e, t, n, r) {
		let i = `${e},${t},${n}`;
		if (R.has(i)) return;
		let a = this.samples(t, n), o = {
			cos: r.cos.slice(),
			sin: r.sin.slice(),
			dcos: r.dcos.slice(),
			dsin: r.dsin.slice()
		}, s = {
			...o,
			totalCos: o.cos[128],
			totalSin: o.sin[128]
		}, { radius: c } = this.capMetrics(1, e, a.lambda, s);
		z(R, F, i, {
			samples: a,
			fresnel: s,
			radiusRatio: c
		});
	}
	fitTable(e, t, n, r) {
		let i = [];
		for (let t = 0; t <= 24; t++) i.push(e * t / 24);
		let a = (t, n) => Math.abs(t - n) <= 1e-9 * e;
		n > 0 && n < e && !i.some((e) => a(e, n)) && (i.push(n), i.sort((e, t) => e - t));
		let o = i.length - 1, s = i.map((e, n) => n === 0 ? 1 : n === o ? t : r(e)), c = [];
		for (let e = 0; e < o; e++) c.push((i[e + 1] - i[e]) / (s[e + 1] - s[e]));
		let l = [], u = [];
		for (let e = 0; e <= o; e++) {
			let t = c[e - 1], r = c[e], o = t !== void 0 && r !== void 0 && !a(i[e], n) ? t * r <= 0 ? 0 : 2 / (1 / t + 1 / r) : void 0;
			l.push(o ?? t ?? 0), u.push(o ?? r ?? 0);
		}
		return {
			offsets: i,
			ratios: s,
			slopesIn: l,
			slopesOut: u
		};
	}
	fitGuess(e, t) {
		let { offsets: n, ratios: r, slopesIn: i, slopesOut: a } = e, o = n.length - 2, s = 0;
		for (; s < o && r[s + 1] < t;) s++;
		if (s === 0) {
			let e = n[1], i = n[2] ?? e, a = r[1] - 1, o = (r[2] ?? r[1]) - 1, c = i === e ? 0 : (o / i - a / e) / (i - e), l = a / e - c * e, u = t - 1, d = l * l + 4 * c * u;
			return {
				segment: s,
				offset: d > 0 ? 2 * u / (l + Math.sqrt(d)) : u / l
			};
		}
		let c = r[s], l = r[s + 1] - c, u = (t - c) / l, d = u * u, f = d * u;
		return {
			segment: s,
			offset: (2 * f - 3 * d + 1) * n[s] + (f - 2 * d + u) * l * a[s] + (-2 * f + 3 * d) * n[s + 1] + (f - d) * l * i[s + 1]
		};
	}
	quadrant(e, t, n, r = 3) {
		if (n === 1 && t > 0) return this.quadrant(e, 0, n, r);
		let i = t > 0 ? this.transition(t, n, r) : null, a = i ? i.radiusRatio * e : e, o = [], s = Math.PI / 2 - t, c = Math.min(Math.max(Math.ceil(s / Math.sqrt(8 * Z / a)), 4), 256);
		for (let t = 0; t <= c; t++) {
			let n = Math.PI + s * t / c;
			o.push({
				x: a + a * Math.cos(n),
				y: e + a * Math.sin(n)
			});
		}
		if (!i) return o;
		let { samples: l, fresnel: u } = i;
		if (l.lambda <= 0) return o;
		let d = l.lambda * a * t, f = o[o.length - 1], p = (e) => ({
			x: f.x + d * K(u.cos, u.dcos, e),
			y: f.y - d * K(u.sin, u.dsin, e)
		}), m = (e) => this.remainingAt(l, n, r, e), h = 0, g = m(0), _ = (e) => d * ((e - h) / 512) * t * (g - m(e)) >= 8 * Z;
		for (;;) {
			let e = (Math.floor(h / 4) + 1) * 4;
			for (; e < 512 && !_(e);) e += 4;
			let t = Math.max(h + 1, e - 4 + 1);
			for (; t < e && !_(t);) t++;
			if (t >= 512) break;
			o.push(p(t)), h = t, g = m(t);
		}
		return o.push(p(512)), o;
	}
	paint(e, t, n) {
		let { width: r, height: i } = t;
		if (r <= 0 || i <= 0) return;
		e.fillStyle = `#000`, e.beginPath();
		let a = Math.max(this.resolveLength(n, S), 0);
		if (a <= 0 && this.resolveSide(n) === `auto`) {
			this.traceBox(e, r, i, n), e.closePath(), e.fill();
			return;
		}
		let o = a > 0 ? this.offsetOutline(this.boxOutline(r, i, n), -a) : this.boxOutline(r, i, n);
		for (let t = 0; t < o.length; t++) {
			let n = o[t];
			t === 0 ? e.moveTo(n.x, n.y) : e.lineTo(n.x, n.y);
		}
		e.closePath(), e.fill();
	}
	transition(e, t, n) {
		let r = `${e},${t},${n}`, i = R.get(r);
		if (i) return i;
		let a = this.samples(t, n), o = this.fresnel(e, a), { radius: s } = this.capMetrics(1, e, a.lambda, o);
		return z(R, F, r, {
			samples: a,
			fresnel: o,
			radiusRatio: s
		});
	}
	fittedQuadrant(e, t, n, r, i = 3) {
		let a = t / 2, o = this.fitEasing(a, e / 2, n, r, i), s = `${t},${o.beta},${o.exponent},${i}`, c = A.get(s);
		if (c) return c;
		let l = this.quadrant(a, o.beta, o.exponent, i);
		return A.size >= k && A.delete(A.keys().next().value), A.set(s, l), l;
	}
	capAt(e, t, n, r) {
		let i = n === 1 || t <= 0 ? null : this.transition(t, n, r), a = i ? i.radiusRatio * e : e, o = (t) => ({
			x: a - a * Math.cos(t),
			y: e - a * Math.sin(t)
		});
		if (!i || i.samples.lambda <= 0) return o;
		let { samples: s, fresnel: c } = i, { remaining: l } = this.profile(n, r), u = Math.PI / 2 - t, d = s.lambda * a * t, f = o(u);
		return (e) => {
			if (e <= u) return o(e);
			let n = 512;
			if (e < Math.PI / 2) {
				let r = (Math.PI / 2 - e) / t, i = 0, a = 512;
				for (; a - i > 1;) {
					let e = i + a >> 1;
					l[e] >= r ? i = e : a = e;
				}
				let o = l[i] - l[a];
				n = Math.min(i + (o > 0 ? (l[i] - r) / o : 0), 512);
			}
			return {
				x: f.x + d * K(c.cos, c.dcos, n),
				y: f.y - d * K(c.sin, c.dsin, n)
			};
		};
	}
	roundnessExponent(e, t, n) {
		let r = `${e},${t},${n}`, i = ae.get(r);
		if (i !== void 0) return i;
		let a = this.capAt(1, e, t, n)(Math.PI / 4), o = (a.x + a.y) / Math.SQRT2, s = this.fitsAsAsked(e, t, n), c = -1 / Math.log2(1 - o / (Math.SQRT2 * s)), l = ie[n];
		return z(ae, M, r, Number.isFinite(c) ? Math.max(c, l) : l);
	}
	corner(e, t, n, r, i, a = 3) {
		let o = Math.min(t, n);
		if (o >= t) return this.fittedQuadrant(2 * e, 2 * t, r, i, a);
		if (!(o > 0)) return [{
			x: 0,
			y: 0
		}];
		let s = this.cornerCurve(e, t, n, r, i, a);
		if (!s) return this.fittedQuadrant(2 * e, 2 * o, r, i, a);
		let c = A.get(s.key);
		if (c) return c;
		let l = [];
		for (let e = 0; e <= Q; e++) l.push(s.at(Math.PI / 2 * (e / Q)));
		let u = [l[0]], d = 0;
		for (let e = 2; e <= Q; e++) {
			let t = l[d], n = l[e], r = n.x - t.x, i = n.y - t.y, a = Z * Math.sqrt(r * r + i * i);
			for (let n = d + 1; n < e; n++) {
				let o = l[n];
				if (Math.abs((o.x - t.x) * i - (o.y - t.y) * r) > a) {
					d = e - 1, u.push(l[d]);
					break;
				}
			}
		}
		return u.push(l[Q]), A.size >= k && A.delete(A.keys().next().value), A.set(s.key, u), u;
	}
	cornerCurve(e, t, n, r, i, a = 3) {
		let o = Math.min(t, n);
		if (o >= t || !(o > 0) || r <= 0 || i <= 1) return null;
		let s = o * this.fitsAsAsked(r, i, a);
		if (s <= o) return null;
		let c = this.fitEasing(o, e, r, i, a), l = Math.min(Math.max((t - o) / (s - o), 0), 1), u = Math.min(s, e), d = this.roundnessExponent(r, i, a), f = d / (2 * d - 2), p = this.capAt(o, c.beta, c.exponent, a), m = (e, t, n) => ({
			x: (1 - l) * e.x + l * t,
			y: (1 - l) * e.y + l * n
		});
		return {
			key: `max:${o},${c.beta},${c.exponent},${l},${u},${i},${a}`,
			at: (e) => {
				if (e <= 0) return {
					x: 0,
					y: (1 - l) * o + l * u
				};
				if (e >= Math.PI / 2) return {
					x: m(p(Math.PI / 2), u, 0).x,
					y: 0
				};
				let t = Math.atan(Math.tan(e) ** f);
				return m(p(e), u - u * Math.cos(t) ** (2 / d), u - u * Math.sin(t) ** (2 / d));
			}
		};
	}
	boxOutline(e, t, n) {
		let r = this.resolveSide(n);
		if (r !== `auto`) return this.sideOutline(e, t, r, n);
		let i = t > e, a = i ? t : e, o = i ? e : t, s = this.outline(a, o, this.corner(a / 2, o / 2, this.resolveMaxRadius(n), this.resolveAngle(n), this.resolveExponent(n), this.resolveContinuity(n)));
		return i ? s.map((e) => ({
			x: e.y,
			y: e.x
		})) : s;
	}
	traceBox(e, t, n, r) {
		let i = n > t, a = i ? n : t, o = i ? t : n, s = this.corner(a / 2, o / 2, this.resolveMaxRadius(r), this.resolveAngle(r), this.resolveExponent(r), this.resolveContinuity(r)), c = s.length, l = !0, u = (t, n) => {
			let r = i ? n : t, a = i ? t : n;
			l ? e.moveTo(r, a) : e.lineTo(r, a), l = !1;
		};
		for (let e = 0; e < c; e++) u(s[e].x, s[e].y);
		for (let e = c - 1; e >= 0; e--) u(a - s[e].x, s[e].y);
		for (let e = 0; e < c; e++) u(a - s[e].x, o - s[e].y);
		for (let e = c - 1; e >= 0; e--) u(s[e].x, o - s[e].y);
	}
	sideOutline(e, t, n, r) {
		let i = n === `t` || n === `b`, a = i ? t : e, o = i ? e : t, s = this.resolveAngle(r), c = this.resolveExponent(r), l = this.resolveContinuity(r), u = this.resolveMaxRadius(r), d = Math.max(o / 2, a), f = Math.min(o / 2, a), p = o / 2 <= a ? this.corner(d, f, u, s, c, l) : this.corner(d, f, u, s, c, l).map((e) => ({
			x: e.y,
			y: e.x
		})).reverse(), m = [
			...p,
			{
				x: a,
				y: 0
			},
			{
				x: a,
				y: o
			},
			...[...p].reverse().map((e) => ({
				x: e.x,
				y: o - e.y
			}))
		];
		return n === `l` ? m : n === `r` ? m.map((e) => ({
			x: a - e.x,
			y: e.y
		})) : n === `t` ? m.map((e) => ({
			x: e.y,
			y: e.x
		})) : m.map((e) => ({
			x: e.y,
			y: a - e.x
		}));
	}
	shadowCutout(e) {
		return this.offsetOutline(e, -.5);
	}
	offsetOutline(e, t) {
		let n = [];
		for (let t of e) {
			let e = n[n.length - 1];
			(!e || Math.abs(e.x - t.x) > 1e-6 || Math.abs(e.y - t.y) > 1e-6) && n.push(t);
		}
		let r = n[0], i = n[n.length - 1];
		if (r && i && n.length > 1 && Math.abs(r.x - i.x) < 1e-6 && Math.abs(r.y - i.y) < 1e-6 && n.pop(), t === 0) return n;
		let a = 0;
		for (let e = 0; e < n.length; e++) {
			let t = n[e], r = n[(e + 1) % n.length];
			a += t.x * r.y - r.x * t.y;
		}
		let o = a > 0 ? t : -t, s = n.length, c = q(s), l = q(s);
		for (let e = 0; e < s; e++) {
			let t = n[e], r = n[(e + 1) % s], i = r.x - t.x, a = r.y - t.y, o = Math.sqrt(i * i + a * a) || 1;
			c[e] = a / o, l[e] = -i / o;
		}
		let u = [];
		for (let e = 0; e < s; e++) {
			let t = (e - 1 + s) % s, r = c[t] + c[e], i = l[t] + l[e], a = c[t] * c[e] + l[t] * l[e], d = a < Math.SQRT1_2 ? o / (1 + a) : o / (Math.sqrt(r * r + i * i) || 1), f = n[e];
			u.push({
				x: f.x + r * d,
				y: f.y + i * d
			});
		}
		return u;
	}
	resolveLength(e, t) {
		let n = Number.parseFloat(e?.get(t)?.toString() ?? ``);
		return Number.isFinite(n) ? n : 0;
	}
	decorationOutset(e) {
		return Math.max(this.resolveLength(e, x), 0);
	}
	decorationShadows(e) {
		let t = (e?.get(y)?.toString().trim() || e?.get(v)?.toString().trim()) ?? ``;
		if (t === `` || t === `none`) return [];
		let n = [];
		for (let r of E(t, `,`)) {
			let t = E(r, ` `);
			if (t.includes(`inset`)) continue;
			let i = [], a = [];
			for (let e of t) {
				let t = ee(e);
				t === null ? a.push(e) : i.push(t);
			}
			let [o = 0, s = 0, c = 0, l = 0] = i, u = D(a.join(` `) || `currentColor`, e);
			i.length < 2 || !O(u) || n.push({
				x: o,
				y: s,
				blur: Math.max(c, 0),
				spread: l,
				color: u
			});
		}
		return n;
	}
	decorationBands(e) {
		let t = (t) => D(e?.get(t)?.toString().trim() ?? ``, e), n = (e) => e !== `` && O(e), i = [], a = this.resolveLength(e, o), v = t(s), y = this.resolveDash(e?.get(r)?.toString(), a);
		a > 0 && n(v) && y !== null && i.push({
			from: -a,
			to: 0,
			color: v,
			dash: y
		});
		let b = this.resolveLength(e, g), x = t(_);
		b > 0 && n(x) && i.push({
			from: -a - b,
			to: -a,
			color: x,
			dash: []
		});
		let S = this.resolveLength(e, f);
		if (S > 0) {
			let r = Math.max(this.resolveLength(e, m), 0), a = t(h);
			r > 0 && n(a) && i.push({
				from: 0,
				to: r,
				color: a,
				dash: []
			});
			let o = t(p);
			n(o) && i.push({
				from: r,
				to: r + S,
				color: o,
				dash: []
			});
		}
		let C = this.resolveLength(e, c), w = t(u), T = this.resolveDash(e?.get(d)?.toString(), C);
		if (C > 0 && n(w) && T !== null) {
			let t = this.resolveLength(e, l);
			i.push({
				from: t,
				to: t + C,
				color: w,
				dash: T
			});
		}
		return this.closeSeams(i);
	}
	closeSeams(e) {
		let t = (e, t) => Math.abs(e - t) < 1e-6, n = e.some((e) => e.from < 0 && t(e.to, 0));
		return e.map((r, i) => {
			let { from: a, to: o } = r;
			for (let n of e.slice(i + 1)) n.dash.length > 0 || (t(n.from, r.to) && (o = r.to + T), t(n.to, r.from) && (a = r.from - T));
			return !n && t(r.from, 0) && (a = -.5), {
				...r,
				from: a,
				to: o
			};
		});
	}
	outline(e, t, n) {
		let r = [...n].reverse();
		return [
			...n,
			...r.map((t) => ({
				x: e - t.x,
				y: t.y
			})),
			...n.map((n) => ({
				x: e - n.x,
				y: t - n.y
			})),
			...r.map((e) => ({
				x: e.x,
				y: t - e.y
			}))
		];
	}
}, oe = class extends $ {
	static get inputProperties() {
		return [
			t,
			n,
			i,
			a,
			C,
			...w
		];
	}
	paint(e, t, n) {
		let r = this.decorationBands(n), i = this.decorationShadows(n);
		if (r.length === 0 && i.length === 0) return;
		let a = this.decorationOutset(n), o = t.width - 2 * a, s = t.height - 2 * a;
		if (o <= 0 || s <= 0) return;
		let c = this.boxOutline(o, s, n).map((e) => ({
			x: e.x + a,
			y: e.y + a
		})), l = (t, n = 0, r = 0) => {
			e.beginPath(), t.forEach((t, i) => i === 0 ? e.moveTo(t.x + n, t.y + r) : e.lineTo(t.x + n, t.y + r)), e.closePath();
		};
		if (i.length > 0) {
			e.save(), e.beginPath(), e.rect(0, 0, t.width, t.height), this.shadowCutout(c).forEach((t, n) => n === 0 ? e.moveTo(t.x, t.y) : e.lineTo(t.x, t.y)), e.closePath(), e.clip(`evenodd`);
			let r = t.width + t.height + 1e3, a = typeof e.filter == `string`, o = this.resolveLength(n, b) || 1;
			for (let t of [...i].reverse()) {
				if (t.blur === 0 || a) {
					a && (e.filter = t.blur > 0 ? `blur(${t.blur * o / 2}px)` : `none`), e.shadowColor = `transparent`, e.fillStyle = t.color, l(this.offsetOutline(c, t.spread), t.x, t.y), e.fill();
					continue;
				}
				e.fillStyle = `#000`, e.shadowColor = t.color, e.shadowBlur = t.blur, e.shadowOffsetX = t.x + r, e.shadowOffsetY = t.y, l(this.offsetOutline(c, t.spread), -r), e.fill();
			}
			e.restore();
		}
		e.lineJoin = `miter`;
		for (let t of r) l(this.offsetOutline(c, (t.from + t.to) / 2)), e.strokeStyle = t.color, e.lineWidth = t.to - t.from, e.setLineDash(t.dash), e.stroke();
	}
};
typeof registerPaint < `u` && (registerPaint(`pill-shape`, $), registerPaint(`pill-decoration`, oe));
export { oe as decorationDef, $ as paintDef };

//# sourceMappingURL=pill-shape.worklet.mjs.map