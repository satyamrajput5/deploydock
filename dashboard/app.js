document.addEventListener('DOMContentLoaded', () => {
  const tbody = document.getElementById('deployments-tbody');
  const emptyState = document.getElementById('table-empty-state');
  const countBadge = document.getElementById('deployment-count-badge');
  const deployModal = document.getElementById('deploy-modal');
  const logsModal = document.getElementById('logs-modal');
  const deployForm = document.getElementById('deploy-form');
  const repoInput = document.getElementById('repo-url-input');
  const errorBanner = document.getElementById('deploy-error-banner');
  const terminalText = document.getElementById('terminal-output-text');
  const logsTargetId = document.getElementById('logs-target-id');
  const sidebar = document.getElementById('app-sidebar');
  const backdrop = document.getElementById('sidebar-backdrop');
  const toggleBtn = document.getElementById('sidebar-toggle');
  const themeToggleBtn = document.getElementById('theme-toggle-btn');
  const sunIcon = document.getElementById('theme-sun-icon');
  const moonIcon = document.getElementById('theme-moon-icon');

  let activeLogsDeploymentId = null;

  const authModal = document.getElementById('auth-modal');
  const authForm = document.getElementById('auth-form');
  const authEmail = document.getElementById('auth-email');
  const authPassword = document.getElementById('auth-password');
  const authErrorBanner = document.getElementById('auth-error-banner');
  const authRegisterBtn = document.getElementById('auth-register-btn');
  const TOKEN_KEY = 'deploydock-token';

  function showAuthModal() {
    authModal.style.display = 'flex';
  }

  // Same as fetch(), but adds the JWT. If the server says 401, ask the user to log in again.
  async function authFetch(url, options = {}) {
    const headers = { ...(options.headers || {}) };
    const token = sessionStorage.getItem(TOKEN_KEY);
    if (token) headers['Authorization'] = 'Bearer ' + token;

    const res = await fetch(url, { ...options, headers });
    if (res.status === 401) {
      sessionStorage.removeItem(TOKEN_KEY);
      showAuthModal();
    }
    return res;
  }

  async function submitAuth(mode) {
    authErrorBanner.style.display = 'none';
    try {
      const res = await fetch('/auth/' + mode, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: authEmail.value.trim(), password: authPassword.value })
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Authentication failed');

      sessionStorage.setItem(TOKEN_KEY, data.token);
      authPassword.value = '';
      authModal.style.display = 'none';
      await fetchDeployments();
    } catch (err) {
      authErrorBanner.textContent = err.message;
      authErrorBanner.style.display = 'block';
    }
  }

  authForm?.addEventListener('submit', (e) => {
    e.preventDefault();
    submitAuth('login');
  });
  authRegisterBtn?.addEventListener('click', () => submitAuth('register'));


  function initTheme() {
    const savedTheme = localStorage.getItem('deploydock-theme') || 'dark';
    applyTheme(savedTheme);
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('deploydock-theme', theme);

    if (theme === 'light') {
      sunIcon.style.display = 'none';
      moonIcon.style.display = 'block';
    } else {
      sunIcon.style.display = 'block';
      moonIcon.style.display = 'none';
    }

    resizeAllCanvases();
  }

  themeToggleBtn?.addEventListener('click', () => {
    const current = document.documentElement.getAttribute('data-theme') || 'dark';
    const next = current === 'dark' ? 'light' : 'dark';
    applyTheme(next);
  });

  function toggleSidebar() {
    if (window.innerWidth <= 768) {
      const isOpen = sidebar.classList.toggle('open');
      if (isOpen) {
        backdrop.classList.add('visible');
      } else {
        backdrop.classList.remove('visible');
      }
    } else {
      sidebar.classList.toggle('collapsed');
      setTimeout(resizeAllCanvases, 220);
    }
  }

  toggleBtn?.addEventListener('click', toggleSidebar);

  backdrop?.addEventListener('click', () => {
    sidebar.classList.remove('open');
    backdrop.classList.remove('visible');
  });

  document.querySelectorAll('.nav-item').forEach(item => {
    item.addEventListener('click', () => {
      document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
      item.classList.add('active');
      if (window.innerWidth <= 768) {
        sidebar.classList.remove('open');
        backdrop.classList.remove('visible');
      }
    });
  });

  document.getElementById('btn-open-deploy-modal')?.addEventListener('click', openDeployModal);
  document.getElementById('btn-empty-deploy')?.addEventListener('click', openDeployModal);
  document.getElementById('modal-close-btn')?.addEventListener('click', closeDeployModal);
  document.getElementById('modal-cancel-btn')?.addEventListener('click', closeDeployModal);

  document.getElementById('logs-close-btn')?.addEventListener('click', () => {
    logsModal.style.display = 'none';
    activeLogsDeploymentId = null;
  });

  document.getElementById('btn-refresh-list')?.addEventListener('click', () => {
    fetchDeployments();
  });

  document.getElementById('btn-refresh-logs')?.addEventListener('click', () => {
    if (activeLogsDeploymentId) {
      fetchLogs(activeLogsDeploymentId);
    }
  });

  document.querySelectorAll('.chip-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      repoInput.value = btn.getAttribute('data-url');
      repoInput.focus();
    });
  });

  function openDeployModal() {
    deployModal.style.display = 'flex';
    errorBanner.style.display = 'none';
    repoInput.value = '';
    repoInput.focus();
  }

  function closeDeployModal() {
    deployModal.style.display = 'none';
  }

  deployForm?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const url = repoInput.value.trim();
    if (!url) return;

    const submitBtn = document.getElementById('modal-submit-btn');
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Deploying...</span>';
    errorBanner.style.display = 'none';

    try {
      const res = await authFetch('/deployments', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ repo_url: url })
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to submit deployment');
      }

      closeDeployModal();
      await fetchDeployments();
    } catch (err) {
      errorBanner.textContent = err.message;
      errorBanner.style.display = 'block';
    } finally {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span class="btn-text">Deploy</span>';
    }
  });

  async function fetchDeployments() {
    try {
      const res = await authFetch('/deployments');
      if (!res.ok) throw new Error('API request failed');
      const data = await res.json();
      const list = data.deployments || [];

      renderDeployments(list);
    } catch (err) {
      console.warn('Error fetching deployments:', err);
    }
  }

  function renderDeployments(list) {
    countBadge.textContent = list.length;

    if (list.length === 0) {
      tbody.innerHTML = '';
      emptyState.style.display = 'flex';
      return;
    }

    emptyState.style.display = 'none';
    tbody.innerHTML = list.map(item => {
      const statusClass = `status-${item.status || 'pending'}`;
      const portDisplay = item.port ? `:${item.port}` : '<span style="color: var(--fg-faint);">—</span>';
      const liveUrl = item.port 
        ? `<a href="http://localhost:${item.port}" target="_blank" rel="noopener noreferrer">http://localhost:${item.port}</a>` 
        : '<span style="color: var(--fg-faint);">—</span>';

      const created = item.created_at ? new Date(item.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '—';

      return `
        <tr data-id="${item.id}">
          <td class="cell-id">#${item.id}</td>
          <td>
            <a class="cell-repo" href="${item.repo_url}" target="_blank" rel="noopener noreferrer" title="${item.repo_url}">
              ${item.repo_url.replace('https://github.com/', '')}
            </a>
          </td>
          <td>
            <span class="status-badge ${statusClass}">${item.status}</span>
          </td>
          <td class="cell-port">${portDisplay}</td>
          <td class="cell-url">${liveUrl}</td>
          <td style="color: var(--fg-muted); font-size: 0.8rem; font-family: var(--font-mono);">${created}</td>
          <td style="text-align: right;">
            <div style="display: inline-flex; gap: 4px;">
              <button class="btn btn-secondary btn-sm" onclick="window.viewLogs(${item.id})">Logs</button>
              <button class="btn btn-danger btn-sm" onclick="window.deleteDeployment(${item.id})">Delete</button>
            </div>
          </td>
        </tr>
      `;
    }).join('');
  }

  window.viewLogs = async function(id) {
    activeLogsDeploymentId = id;
    logsTargetId.textContent = `#${id}`;
    terminalText.textContent = 'Loading...';
    logsModal.style.display = 'flex';
    await fetchLogs(id);
  };

  async function fetchLogs(id) {
    try {
      const res = await authFetch(`/deployments/${id}/logs`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to fetch logs');

      if (!data.logs || data.logs.length === 0) {
        terminalText.textContent = `Deployment #${id} queued.\nAwaiting build execution...`;
        return;
      }

      terminalText.textContent = data.logs.map(l => `[${new Date(l.created_at).toLocaleTimeString()}] ${l.message}`).join('\n');
    } catch (err) {
      terminalText.textContent = `Error loading logs: ${err.message}`;
    }
  }

  window.deleteDeployment = async function(id) {
    if (!confirm(`Delete deployment #${id}?`)) return;

    try {
      const res = await authFetch(`/deployments/${id}`, { method: 'DELETE' });
      if (!res.ok) throw new Error('Failed to delete');
      await fetchDeployments();
    } catch (err) {
      alert(`Error deleting deployment: ${err.message}`);
    }
  };

  const chartDrawers = [];

  function resizeAllCanvases() {
    chartDrawers.forEach(drawer => drawer());
  }

  window.addEventListener('resize', resizeAllCanvases);

  function initCharts() {
    setupCanvasChart('cpu-chart-canvas', 'var(--chart-cpu)', 'rgba(6, 182, 212, 0.08)', (val) => {
      const el = document.getElementById('cpu-current-pct');
      if (el) el.textContent = `${val.toFixed(1)}%`;
    });

    setupCanvasChart('ram-chart-canvas', 'var(--chart-ram)', 'rgba(139, 92, 246, 0.08)', (val) => {
      const el = document.getElementById('ram-current-val');
      if (el) el.textContent = `${Math.round(val * 16)} MB`;
    });
  }

  function setupCanvasChart(canvasId, strokeColor, fillColor, onUpdate) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    const ctx = canvas.getContext('2d');

    const points = Array.from({ length: 24 }, () => 12 + Math.random() * 18);

    function syncDimensions() {
      const rect = canvas.parentElement.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      const targetW = Math.round(rect.width * dpr);
      const targetH = Math.round((rect.height || 84) * dpr);

      if (canvas.width !== targetW || canvas.height !== targetH) {
        canvas.width = targetW;
        canvas.height = targetH;
      }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    }

    syncDimensions();

    function draw() {
      syncDimensions();

      const rect = canvas.parentElement.getBoundingClientRect();
      const w = rect.width;
      const h = rect.height || 84;

      ctx.save();
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.restore();

      points.shift();
      const last = points[points.length - 1];
      const next = Math.max(8, Math.min(65, last + (Math.random() * 8 - 4)));
      points.push(next);

      if (onUpdate) onUpdate(next);

      const isLight = document.documentElement.getAttribute('data-theme') === 'light';
      const actualStroke = strokeColor.startsWith('var') 
        ? (strokeColor.includes('cpu') ? (isLight ? '#0891b2' : '#06b6d4') : (isLight ? '#7c3aed' : '#8b5cf6')) 
        : strokeColor;

      const actualFill = strokeColor.includes('cpu') 
        ? (isLight ? 'rgba(8, 145, 178, 0.06)' : 'rgba(6, 182, 212, 0.08)')
        : (isLight ? 'rgba(124, 58, 237, 0.06)' : 'rgba(139, 92, 246, 0.08)');

      const axisX = 22;
      const rightPad = 8;
      const topY = 7;
      const bottomY = h - 7;
      const ticks = [0, 50, 100];

      ctx.textAlign = 'right';
      ctx.textBaseline = 'middle';
      ctx.font = '500 8px "Geist Mono", monospace';
      const labelColor = isLight ? 'rgba(0, 0, 0, 0.32)' : 'rgba(255, 255, 255, 0.28)';
      const gridColor = isLight ? 'rgba(0, 0, 0, 0.04)' : 'rgba(255, 255, 255, 0.04)';
      const axisColor = isLight ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.08)';

      ticks.forEach(val => {
        const tickY = bottomY - (val / 100) * (bottomY - topY);

        ctx.fillStyle = labelColor;
        ctx.fillText(val.toString(), axisX - 5, tickY);

        ctx.beginPath();
        ctx.strokeStyle = gridColor;
        ctx.lineWidth = 1;
        ctx.setLineDash([2, 3]);
        ctx.moveTo(axisX, tickY);
        ctx.lineTo(w - rightPad, tickY);
        ctx.stroke();
        ctx.setLineDash([]);
      });

      ctx.beginPath();
      ctx.strokeStyle = axisColor;
      ctx.lineWidth = 1;
      ctx.moveTo(axisX, topY);
      ctx.lineTo(axisX, bottomY);
      ctx.stroke();

      const plotW = w - axisX - rightPad;
      const step = plotW / (points.length - 1);

      ctx.beginPath();
      ctx.moveTo(axisX, bottomY);
      points.forEach((p, i) => {
        const x = axisX + i * step;
        const y = bottomY - (p / 100) * (bottomY - topY);
        ctx.lineTo(x, y);
      });
      ctx.lineTo(axisX + plotW, bottomY);
      ctx.closePath();
      ctx.fillStyle = actualFill;
      ctx.fill();

      ctx.beginPath();
      points.forEach((p, i) => {
        const x = axisX + i * step;
        const y = bottomY - (p / 100) * (bottomY - topY);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      });
      ctx.strokeStyle = actualStroke;
      ctx.lineWidth = 1.5;
      ctx.stroke();

      const lastX = axisX + (points.length - 1) * step;
      const lastY = bottomY - (next / 100) * (bottomY - topY);

      ctx.beginPath();
      ctx.arc(lastX, lastY, 2.75, 0, Math.PI * 2);
      ctx.fillStyle = actualStroke;
      ctx.fill();
      ctx.strokeStyle = isLight ? '#ffffff' : '#141416';
      ctx.lineWidth = 1.5;
      ctx.stroke();
    }

    chartDrawers.push(() => {
      syncDimensions();
      draw();
    });

    draw();
    setInterval(draw, 2500);
  }

  initTheme();
  fetchDeployments();
  initCharts();
  setInterval(fetchDeployments, 5000);
});
