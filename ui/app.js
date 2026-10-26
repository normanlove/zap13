const MIN = 5, MAX = 120;
const sliders = [...document.querySelectorAll('.slider')].map(el => ({
  input: el.querySelector('input'), label: el.querySelector('.val b'), steps: el.querySelectorAll('.step')
}));
const $ = id => document.getElementById(id);
const fmt = l => `${l[0]} / ${l[1]} / ${l[2]} W`;

function show(i, v) {
  const s = sliders[i];
  s.input.value = v;
  s.label.textContent = v;
  s.input.style.setProperty('--p', ((v - MIN) / (MAX - MIN) * 100) + '%');
}
function setVal(i, v) {
  v = Math.max(MIN, Math.min(MAX, Math.round(v)));
  show(i, v);
  // keep SPL <= sPPT <= fPPT
  sliders.forEach((s, j) => {
    const cur = +s.input.value;
    if (j > i && cur < v) show(j, v);
    if (j < i && cur > v) show(j, v);
  });
}
const values = () => sliders.map(s => +s.input.value);

sliders.forEach((s, i) => {
  s.input.addEventListener('input', () => { setVal(i, +s.input.value); markPreset(); });
  s.steps.forEach(btn => {
    const d = +btn.dataset.d;
    let t1, t2;
    const stop = () => { clearTimeout(t1); clearInterval(t2); };
    btn.addEventListener('pointerdown', e => {
      e.preventDefault(); btn.setPointerCapture(e.pointerId);
      const step = () => { setVal(i, +s.input.value + d); markPreset(); };
      step();
      t1 = setTimeout(() => { t2 = setInterval(step, 70); }, 420);
    });
    ['pointerup', 'pointercancel', 'lostpointercapture'].forEach(ev => btn.addEventListener(ev, stop));
  });
});

// Go functions are bound onto window by the webview
const call = (name, ...args) => (window[name] ? window[name](...args) : Promise.resolve());
$('apply').onclick = () => call('applyLimits', ...values());
$('auto').onclick = () => call('backToAuto');
$('min').onclick = () => call('minimise');
$('close').onclick = () => call('closeApp');
let defaults = [22, 26, 30];
$('def').onclick = () => { defaults.forEach((v, i) => show(i, v)); markPreset(); };

// no title bar, so the header is the drag handle
$('drag').addEventListener('pointerdown', e => { if (e.button === 0) call('startDrag'); });

// presets only move the sliders, Apply is still needed
const PRESETS = [
  ['Eco', [15, 18, 22]], ['Quiet', [30, 35, 40]], ['Balanced', [45, 52, 60]],
  ['Gaming', [65, 72, 80]], ['Max', [90, 95, 100]],
];
PRESETS.forEach(([name, l]) => {
  const b = document.createElement('button');
  b.className = 'preset';
  b.innerHTML = `<b>${name}</b><span>${l[0]} W</span>`;
  b.onclick = () => { l.forEach((v, i) => show(i, v)); markPreset(); };
  b.dataset.l = l.join(',');
  $('presets').appendChild(b);
});
function markPreset() {
  const v = values().join(',');
  document.querySelectorAll('.preset').forEach(b => b.classList.toggle('on', b.dataset.l === v));
}

function onStatus(st) {
  $('pill').className = 'pill ' + (st.manual ? 'manual' : 'auto');
  $('pillText').textContent = st.manual ? 'MANUAL' : 'AUTO';
  const pct = st.percent;
  $('pct').textContent = pct == null ? '—' : pct + '%';
  const flow = $('flow');
  let cls = '';
  if (st.chargeW > 0.1) { flow.textContent = `Charging  +${st.chargeW.toFixed(1)} W`; cls = 'charging'; }
  else if (st.dischargeW > 0.1) { flow.textContent = `Draining  −${st.dischargeW.toFixed(1)} W`; cls = 'draining'; }
  else flow.textContent = st.ac ? 'Plugged in, holding' : 'Idle';
  flow.className = 'flow ' + cls;
  const fill = $('barFill');
  fill.style.width = (pct || 0) + '%';
  fill.style.background = cls === 'charging' ? 'var(--green)' : cls === 'draining' ? 'var(--amber)' : 'var(--muted)';
  if (st.limits) { $('limMain').textContent = fmt(st.limits); $('limSub').textContent = 'SPL / sPPT / fPPT'; }
  else { $('limMain').textContent = 'Full power'; $('limSub').textContent = 'Firmware Turbo'; }
  const src = st.source === 'ac' ? 'usbc' : st.source;
  document.querySelectorAll('.card').forEach(c => c.classList.toggle('active', c.dataset.src === src));
}

