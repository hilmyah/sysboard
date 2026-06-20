'use strict';

// ── Utilities ────────────────────────────────────────────────────────────────

const $ = id => document.getElementById(id);

function fmtB(b) {
  if (!b || b <= 0) return '0 B';
  const u = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let i = 0;
  while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
  return b.toFixed(i ? 1 : 0) + ' ' + u[i];
}

function fmtRate(b) {
  if (!b || b <= 0) return '0 B/s';
  const u = ['B/s', 'KiB/s', 'MiB/s', 'GiB/s'];
  let i = 0;
  while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
  return b.toFixed(i ? 1 : 0) + ' ' + u[i];
}

function barColor(p) {
  return p >= 90 ? 'var(--danger)' : p >= 70 ? 'var(--warn)' : 'var(--accent)';
}

function pillClass(a) {
  if (a === 'active' || a === 'running') return 'p-active';
  if (a === 'failed' || a === 'exited')  return 'p-failed';
  if (a === 'inactive')                  return 'p-inactive';
  return 'p-other';
}

// Escape HTML to prevent XSS when injecting user-controlled strings into innerHTML.
function esc(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// ── Auth helpers ─────────────────────────────────────────────────────────────

const tok  = () => sessionStorage.getItem('sb_tok') || '';
const hdrs = () => ({ 'Content-Type': 'application/json', 'X-Auth-Token': tok() });

async function api(path, opts = {}) {
  const res = await fetch(path, { headers: hdrs(), ...opts });
  if (res.status === 401) {
    sessionStorage.removeItem('sb_tok');
    location.reload();
    throw new Error('unauthorized');
  }
  return res;
}

// ── Toast ────────────────────────────────────────────────────────────────────

function toast(msg, type = 'info') {
  const d = document.createElement('div');
  d.className = 'toast' + (type === 'err' ? ' err' : type === 'warn' ? ' warn' : '');
  d.textContent = msg;
  $('toasts').appendChild(d);
  setTimeout(() => d.remove(), 3500);
}

// ── Login ────────────────────────────────────────────────────────────────────

$('login-btn').onclick = doLogin;
$('tok-in').onkeydown = e => { if (e.key === 'Enter') doLogin(); };

async function doLogin() {
  const token = $('tok-in').value.trim();
  if (!token) return;
  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    });
    if (res.ok) {
      sessionStorage.setItem('sb_tok', token);
      startApp();
    } else {
      $('login-err').textContent = 'Invalid token';
    }
  } catch (e) {
    $('login-err').textContent = 'Connection error';
  }
}

// ── Tabs ─────────────────────────────────────────────────────────────────────

let activeTab = 'overview';

document.querySelectorAll('.nav-btn').forEach(btn => {
  btn.onclick = () => switchTab(btn.dataset.tab);
});

function switchTab(name) {
  activeTab = name;
  document.querySelectorAll('.nav-btn').forEach(b =>
    b.classList.toggle('on', b.dataset.tab === name)
  );
  document.querySelectorAll('.tab').forEach(t =>
    t.classList.toggle('on', t.id === 'tab-' + name)
  );
  if (name === 'services' && !svcs.length) fetchServices();
  if (name === 'containers') fetchContainers();
  if (name === 'processes')  fetchProcesses();
}

// ── Logout ───────────────────────────────────────────────────────────────────

$('logout-btn').onclick = () => {
  sessionStorage.removeItem('sb_tok');
  location.reload();
};

// ── Clock ────────────────────────────────────────────────────────────────────

function tickClock() {
  $('clock').textContent = new Date().toTimeString().slice(0, 8);
}

// ── Metrics ──────────────────────────────────────────────────────────────────

