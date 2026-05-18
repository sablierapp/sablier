			// ─── Global state (updated by SSE) ─────────────────────────────
			window.__sablier = {
				instances: {},
			activeGroup: null,
			sortKey: 'name',
			sortDir: 1,
			searchQuery: '',
		};

		// ─── Debounced render ────────────────────────────────────────────
		// State updates happen immediately; the visual re-render is coalesced
		// so that a burst of events (e.g. 30 requests within 1 s) only
		// triggers a single DOM update at the end of the quiet period.
		let _renderTimer = null;
		function scheduleRender() {
			clearTimeout(_renderTimer);
			_renderTimer = setTimeout(renderInstances, 1000);
		}

		// ─── SSE ────────────────────────────────────────────────────────
		(function connectSSE() {
			const es = new EventSource('/dashboard/stream');

			es.addEventListener('instances', function(e) {
				const data = JSON.parse(e.data);
				// Replace the entire map — handles instance removals too
				const next = {};
				data.forEach(inst => {
					// Convert server-relative seconds to a client-local deadline (ms)
					if (inst.expiresInSeconds > 0) {
						inst._expiresAt = Date.now() + inst.expiresInSeconds * 1000;
					}
					next[inst.name] = inst;
				});
				window.__sablier.instances = next;
				scheduleRender();
				document.getElementById('last-ts').textContent = new Date().toLocaleTimeString();
			});

			es.addEventListener('stats', function(e) {
				updateStats(JSON.parse(e.data));
			});

			es.addEventListener('starting', function(e) {
				const list = JSON.parse(e.data);
				// Patch only the starting instances in the store so the table
				// reflects their latest state immediately, without waiting for
				// the next full 'instances' event.
				list.forEach(inst => {
					if (inst.expiresInSeconds > 0) {
						inst._expiresAt = Date.now() + inst.expiresInSeconds * 1000;
					}
					window.__sablier.instances[inst.name] = inst;
				});
				if (list.length > 0) scheduleRender();
			});

			es.addEventListener('groups', function(e) {
				window.__sablier.groups = JSON.parse(e.data);
				renderGroups(window.__sablier.groups);
			});

			es.onerror = function() {
				// reconnect handled automatically by EventSource
				document.querySelector('.live-dot').style.background = 'var(--amber)';
				document.querySelector('.live-dot').style.boxShadow = '0 0 8px var(--amber)';
			};

			es.onopen = function() {
				document.querySelector('.live-dot').style.background = 'var(--green)';
				document.querySelector('.live-dot').style.boxShadow = '0 0 8px var(--green)';
			};
		})();

		// ─── Stats update ────────────────────────────────────────────────
		function setInner(id, val) {
			const el = document.getElementById(id);
			if (el && el.textContent !== String(val)) {
				el.textContent = val;
				el.classList.add('sse-update');
				setTimeout(() => el.classList.remove('sse-update'), 400);
			}
		}

		function updateStats(s) {
			setInner('stat-total',      s.total);
			setInner('stat-active',     s.active);
			setInner('stat-errors',     s.errors);
			setInner('stat-efficiency', Math.round(s.overallEfficiency));
			const av = document.getElementById('stat-active');
			if (av) av.className = 'stat-value' + (s.active > 0 ? ' green' : '');
			const ev = document.getElementById('stat-errors');
			if (ev) ev.className = 'stat-value' + (s.errors > 0 ? ' red' : '');
		}

			// ─── Render instances table ──────────────────────────────────────
			function renderInstances() {
				const tbody = document.getElementById('instances-tbody');
				if (!tbody) return;

				let list = Object.values(window.__sablier.instances);
				const q = window.__sablier.searchQuery.toLowerCase();
				const group = window.__sablier.activeGroup;

				// Filter
				if (q) list = list.filter(i => i.name.toLowerCase().includes(q) || (i.groups||[]).some(g=>g.toLowerCase().includes(q)));
				if (group) list = list.filter(i => (i.groups||[]).includes(group));

				// Sort
				const key = window.__sablier.sortKey;
				const dir = window.__sablier.sortDir;
				list.sort((a,b) => {
					let va, vb;
				if (key === 'name')         { va = a.name;   vb = b.name; }
				else if (key === 'status')       { va = statusOrder(a.status); vb = statusOrder(b.status); }
				else if (key === 'efficiency')   { va = a.efficiencyPct || 0; vb = b.efficiencyPct || 0; }
				else { va = a.name; vb = b.name; }
				if (va < vb) return -dir;
				if (va > vb) return dir;
				return 0;
			});

			if (list.length === 0) {
				tbody.innerHTML = '<tr><td colspan="7"><div class="empty-state">No instances match the current filter.</div></td></tr>';
				return;
			}

			tbody.innerHTML = list.map(inst => instanceRowHTML(inst)).join('');
		}

		function statusOrder(s) {
			return {error:0, starting:1, ready:2, stopped:3}[s] ?? 9;
		}

		function statusBadgeHTML(status) {
			const known = { ready: 1, starting: 1, stopped: 1, error: 1 };
			const cls = known[status] ? 'badge badge-' + status : 'badge badge-stopped';
			return `<span class="${cls}"><span class="badge-dot"></span>${escHtml(status)}</span>`;
		}

		function instanceRowHTML(inst) {
			const badge   = statusBadgeHTML(inst.status);
			const lastAcc = lastAccessHTML(inst.lastAccess);
			const expiry  = expiresInHTML(inst._expiresAt, inst.name);
			const groups  = (inst.groups||[]).map(g => `<span class="group-pill" onclick="setGroupFilter('${escHtml(g)}',event)">${escHtml(g)}</span>`).join('');
			const chip    = `<span class="chip">${escHtml(inst.provider)}</span>`;
			const err     = inst.message ? `<div class="err-msg" title="${escHtml(inst.message)}">${escHtml(inst.message)}</div>` : '';
			const eff     = inst.uptimeWindowSeconds > 0
				? `<span style="color:var(--gold);font-weight:600">${Math.round(inst.efficiencyPct)}%</span> <span style="color:var(--text-sub)">idle</span>`
				: '<span style="color:var(--text-sub)">—</span>';

			return `<tr onclick="openModal('${escHtml(inst.name)}')" title="Click for details">
				<td style="font-weight:500">${escHtml(inst.name)}</td>
				<td>${badge}${err}</td>
				<td>${chip}</td>
				<td>${groups || '<span style="color:var(--text-sub)">—</span>'}</td>
				<td>${lastAcc}</td>
				<td>${expiry}</td>
				<td>${eff}</td>
			</tr>`;
		}

			function lastAccessHTML(lastAccess) {
				if (!lastAccess) return '<span class="last-access">—</span>';
				const ago = (Date.now() - new Date(lastAccess)) / 1000;
				const recent = ago < 120;
				return `<span class="last-access ${recent?'recent':''}" title="${new Date(lastAccess).toLocaleString()}">${fmtSeconds(ago)} ago</span>`;
			}

		// Render the expiry cell once; tickExpiry() keeps it ticking every second.
		function expiresInHTML(expiresAtMs, name) {
			if (!expiresAtMs) return '<span style="color:var(--text-sub)">—</span>';
			const remaining = (expiresAtMs - Date.now()) / 1000;
			if (remaining <= 0) return '<span style="color:var(--text-sub)">—</span>';
			const urgent = remaining < 60;
			const color = urgent ? 'var(--amber,#f59e0b)' : 'var(--text-sub)';
			const title = new Date(expiresAtMs).toLocaleString();
			return `<span data-expiry="${escHtml(name)}" style="color:${color}" title="Expires at ${title}">${fmtSeconds(remaining)}</span>`;
		}

		// Tick all visible expiry cells every second without re-rendering the table.
		function tickExpiry() {
			document.querySelectorAll('[data-expiry]').forEach(function(el) {
				const inst = window.__sablier.instances[el.dataset.expiry];
				if (!inst || !inst._expiresAt) { el.textContent = '—'; el.style.color = 'var(--text-sub)'; return; }
				const remaining = (inst._expiresAt - Date.now()) / 1000;
				if (remaining <= 0) { el.textContent = '—'; el.style.color = 'var(--text-sub)'; return; }
				el.style.color = remaining < 60 ? 'var(--amber,#f59e0b)' : 'var(--text-sub)';
				el.textContent = fmtSeconds(remaining);
			});
		}
		setInterval(tickExpiry, 1000);

		function fmtSeconds(s) {
			if (s < 0) return 'expired';
			if (s < 60) return Math.round(s) + 's';
			if (s < 3600) return Math.floor(s/60) + 'm ' + String(Math.round(s%60)).padStart(2,'0') + 's';
			return Math.floor(s/3600) + 'h ' + Math.floor((s%3600)/60) + 'm';
		}

		function escHtml(s) {
			return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
		}

		// ─── Live ticking (re-render to keep last-access times fresh) ───
		setInterval(renderInstances, 5000);

		// ─── Group filter ─────────────────────────────────────────────────
		function renderGroups(groups) {
			const container = document.getElementById('group-pills');
			if (!container) return;
			const active = window.__sablier.activeGroup;
			let html = `<span class="filter-pill ${!active ? 'active' : ''}" data-group="__all__" onclick="setGroupFilter(null,event)">All</span>`;
			groups.forEach(function(g) {
				html += `<span class="filter-pill ${active === g ? 'active' : ''}" data-group="${escHtml(g)}" onclick="setGroupFilter('${escHtml(g)}',event)">${escHtml(g)}</span>`;
			});
			container.innerHTML = html;
		}

		function setGroupFilter(group, event) {
			if (event) event.stopPropagation();
			window.__sablier.activeGroup = group;
			document.querySelectorAll('#group-pills .filter-pill').forEach(el => {
				el.classList.toggle('active', el.dataset.group === (group || '__all__'));
			});
			renderInstances();
		}

			function setSort(key) {
				if (window.__sablier.sortKey === key) {
					window.__sablier.sortDir *= -1;
				} else {
					window.__sablier.sortKey = key;
					window.__sablier.sortDir = 1;
				}
				document.querySelectorAll('thead th[data-sort]').forEach(th => {
					th.classList.toggle('sorted', th.dataset.sort === key);
				});
				renderInstances();
			}

			// ─── Search ──────────────────────────────────────────────────────
			document.addEventListener('DOMContentLoaded', function() {
				const inp = document.getElementById('search-input');
				if (inp) {
					inp.addEventListener('input', function() {
						window.__sablier.searchQuery = this.value;
						renderInstances();
					});
				}
			});

			// ─── Modal ───────────────────────────────────────────────────────
			function openModal(name) {
				const inst = window.__sablier.instances[name];
				if (!inst) return;
				document.getElementById('modal-instance-name').textContent = name;
				document.getElementById('modal-status-badge').innerHTML = statusBadgeHTML(inst.status);
				document.getElementById('modal-body').innerHTML = buildModalBody(inst);
				document.getElementById('instance-modal').classList.add('open');
			}

			function closeModal() {
				document.getElementById('instance-modal').classList.remove('open');
			}

			document.addEventListener('keydown', e => { if (e.key === 'Escape') closeModal(); });

			function buildModalBody(inst) {
			const eff = inst.efficiencyPct || 0;
			const activeSec = inst.activeSeconds || 0;
			const windowSec = inst.uptimeWindowSeconds || 0;
			const idleSec = windowSec - activeSec;

			let meta = '';
			if (inst.docker) meta += `<div class="config-row"><span class="config-key">image</span><code>${escHtml(inst.docker.image)}</code></div>`;
			if (inst.kubernetes) {
				meta += `<div class="config-row"><span class="config-key">namespace</span><code>${escHtml(inst.kubernetes.namespace)}</code></div>`;
				meta += `<div class="config-row"><span class="config-key">kind</span><code>${escHtml(inst.kubernetes.kind)}</code></div>`;
			}
			if (inst.runningHours) meta += `<div class="config-row"><span class="config-key">running-hours</span><code>${escHtml(inst.runningHours)}</code></div>`;
			if (inst.readyAfter)   meta += `<div class="config-row"><span class="config-key">ready-after</span><code>${fmtSeconds(inst.readyAfter/1e9)}</code></div>`;

			const effSection = windowSec > 0 ? `
			<div class="graph-section">
				<div class="graph-label">Efficiency — since Sablier started</div>
				<div class="savings-detail">
					<div class="savings-kpi" style="border-color:var(--gold-border);background:var(--gold-dim)">
						<div class="savings-kpi-label">Efficiency score</div>
						<div class="savings-kpi-value" style="color:var(--gold)">${Math.round(eff)}<span style="font-size:14px;font-weight:400">/100</span></div>
						<div class="savings-kpi-desc">idle fraction since startup</div>
					</div>
					<div class="savings-kpi">
						<div class="savings-kpi-label">Idle time</div>
						<div class="savings-kpi-value">${fmtSeconds(idleSec)}</div>
						<div class="savings-kpi-desc">total idle since startup</div>
					</div>
					<div class="savings-kpi">
						<div class="savings-kpi-label">Active time</div>
						<div class="savings-kpi-value">${fmtSeconds(activeSec)}</div>
						<div class="savings-kpi-desc">total ready time</div>
					</div>
				</div>
			</div>` : '';

			return effSection + (meta ? `<div class="graph-section"><div class="graph-label">Instance metadata</div><div class="section" style="border-radius:8px">${meta}</div></div>` : '');
		}
