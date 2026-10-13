// DeployDock — Budget & Usage page (DUMMY DATA for now).
// Later: replace SERVICES with real deployments + usage from the backend / Prometheus.
document.addEventListener('DOMContentLoaded', () => {
  const view = document.getElementById('budget-view');
  if (!view) return;

  /* ------------------------- dummy data ------------------------- */
  const HOURS_PER_BILLING_MONTH = 730; // monthly price = hourly price x 730, billing is capped at monthly

  const PLANS = [
    { id: 'free',     name: 'Free',     vcpu: 0.25, ramGb: 0.5, monthly: 0,  note: 'Sleeps after 15 min idle' },
    { id: 'starter',  name: 'Starter',  vcpu: 0.5,  ramGb: 1,   monthly: 7,  note: 'Small APIs, side projects' },
    { id: 'standard', name: 'Standard', vcpu: 1,    ramGb: 2,   monthly: 25, note: 'Production web apps' },
    { id: 'pro',      name: 'Pro',      vcpu: 2,    ramGb: 4,   monthly: 85, note: 'Heavy / always-busy services' },
  ];
  const PLAN_BY_ID = Object.fromEntries(PLANS.map(p => [p.id, p]));

  // Extras billed on top of compute (account-wide, with a free allowance)
  const RATES = {
    bandwidthFreeGb: 100, bandwidthPerGb: 0.10,
    buildFreeMin: 500,    buildPerMin: 0.005,
    storagePerGbMonth: 0.25,
  };

  // ageHours = how long the service has existed (big number = whole month)
  const SERVICES = [
    { name: 'portfolio-site', plan: 'free',     ageHours: 99999, avgCpu: 2,  bandwidthGbPerDay: 0.5, buildMinPerDay: 2,  storageGb: 0.5 },
    { name: 'blog-api',       plan: 'starter',  ageHours: 99999, avgCpu: 6,  bandwidthGbPerDay: 1.2, buildMinPerDay: 4,  storageGb: 2 },
    { name: 'shop-backend',   plan: 'standard', ageHours: 99999, avgCpu: 41, bandwidthGbPerDay: 5.5, buildMinPerDay: 12, storageGb: 8 },
    { name: 'staging-api',    plan: 'starter',  ageHours: 200,   avgCpu: 24, bandwidthGbPerDay: 0.3, buildMinPerDay: 6,  storageGb: 1.5 },
  ];

  /* ------------------------- settings ------------------------- */
  const SETTINGS_KEY = 'deploydock-budget-settings';
  const settings = { budget: 50, alertPct: 80, autoPause: false };
  try { Object.assign(settings, JSON.parse(localStorage.getItem(SETTINGS_KEY) || '{}')); } catch (e) { /* ignore */ }
  function saveSettings() {
    try { localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings)); } catch (e) { /* ignore */ }
  }
  let whatIf = 1;
  let data = null;

  /* ------------------------- helpers ------------------------- */
  const money = (n) => '$' + n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  const money4 = (n) => '$' + n.toFixed(4);
  const pct = (n) => Math.round(n) + '%';
  const sum = (arr, f) => arr.reduce((a, x) => a + f(x), 0);
  const statusClass = (p) => (p >= 100 ? 'bad' : p >= settings.alertPct ? 'warn' : 'ok');

  /* ------------------------- the maths ------------------------- */
  function calculate() {
    const now = new Date();
    const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
    const day = now.getDate();
    const elapsedHours = Math.max((day - 1) * 24 + now.getHours() + now.getMinutes() / 60, 1);
    const monthHours = daysInMonth * 24;
    const remainingHours = Math.max(monthHours - elapsedHours, 0);
    const elapsedDays = elapsedHours / 24;
    const remainingDays = remainingHours / 24;

    const rows = SERVICES.map(s => {
      const plan = PLAN_BY_ID[s.plan];
      const hourly = plan.monthly / HOURS_PER_BILLING_MONTH;
      const hoursMtd = Math.min(elapsedHours, s.ageHours);
      return {
        ...s, plan, hourly, hoursMtd,
        costMtd: Math.min(hoursMtd * hourly, plan.monthly),
        costProj: Math.min((hoursMtd + remainingHours) * hourly, plan.monthly),
      };
    });

    const bwPerDay = sum(SERVICES, s => s.bandwidthGbPerDay);
    const buildPerDay = sum(SERVICES, s => s.buildMinPerDay);
    const storageGb = sum(SERVICES, s => s.storageGb);

    const categories = [
      { name: 'Compute',   mtd: sum(rows, r => r.costMtd), proj: sum(rows, r => r.costProj),
        detail: 'hourly plan price, capped at monthly' },
      { name: 'Bandwidth', mtd: Math.max(0, bwPerDay * elapsedDays - RATES.bandwidthFreeGb) * RATES.bandwidthPerGb,
        proj: Math.max(0, bwPerDay * daysInMonth - RATES.bandwidthFreeGb) * RATES.bandwidthPerGb,
        detail: `${RATES.bandwidthFreeGb} GB free, then ${money(RATES.bandwidthPerGb)}/GB` },
      { name: 'Build minutes', mtd: Math.max(0, buildPerDay * elapsedDays - RATES.buildFreeMin) * RATES.buildPerMin,
        proj: Math.max(0, buildPerDay * daysInMonth - RATES.buildFreeMin) * RATES.buildPerMin,
        detail: `${RATES.buildFreeMin} min free, then ${money4(RATES.buildPerMin)}/min` },
      { name: 'Storage', mtd: storageGb * RATES.storagePerGbMonth * (elapsedHours / monthHours),
        proj: storageGb * RATES.storagePerGbMonth,
        detail: `${money(RATES.storagePerGbMonth)}/GB-month` },
    ];
    const mtd = sum(categories, c => c.mtd);
    const proj = sum(categories, c => c.proj);

    // Daily bars: spread month-to-date over past days, spread the rest evenly over future days
    const weights = [];
    for (let i = 1; i <= day; i++) {
      let w = 0.8 + 0.4 * (((i * 7919) % 10) / 9);
      if (i === day) w *= Math.max(now.getHours() / 24, 0.1);
      weights.push(w);
    }
    const wSum = sum(weights, w => w);
    const futureDays = daysInMonth - day;
    const days = [];
    for (let i = 1; i <= daysInMonth; i++) {
      days.push(i <= day
        ? { day: i, cost: mtd * weights[i - 1] / wSum, projected: false }
        : { day: i, cost: futureDays > 0 ? (proj - mtd) / futureDays : 0, projected: true });
    }
    let running = 0, hitDay = null;
    for (const d of days) {
      running += d.cost;
      if (hitDay === null && running >= settings.budget) hitDay = d.day;
    }

    return { now, daysInMonth, day, elapsedDays, remainingDays, rows, categories, mtd, proj, days, hitDay };
  }

  /* ------------------------- page shell (built once) ------------------------- */
  view.innerHTML = `
    <div class="bud-head">
      <div>
        <h1 class="bud-title">Budget &amp; Usage</h1>
        <div class="bud-sub" id="bud-period"></div>
      </div>
      <span class="bud-tag">Demo data</span>
    </div>
    <div id="bud-banner"></div>
    <div class="bud-stats" id="bud-stats"></div>
    <div class="card bud-card" id="bud-progress"></div>
    <div class="bud-cols">
      <div class="card bud-card">
        <div class="bud-card-title">Daily spend (dashed = projected)</div>
        <div id="bud-chart"></div>
      </div>
      <div class="card bud-card">
        <div class="bud-card-title">Where the money goes</div>
        <div id="bud-breakdown"></div>
      </div>
    </div>
    <div class="card bud-card">
      <div class="bud-card-title">What if usage grows?</div>
      <div class="bud-whatif">
        <label class="form-label" for="bud-whatif-range">Traffic &amp; replicas scale &times;<strong id="bud-whatif-val">1.0</strong></label>
        <input type="range" id="bud-whatif-range" min="1" max="4" step="0.5" value="1">
      </div>
      <div id="bud-whatif-out"></div>
    </div>
    <div class="card bud-card">
      <div class="bud-card-title">Plans</div>
      <div class="bud-plans" id="bud-plans"></div>
    </div>
    <div class="card" id="bud-services-card">
      <div class="bud-card bud-card-title" style="margin:0">Cost per service</div>
      <div class="table-responsive" id="bud-services"></div>
    </div>
    <div class="card bud-card">
      <div class="bud-card-title">Ways to save</div>
      <ul class="bud-recs" id="bud-recs"></ul>
    </div>
    <div class="card bud-card">
      <div class="bud-card-title">Budget settings</div>
      <div class="bud-settings">
        <div class="form-group">
          <label class="form-label" for="bud-set-budget">Monthly budget ($)</label>
          <input type="number" id="bud-set-budget" class="form-control" min="1" step="5">
        </div>
        <div class="form-group">
          <label class="form-label" for="bud-set-alert">Alert me at</label>
          <select id="bud-set-alert" class="form-control">
            <option value="50">50% of budget</option>
            <option value="70">70% of budget</option>
            <option value="80">80% of budget</option>
            <option value="90">90% of budget</option>
          </select>
        </div>
        <div class="form-group">
          <label class="form-label" for="bud-set-pause">Auto-pause at 100%</label>
          <label class="bud-check"><input type="checkbox" id="bud-set-pause"> Stop services when budget is hit</label>
          <span class="form-hint">Saved, but not enforced yet. Needs backend support.</span>
        </div>
      </div>
    </div>`;

  const $ = (id) => document.getElementById(id);
  $('bud-set-budget').value = settings.budget;
  $('bud-set-alert').value = String(settings.alertPct);
  $('bud-set-pause').checked = !!settings.autoPause;

  /* ------------------------- rendering ------------------------- */
  function renderAll() {
    data = calculate();
    const { now, daysInMonth, day, elapsedDays, remainingDays, rows, categories, mtd, proj, days, hitDay } = data;
    const budget = settings.budget;
    const usedPct = (mtd / budget) * 100;
    const projPct = (proj / budget) * 100;
    const monthName = now.toLocaleString('en-US', { month: 'long' });

    $('bud-period').textContent =
      `Billing period: ${monthName} 1 – ${monthName} ${daysInMonth} · resets in ${Math.ceil(remainingDays)} day(s)`;

    // banner
    let banner;
    if (mtd >= budget) {
      banner = ['bad', `Budget exceeded. You've spent ${money(mtd)} against a ${money(budget)} budget.`];
    } else if (proj > budget) {
      const when = hitDay ? ` At this pace you'll hit it around ${monthName} ${hitDay}.` : '';
      banner = ['warn', `On pace to exceed your budget by ${money(proj - budget)}.${when}`];
    } else if (usedPct >= settings.alertPct) {
      banner = ['warn', `You've used ${pct(usedPct)} of your budget (alert set at ${settings.alertPct}%).`];
    } else {
      banner = ['ok', `On track. Projected ${money(proj)} this month (${pct(projPct)} of budget).`];
    }
    $('bud-banner').innerHTML = `<div class="bud-banner ${banner[0]}">${banner[1]}</div>`;

    // stat cards
    const allowedPerDay = remainingDays > 0.01 ? Math.max(budget - mtd, 0) / remainingDays : 0;
    $('bud-stats').innerHTML = [
      ['Spent so far', money(mtd), `Day ${day} of ${daysInMonth}`, ''],
      ['Budget used', pct(usedPct), `of ${money(budget)}`, statusClass(usedPct)],
      ['Projected month-end', money(proj), `${pct(projPct)} of budget at current usage`, projPct > 100 ? 'bad' : ''],
      ['Avg daily spend', money(mtd / elapsedDays), `${money(allowedPerDay)}/day left to stay in budget`, ''],
    ].map(([label, value, sub, cls]) => `
      <div class="card bud-stat">
        <div class="bud-stat-label">${label}</div>
        <div class="bud-stat-value ${cls}">${value}</div>
        <div class="bud-stat-sub">${sub}</div>
      </div>`).join('');

    // progress bar
    $('bud-progress').innerHTML = `
      <div class="bud-card-title">Budget progress</div>
      <div class="bud-track-wrap">
        <div class="bud-track">
          <div class="bud-fill-proj" style="width:${Math.min(projPct, 100)}%"></div>
          <div class="bud-fill ${statusClass(usedPct)}" style="width:${Math.min(usedPct, 100)}%"></div>
        </div>
        <div class="bud-marker" style="left:${settings.alertPct}%" title="Alert at ${settings.alertPct}%"></div>
      </div>
      <div class="bud-legend">
        <span><i class="dot solid"></i> Spent ${money(mtd)}</span>
        <span><i class="dot striped"></i> Projected ${money(proj)}</span>
        <span><i class="dot line"></i> Alert at ${settings.alertPct}%</span>
      </div>`;

    // daily chart
    const maxDay = Math.max(...days.map(d => d.cost), 0.01);
    $('bud-chart').innerHTML = `
      <div class="bud-chart">${days.map(d =>
        `<div class="bud-bar ${d.projected ? 'projected' : ''}" style="height:${Math.max((d.cost / maxDay) * 100, 2)}%"
           title="${monthName} ${d.day}: ${money(d.cost)}${d.projected ? ' (projected)' : ''}"></div>`).join('')}
      </div>
      <div class="bud-axis"><span>1</span><span>${Math.round(daysInMonth / 2)}</span><span>${daysInMonth}</span></div>`;

    // breakdown
    $('bud-breakdown').innerHTML = categories.map(c => `
      <div class="bud-cat">
        <div class="bud-row"><span>${c.name}</span><span>${money(c.mtd)} <small>→ ${money(c.proj)}</small></span></div>
        <div class="bud-mini"><span style="width:${proj > 0 ? (c.proj / proj) * 100 : 0}%"></span></div>
        <div class="bud-note">${c.detail}</div>
      </div>`).join('') +
      `<div class="bud-row bud-total"><span>Total</span><span>${money(mtd)} <small>→ ${money(proj)}</small></span></div>`;

    renderWhatIf();

    // plans
    $('bud-plans').innerHTML = PLANS.map(p => {
      const hourly = p.monthly / HOURS_PER_BILLING_MONTH;
      const using = SERVICES.filter(s => s.plan === p.id).length;
      return `
        <div class="bud-plan ${using ? 'in-use' : ''}">
          <div class="bud-plan-name">${p.name} ${using ? `<span class="bud-tag">${using} in use</span>` : ''}</div>
          <div class="bud-plan-price">${money(p.monthly)}<small>/mo</small></div>
          <div class="bud-note">${money4(hourly)}/hour</div>
          <div class="bud-note">${p.vcpu} vCPU · ${p.ramGb} GB RAM</div>
          <div class="bud-note">${p.vcpu && hourly ? money4(hourly / p.vcpu) + ' per vCPU-hour' : 'no charge'}</div>
          <div class="bud-note">${p.note}</div>
        </div>`;
    }).join('');

    // services table
    $('bud-services').innerHTML = `
      <table class="data-table">
        <thead><tr><th>Service</th><th>Plan</th><th>Hours</th><th>Rate/hr</th><th>Cost to date</th><th>Projected</th><th>Avg CPU</th></tr></thead>
        <tbody>${rows.map(r => `
          <tr>
            <td>${r.name}</td>
            <td>${r.plan.name}</td>
            <td>${Math.round(r.hoursMtd)}h</td>
            <td>${money4(r.hourly)}</td>
            <td>${money(r.costMtd)}</td>
            <td>${money(r.costProj)}</td>
            <td class="${r.avgCpu < 10 ? 'bud-low' : r.avgCpu > 75 ? 'bud-high' : ''}">${r.avgCpu}%</td>
          </tr>`).join('')}
        </tbody>
      </table>`;

    // recommendations
    const recs = [];
    rows.forEach(r => {
      const idx = PLANS.findIndex(p => p.id === r.plan.id);
      if (r.avgCpu < 10 && idx > 0) {
        const lower = PLANS[idx - 1];
        recs.push(`<b>${r.name}</b> averages only ${r.avgCpu}% CPU on ${r.plan.name}. Moving to ${lower.name} would save about ${money(r.plan.monthly - lower.monthly)}/month.`);
      }
      if (r.avgCpu > 75 && idx < PLANS.length - 1) {
        recs.push(`<b>${r.name}</b> averages ${r.avgCpu}% CPU. Consider upgrading or enabling autoscaling before it starts failing health checks.`);
      }
    });
    const bwProj = categories[1].proj;
    if (bwProj > 0) recs.push(`Bandwidth is projected at ${money(bwProj)} this month. Caching static files or compressing responses can cut it.`);
    if (projPct > 100) recs.push(`You're projected ${money(proj - budget)} over budget. Either raise the budget or trim the items above.`);
    $('bud-recs').innerHTML = recs.length ? recs.map(r => `<li>${r}</li>`).join('') : '<li>Nothing to trim right now.</li>';

    syncTopCard(mtd, proj, budget);
  }

  function renderWhatIf() {
    if (!data) return;
    const { mtd, proj } = data;
    const budget = settings.budget;
    const scaled = mtd + (proj - mtd) * whatIf;
    const p = (scaled / budget) * 100;
    $('bud-whatif-val').textContent = whatIf.toFixed(1);
    $('bud-whatif-out').innerHTML = `
      <div class="bud-row"><span>Projected month-end at ×${whatIf.toFixed(1)}</span>
        <span class="${p > 100 ? 'bad' : ''}">${money(scaled)} (${pct(p)} of budget)</span></div>
      <div class="bud-note">Rough estimate: the rest of the month's usage is multiplied, month-to-date stays as is.
        ${whatIf > 1 ? `That's ${money(scaled - proj)} more than the current projection.` : ''}</div>`;
  }

  // keep the small card on the main dashboard in sync with this page
  function syncTopCard(mtd, proj, budget) {
    const bar = $('budget-donut-bar');
    if (bar) bar.setAttribute('stroke-dashoffset', (301.59 * (1 - Math.min(mtd / budget, 1))).toFixed(1));
    if ($('val-est-billing')) $('val-est-billing').textContent = money(proj);
    if ($('val-budget-total')) $('val-budget-total').textContent = money(budget);
  }

  /* ------------------------- events ------------------------- */
  $('bud-set-budget').addEventListener('change', (e) => {
    const v = parseFloat(e.target.value);
    settings.budget = v > 0 ? v : settings.budget;
    e.target.value = settings.budget;
    saveSettings(); renderAll();
  });
  $('bud-set-alert').addEventListener('change', (e) => {
    settings.alertPct = parseInt(e.target.value, 10);
    saveSettings(); renderAll();
  });
  $('bud-set-pause').addEventListener('change', (e) => {
    settings.autoPause = e.target.checked;
    saveSettings();
  });
  $('bud-whatif-range').addEventListener('input', (e) => {
    whatIf = parseFloat(e.target.value);
    renderWhatIf();
  });

  /* ------------------------- switching pages via the sidebar ------------------------- */
  const metricsGrid = document.querySelector('.metrics-grid');
  const deploymentsCard = document.getElementById('deployments-table-card');
  function showView(name) {
    const budgetOn = name === 'budget';
    view.style.display = budgetOn ? 'flex' : 'none';
    if (metricsGrid) metricsGrid.style.display = budgetOn ? 'none' : '';
    if (deploymentsCard) deploymentsCard.style.display = budgetOn ? 'none' : '';
  }
  document.getElementById('nav-budget')?.addEventListener('click', () => showView('budget'));
  document.getElementById('nav-dashboard')?.addEventListener('click', () => showView('dashboard'));
  document.getElementById('nav-deployments')?.addEventListener('click', () => showView('dashboard'));

  renderAll();
  if (location.hash === '#budget') {
    document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
    document.getElementById('nav-budget')?.classList.add('active');
    showView('budget');
  }
  setInterval(renderAll, 60000);
});