async function fetchMetrics() {
  try {
    const res = await api('/api/metrics');
    if (!res.ok) return;
    const m = await res.json();

    const cpu = m.cpu_percent || 0;
    const ram = m.ram_percent || 0;

    $('cpu-val').textContent = cpu.toFixed(1) + '%';
    $('load-val').textContent = 'Load: ' + (m.load_avg || '--');
    $('cpu-bar').style.cssText = `width:${Math.min(cpu, 100)}%;background:${barColor(cpu)}`;
    $('h-cpu').textContent = cpu.toFixed(1) + '%';

    $('ram-val').textContent = ram.toFixed(1) + '%';
    $('ram-sub').textContent = fmtB(m.ram_used) + ' / ' + fmtB(m.ram_total);
    $('ram-bar').style.cssText = `width:${Math.min(ram, 100)}%;background:${barColor(ram)}`;
    $('h-ram').textContent = ram.toFixed(1) + '%';

    if (m.cpu_temp > 0) {
      $('temp-val').textContent = m.cpu_temp.toFixed(1) + '\u00b0C';
      $('temp-sub').textContent = m.cpu_temp > 70 ? 'High' : 'Normal';
      $('h-temp').textContent = m.cpu_temp.toFixed(1) + '\u00b0C';
      $('h-temp-wrap').style.display = '';
    } else {
      $('temp-val').textContent = 'N/A';
      $('temp-sub').textContent = 'No sensor';
      $('h-temp-wrap').style.display = 'none';
    }

    $('uptime-val').textContent = m.uptime || '--';
    $('ts-val').textContent = 'Updated: ' + (m.timestamp || '--');

    $('rx-rate').textContent  = fmtRate(m.net_rx_rate);
    $('rx-total').textContent = 'Total: ' + fmtB(m.net_rx_bytes);
    $('tx-rate').textContent  = fmtRate(m.net_tx_rate);
    $('tx-total').textContent = 'Total: ' + fmtB(m.net_tx_bytes);

    const dg = $('disk-grid');
    dg.innerHTML = '';
    (m.disks || []).forEach(d => {
      const pct = d.percent ? d.percent.toFixed(1) : '0';
      const div = document.createElement('div');
      div.className = 'disk';
      div.innerHTML =
        `<div class="disk-mount">${esc(d.mount)}</div>` +
        `<div class="stat-sub">${fmtB(d.used)} / ${fmtB(d.total)}</div>` +
        `<div class="bar" style="margin-top:6px">` +
          `<div class="bar-fill" style="width:${Math.min(d.percent || 0, 100)}%;background:${barColor(d.percent)}"></div>` +
        `</div>` +
        `<div class="stat-sub" style="margin-top:3px">${pct}% used</div>`;
      dg.appendChild(div);
    });
  } catch (e) {}
}

// ── Processes ────────────────────────────────────────────────────────────────

let procs = [];

async function fetchProcesses() {
  try {
    const res = await api('/api/processes');
    if (!res.ok) return;
    procs = await res.json() || [];
    renderProcs();
  } catch (e) {}
}

function renderProcs() {
  const filter = $('proc-filter').value.toLowerCase();
  const list = filter ? procs.filter(p => p.name.toLowerCase().includes(filter)) : procs;
  $('proc-count').textContent = list.length + ' processes';
  const tb = $('proc-tbody');
  tb.innerHTML = '';
  list.forEach(p => {
    const cpu = Math.min(p.cpu_pct || 0, 100);
    const barW = Math.round(cpu * 0.6); // max 60px at 100%
    const tr = document.createElement('tr');
    tr.innerHTML =
      `<td style="color:#fff;font-weight:600">${esc(p.name)}</td>` +
      `<td style="color:var(--text2)">${p.pid}</td>` +
      `<td><span class="cpu-bar" style="width:${barW}px"></span> ${cpu.toFixed(1)}%</td>` +
      `<td>${fmtB(p.mem_bytes)}</td>` +
      `<td class="hide-m" style="color:var(--text2)">${esc(p.user || '')}</td>` +
      `<td class="hide-m">` +
        `<span class="pill ${p.state === 'S' || p.state === 'R' ? 'p-active' : 'p-other'}">${esc(p.state || '')}</span>` +
      `</td>`;
    tb.appendChild(tr);
  });
  if (!list.length) {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td colspan="6" style="text-align:center;color:var(--text2);padding:20px">No processes</td>';
    tb.appendChild(tr);
  }
}

$('proc-filter').oninput = renderProcs;

// ── Services ─────────────────────────────────────────────────────────────────

let svcs = [], svcSort = 'name', svcAsc = true;

document.querySelectorAll('.sort-btn[data-sort]').forEach(btn => {
  btn.onclick = () => {
    const key = btn.dataset.sort;
    if (svcSort === key) {
      svcAsc = !svcAsc;
    } else {
      svcSort = key;
      svcAsc = key === 'name';
    }
    document.querySelectorAll('.sort-btn[data-sort]').forEach(b =>
      b.classList.toggle('on', b.dataset.sort === svcSort)
    );
    renderServices();
  };
});

async function fetchServices() {
  try {
    const res = await api('/api/services');
    if (!res.ok) return;
    svcs = await res.json() || [];
    $('svc-count').textContent = svcs.length;
    renderServices();
  } catch (e) {}
}

