			// ─── Global state (updated by SSE) ─────────────────────────────
			window.__sablier = {
				instances: {},
				pending: [],
				activeGroup: null,
				sortKey: 'name',
				sortDir: 1,
				searchQuery: '',
			};

			// ─── SSE ────────────────────────────────────────────────────────
			(function connectSSE() {
				const es = new EventSource('/dashboard/stream');

				es.addEventListener('instances', function(e) {
					const data = JSON.parse(e.data);
					// Update global store
					data.forEach(inst => {
						window.__sablier.instances[inst.name] = inst;
					});
					renderInstances();
					document.getElementById('last-ts').textContent = new Date().toLocaleTimeString();
				});

				es.addEventListener('pending', function(e) {
					window.__sablier.pending = JSON.parse(e.data);
					renderPending();
				});

				es.addEventListener('stats', function(e) {
					const s = JSON.parse(e.data);
					updateStats(s);
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
			function updateStats(s) {
				setInner('stat-total',   s.total);
				setInner('stat-active',  s.active);
				setInner('stat-errors',  s.errors);
				setInner('stat-pending', s.pending);
				setInner('stat-co2',     s.totalCO2Grams.toFixed(0));
				setInner('stat-hours',   s.totalDowntimeHours.toFixed(0));
				// Color active stat
				const av = document.getElementById('stat-active');
				if(av) av.className = 'stat-value' + (s.active > 0 ? ' green' : '');
				const ev = document.getElementById('stat-errors');
				if(ev) ev.className = 'stat-value' + (s.errors > 0 ? ' red' : '');
				const pv = document.getElementById('stat-pending');
				if(pv) pv.className = 'stat-value' + (s.pending > 0 ? ' amber' : '');
			}

			function setInner(id, val) {
				const el = document.getElementById(id);
				if (el && el.textContent !== String(val)) {
					el.textContent = val;
					el.classList.add('sse-update');
					setTimeout(() => el.classList.remove('sse-update'), 400);
				}
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
					if (key === 'name')   { va = a.name;   vb = b.name; }
					else if (key === 'status') { va = statusOrder(a.status); vb = statusOrder(b.status); }
					else if (key === 'ttl')    { va = a.expiresAt || ''; vb = b.expiresAt || ''; }
					else if (key === 'idle')   { va = a.idlePercent || 0; vb = b.idlePercent || 0; }
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

			function instanceRowHTML(inst) {
				const badge = statusBadgeHTML(inst.status);
				const ttl = ttlHTML(inst.expiresAt, inst.sessionTTL);
				const lastAcc = lastAccessHTML(inst.lastAccess);
				const bar = uptimeBarHTML(inst.uptimeSlots || []);
				const groups = (inst.groups||[]).map(g => `<span class="group-pill" onclick="setGroupFilter('${g}',event)">${g}</span>`).join('');
				const chip = `<span class="chip">${inst.provider}</span>`;
				const err = inst.message ? `<div class="err-msg" title="${inst.message}">${inst.message}</div>` : '';
				const idle = inst.idlePercent != null ? `<span style="color:var(--gold);font-weight:600">${inst.idlePercent}%</span> <span style="color:var(--text-sub)">idle</span>` : '';

				return `<tr onclick="openModal('${escHtml(inst.name)}')" title="Click for details">
					<td style="font-weight:500">${escHtml(inst.name)}</td>
					<td>${badge}${err}</td>
					<td>${chip}</td>
					<td>${groups || '<span style="color:var(--text-sub)">—</span>'}</td>
					<td>${ttl}</td>
					<td>${lastAcc}</td>
					<td>${bar}<div style="margin-top:3px">${idle}</div></td>
				</tr>`;
			}

			function statusBadgeHTML(status) {
				const map = {
					ready:    ['badge-ready',    'ready'],
					starting: ['badge-starting', 'starting'],
					stopped:  ['badge-stopped',  'stopped'],
					error:    ['badge-error',    'error'],
				};
				const [cls, label] = map[status] || ['badge-stopped', status];
				return `<span class="badge ${cls}"><span class="badge-dot"></span>${label}</span>`;
			}

			function ttlHTML(expiresAt, sessionTTL) {
				if (!expiresAt || !sessionTTL) return '<span style="color:var(--text-sub)">—</span>';
				const remaining = (new Date(expiresAt) - Date.now()) / 1000;
				const total = sessionTTL / 1e9; // Go Duration is nanoseconds
				if (remaining <= 0) return '<span style="color:var(--red)">expired</span>';
				const pct = Math.min(100, Math.max(0, (remaining / total) * 100));
				const fillCls = pct < 25 ? 'ttl-bar-fill crit' : pct < 50 ? 'ttl-bar-fill low' : 'ttl-bar-fill';
				const label = fmtSeconds(remaining);
				return `<div class="ttl-wrap">
					<span class="ttl-label">${label}</span>
					<div class="ttl-bar-bg"><div class="${fillCls}" style="width:${pct.toFixed(1)}%"></div></div>
				</div>`;
			}

			function lastAccessHTML(lastAccess) {
				if (!lastAccess) return '<span class="last-access">—</span>';
				const ago = (Date.now() - new Date(lastAccess)) / 1000;
				const recent = ago < 120;
				return `<span class="last-access ${recent?'recent':''}" title="${new Date(lastAccess).toLocaleString()}">${fmtSeconds(ago)} ago</span>`;
			}

			function uptimeBarHTML(slots) {
				if (!slots || slots.length === 0) return '';
				// Compact: show last 144 slots (12h) as thin bars
				const show = slots.slice(-144);
				const bars = show.map(s => `<div class="uptime-slot ${slotClass(s.state)}" title="${new Date(s.time).toLocaleTimeString()}"></div>`).join('');
				return `<div class="uptime-bar">${bars}</div>`;
			}

			function slotClass(state) {
				return ['up','starting','idle','error'][state] || 'idle';
			}

			function fmtSeconds(s) {
				if (s < 0) return 'expired';
				if (s < 60) return Math.round(s) + 's';
				if (s < 3600) return Math.floor(s/60) + 'm ' + String(Math.round(s%60)).padStart(2,'0') + 's';
				return Math.floor(s/3600) + 'h ' + Math.floor((s%3600)/60) + 'm';
			}

			function escHtml(s) {
				return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
			}

			// ─── Render pending ──────────────────────────────────────────────
			function renderPending() {
				const tbody = document.getElementById('pending-tbody');
				if (!tbody) return;
				const list = window.__sablier.pending || [];
				if (list.length === 0) {
					tbody.innerHTML = '<tr><td colspan="5"><div class="empty-state">No pending requests.</div></td></tr>';
					document.getElementById('pending-count').textContent = '0';
					return;
				}
				document.getElementById('pending-count').textContent = String(list.length);
				tbody.innerHTML = list.map(req => {
					const waiting = (Date.now() - new Date(req.requestedAt)) / 1000;
					const total   = req.timeout / 1e9;
					const pct     = Math.min(100, (waiting / total) * 100);
					const fillCls = pct > 85 ? 'ttl-bar-fill crit' : pct > 60 ? 'ttl-bar-fill low' : 'ttl-bar-fill';
					const target  = req.group
						? `<span class="group-pill">group: ${escHtml(req.group)}</span>`
						: (req.names||[]).map(n=>`<span class="group-pill">${escHtml(n)}</span>`).join('');
					return `<tr>
						<td class="mono" style="color:var(--text-sub)">${escHtml(req.id)}</td>
						<td>${target}</td>
						<td style="color:var(--text-muted)">${fmtSeconds(waiting)}</td>
						<td style="color:var(--text-muted)">${fmtSeconds(total)}</td>
						<td>
							<div class="ttl-wrap" style="min-width:80px">
								<span class="badge badge-pending"><span class="badge-dot"></span>waiting</span>
								<div class="ttl-bar-bg"><div class="${fillCls}" style="width:${pct.toFixed(1)}%"></div></div>
							</div>
						</td>
					</tr>`;
				}).join('');
			}

			// ─── TTL live ticking ────────────────────────────────────────────
			setInterval(function() {
				renderInstances();
				renderPending();
			}, 1000);

			// ─── Group filter ────────────────────────────────────────────────
			function setGroupFilter(group, event) {
				if (event) event.stopPropagation();
				const cur = window.__sablier.activeGroup;
				window.__sablier.activeGroup = cur === group ? null : group;
				// Update pill UI
				document.querySelectorAll('.filter-pill[data-group]').forEach(el => {
					el.classList.toggle('active', el.dataset.group === window.__sablier.activeGroup);
				});
				document.querySelectorAll('.filter-pill[data-group="__all__"]').forEach(el => {
					el.classList.toggle('active', !window.__sablier.activeGroup);
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
				const slots = inst.uptimeSlots || [];
				// Build 24-column heatmap (last 24h = 288 slots, 12 per hour)
				const last288 = slots.slice(-288);
				let heatCols = '';
				for (let h = 0; h < 24; h++) {
					const hourSlots = last288.slice(h*12, h*12+12);
					const cells = hourSlots.map(s => `<div class="heatmap-slot ${slotClass(s.state)}" title="${new Date(s.time).toLocaleTimeString()}"></div>`).join('');
					const hLabel = String(h).padStart(2,'0');
					heatCols += `<div class="heatmap-hour">
						<div class="heatmap-slots">${cells}</div>
						<div class="heatmap-hour-label">${hLabel}h</div>
					</div>`;
				}

				const co2 = inst.savedCO2Grams || 0;
				const idle = inst.idlePercent || 0;
				const hours = inst.totalDowntimeHours || 0;
				const energySaved = (hours * 0.08).toFixed(1); // 80W avg server
				const moneyUSD = (energySaved * 0.12).toFixed(2); // ~$0.12/kWh

				let meta = '';
				if (inst.docker) meta += `<div class="config-row"><span class="config-key">image</span><code>${escHtml(inst.docker.image)}</code></div>`;
				if (inst.kubernetes) {
					meta += `<div class="config-row"><span class="config-key">namespace</span><code>${escHtml(inst.kubernetes.namespace)}</code></div>`;
					meta += `<div class="config-row"><span class="config-key">kind</span><code>${escHtml(inst.kubernetes.kind)}</code></div>`;
				}
				if (inst.runningHours) meta += `<div class="config-row"><span class="config-key">running-hours</span><code>${escHtml(inst.runningHours)}</code></div>`;
				if (inst.readyAfter)   meta += `<div class="config-row"><span class="config-key">ready-after</span><code>${fmtSeconds(inst.readyAfter/1e9)}</code></div>`;

				return `
				<div class="graph-section">
					<div class="graph-label">Activity — last 24 hours (5-min slots)</div>
					<div class="heatmap-grid">${heatCols}</div>
					<div style="display:flex;gap:16px;margin-top:4px;font-size:11px;color:var(--text-sub)">
						<span><span style="display:inline-block;width:10px;height:10px;background:var(--green);border-radius:2px;margin-right:4px"></span>up</span>
						<span><span style="display:inline-block;width:10px;height:10px;background:var(--blue);border-radius:2px;margin-right:4px"></span>starting</span>
						<span><span style="display:inline-block;width:10px;height:10px;background:var(--bg-hover);border:1px solid var(--border);border-radius:2px;margin-right:4px"></span>idle (resources saved)</span>
						<span><span style="display:inline-block;width:10px;height:10px;background:var(--red);border-radius:2px;margin-right:4px"></span>error</span>
					</div>
				</div>

				<div class="graph-section">
					<div class="graph-label">Efficiency — 7-day savings</div>
					<div class="savings-detail">
						<div class="savings-kpi">
							<div class="savings-kpi-label">Idle time</div>
							<div class="savings-kpi-value">${idle}<span style="font-size:14px;font-weight:400;color:var(--text-muted)">%</span></div>
							<div class="savings-kpi-desc">of the last 24 hours</div>
						</div>
						<div class="savings-kpi">
							<div class="savings-kpi-label">Compute saved</div>
							<div class="savings-kpi-value">${hours.toFixed(0)}<span style="font-size:14px;font-weight:400;color:var(--text-muted)">h</span></div>
							<div class="savings-kpi-desc">server-hours this week</div>
						</div>
						<div class="savings-kpi">
							<div class="savings-kpi-label">Energy saved</div>
							<div class="savings-kpi-value">${energySaved}<span style="font-size:14px;font-weight:400;color:var(--text-muted)">kWh</span></div>
							<div class="savings-kpi-desc">≈ $${moneyUSD} @ $0.12/kWh</div>
						</div>
						<div class="savings-kpi">
							<div class="savings-kpi-label">CO₂ avoided</div>
							<div class="savings-kpi-value">${co2.toFixed(0)}<span style="font-size:14px;font-weight:400;color:var(--text-muted)">g</span></div>
							<div class="savings-kpi-desc">≈ ${(co2/1000*4).toFixed(2)} km not driven</div>
						</div>
						<div class="savings-kpi">
							<div class="savings-kpi-label">Uptime fraction</div>
							<div class="savings-kpi-value">${(100-idle)}<span style="font-size:14px;font-weight:400;color:var(--text-muted)">%</span></div>
							<div class="savings-kpi-desc">only powered on when needed</div>
						</div>
						<div class="savings-kpi" style="border-color:var(--gold-border);background:var(--gold-dim)">
							<div class="savings-kpi-label">Efficiency score</div>
							<div class="savings-kpi-value" style="color:var(--gold)">${idle}<span style="font-size:14px;font-weight:400">/100</span></div>
							<div class="savings-kpi-desc">higher = greener</div>
						</div>
					</div>
				</div>

				${meta ? `<div class="graph-section"><div class="graph-label">Instance metadata</div><div class="section" style="border-radius:8px">${meta}</div></div>` : ''}
				`;
			}