// accent colour
function hexToRgb(h) { const n = parseInt(h.slice(1), 16); return [n >> 16 & 255, n >> 8 & 255, n & 255]; }
function rgbToHex(c) { return '#' + c.map(v => Math.round(Math.max(0, Math.min(255, v))).toString(16).padStart(2, '0')).join(''); }
const mix = (a, b, t) => a.map((v, i) => v + (b[i] - v) * t);
const lum = c => { const f = c.map(v => { v /= 255; return v <= .03928 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; });
                   return .2126 * f[0] + .7152 * f[1] + .0722 * f[2]; };
function toHsl([r, g, b]) {
  r /= 255; g /= 255; b /= 255;
  const mx = Math.max(r, g, b), mn = Math.min(r, g, b), l = (mx + mn) / 2, d = mx - mn;
  if (!d) return [0, 0, l];
  const s = d / (1 - Math.abs(2 * l - 1));
  const h = mx === r ? ((g - b) / d + 6) % 6 : mx === g ? (b - r) / d + 2 : (r - g) / d + 4;
  return [h * 60, s, l];
}
function fromHsl(h, s, l) {
  const k = n => (n + h / 30) % 12, a = s * Math.min(l, 1 - l);
  return [0, 8, 4].map(n => 255 * (l - a * Math.max(-1, Math.min(k(n) - 3, 9 - k(n), 1))));
}
function setAccent(hex) {
  if (!/^#[0-9a-f]{6}$/i.test(hex || '')) return;
  let c = hexToRgb(hex);
  // really dark accents disappear on the background, so brighten them a bit
  if (lum(c) < .09) {
    let [h, s, l] = toHsl(c);
    s = Math.max(s, .55);
    while (l < .9 && lum(fromHsl(h, s, l)) < .12) l += .02;
    c = fromHsl(h, s, l);
  }
  const r = document.documentElement.style;
  r.setProperty('--accent', rgbToHex(c));
  r.setProperty('--accent-rgb', c.map(Math.round).join(','));
  r.setProperty('--accent-hi', rgbToHex(mix(c, [255, 255, 255], .18)));
  r.setProperty('--accent-lo', rgbToHex(mix(c, [0, 0, 0], .16)));
  r.setProperty('--accent-bg', rgbToHex(mix([21, 23, 30], c, .14)));
  r.setProperty('--on-accent', lum(c) > .45 ? '#111318' : '#ffffff');
}

function onLog(line) {
  $('log').textContent = line;
}

// it's an app, not a browser - block reload/zoom/find
document.addEventListener('keydown', e => {
  if (e.key === 'F5' || (e.ctrlKey && 'rpf+-=0'.includes(e.key.toLowerCase()))) e.preventDefault();
});
document.addEventListener('wheel', e => { if (e.ctrlKey) e.preventDefault(); }, { passive: false });
document.addEventListener('contextmenu', e => e.preventDefault());

call('getInitial').then(d => {
  if (!d) return;
  d.sliders.forEach((v, i) => show(i, v));
  markPreset();
  $('batDetail').textContent = 'Silent · ' + fmt(d.battery);
  defaults = d.defaults;
  if (d.accent) setAccent(d.accent);
  if (d.status) onStatus(d.status);
  if (d.log) onLog(d.log);
});