function renderServices() {
  const filter = $('svc-filter').value.toLowerCase();
  let list = filter
    ? svcs.filter(s => s.name.toLowerCase().includes(filter) || (s.desc || '').toLowerCase().includes(filter))
    : [...svcs];

  list.sort((a, b) => {
    let v = 0;
    if (svcSort === 'name')   v = a.name.localeCompare(b.name);
    if (svcSort === 'mem')    v = (a.mem_bytes || 0) - (b.mem_bytes || 0);
    if (svcSort === 'status') {
      const o = { active: 0, failed: 1, inactive: 2 };
      v = (o[a.active] ?? 3) - (o[b.active] ?? 3);
    }
    return svcAsc ? v : -v;
  });

  $('svc-cnt').textContent = list.length + ' services';
  const tb = $('svc-tbody');
  tb.innerHTML = '';
  list.forEach(s => {
    const tr = document.createElement('tr');
    tr.innerHTML =
      `<td>` +
        `<div style="color:#fff;font-weight:600">${esc(s.name)}</div>` +
        `<div style="font-size:10px;color:var(--text2);max-width:180px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(s.desc || '')}</div>` +
      `</td>` +
      `<td class="hide-m">${s.mem_bytes > 0 ? fmtB(s.mem_bytes) : '<span style="color:var(--text2)">--</span>'}</td>` +
      `<td class="hide-m" style="color:var(--text2)">${esc(s.uptime_str || '--')}</td>` +
      `<td><span class="pill ${pillClass(s.active)}">${esc(s.sub || s.active)}</span></td>` +
      `<td><div class="btns">` +
        `<button class="btn btn-g" onclick="svcAction('${esc(s.name)}','start')">start</button>` +
        `<button class="btn btn-r" onclick="svcAction('${esc(s.name)}','stop')">stop</button>` +
        `<button class="btn btn-b" onclick="svcAction('${esc(s.name)}','restart')">&#8635;</button>` +
      `</div></td>`;
    tb.appendChild(tr);
  });
  if (!list.length) {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td colspan="5" style="text-align:center;color:var(--text2);padding:20px">No services</td>';
    tb.appendChild(tr);
  }
}

$('svc-filter').oninput = renderServices;

async function svcAction(svc, action) {
  try {
    const res = await api('/api/services/action', {
      method: 'POST',
      body: JSON.stringify({ service: svc, action }),
    });
    const d = await res.json();
    if (d.error) toast(d.error, 'err');
    else {
      toast(svc + ': ' + action);
      setTimeout(fetchServices, 1000);
    }
  } catch (e) {}
}

// ── Containers ───────────────────────────────────────────────────────────────

let ctrs = [];

async function fetchContainers() {
  try {
    const res = await api('/api/containers');
    if (!res.ok) return;
    ctrs = await res.json() || [];
    $('ctr-count').textContent = ctrs.length;
    renderContainers();
  } catch (e) {}
}

function renderContainers() {
  $('ctr-cnt').textContent = ctrs.length + ' containers';
  const tb = $('ctr-tbody');
  tb.innerHTML = '';
  ctrs.forEach(c => {
    const tr = document.createElement('tr');
    tr.innerHTML =
      `<td style="color:#fff;font-weight:600">${esc(c.name)}</td>` +
      `<td class="hide-m wrap" style="color:var(--text2)">${esc(c.image)}</td>` +
      `<td><span class="pill ${pillClass(c.state)}">${esc(c.state)}</span></td>` +
      `<td class="hide-m" style="color:var(--text2)">${esc(c.engine)}</td>` +
      `<td><div class="btns">` +
        `<button class="btn btn-g" onclick="ctrAction('${esc(c.id)}','${esc(c.engine)}','start')">start</button>` +
        `<button class="btn btn-r" onclick="ctrAction('${esc(c.id)}','${esc(c.engine)}','stop')">stop</button>` +
        `<button class="btn btn-b" onclick="ctrAction('${esc(c.id)}','${esc(c.engine)}','restart')">&#8635;</button>` +
      `</div></td>`;
    tb.appendChild(tr);
  });
  if (!ctrs.length) {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td colspan="5" style="text-align:center;color:var(--text2);padding:20px">No containers detected</td>';
    tb.appendChild(tr);
  }
}

$('ctr-refresh').onclick = fetchContainers;

async function ctrAction(id, engine, action) {
  try {
    await api('/api/containers/action', {
      method: 'POST',
      body: JSON.stringify({ id, engine, action }),
    });
    toast(id.slice(0, 8) + ': ' + action);
    setTimeout(fetchContainers, 1000);
  } catch (e) {}
}

// ── Boot ─────────────────────────────────────────────────────────────────────

async function startApp() {
  $('login').style.display  = 'none';
  $('app').style.display    = 'flex';

  tickClock();
  setInterval(tickClock, 1000);

  fetchMetrics();
  fetchServices();
  setInterval(fetchMetrics,  4000);
  setInterval(fetchServices, 12000);
  setInterval(() => { if (activeTab === 'processes')  fetchProcesses();  }, 5000);
  setInterval(() => { if (activeTab === 'containers') fetchContainers(); }, 10000);
}

// Auto-login if a valid token is already stored in sessionStorage.
window.addEventListener('DOMContentLoaded', () => {
  const t = tok();
  if (t) {
    fetch('/api/metrics', { headers: hdrs() })
      .then(r => {
        if (r.ok) startApp();
        else sessionStorage.removeItem('sb_tok');
      })
      .catch(() => sessionStorage.removeItem('sb_tok'));
  }
});
