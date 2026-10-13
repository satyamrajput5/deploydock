document.addEventListener('DOMContentLoaded', () => {
  const tbody = document.getElementById('deployments-tbody');
  const emptyState = document.getElementById('table-empty-state');
  const countBadge = document.getElementById('deployment-count-badge');
  const deployModal = document.getElementById('deploy-modal');
  const logsModal = document.getElementById('logs-modal');
  const deployForm = document.getElementById('deploy-form');
  const repoInput = document.getElementById('repo-url-input');
  const branchInput = document.getElementById('branch-input');
  const errorBanner = document.getElementById('deploy-error-banner');
  const terminalText = document.getElementById('terminal-output-text');
  const logsTargetId = document.getElementById('logs-target-id');
  const sidebar = document.getElementById('app-sidebar');
  const backdrop = document.getElementById('sidebar-backdrop');
  const toggleBtn = document.getElementById('sidebar-toggle');
  const themeToggleBtn = document.getElementById('theme-toggle-btn');
  const sunIcon = document.getElementById('theme-sun-icon');
  const moonIcon = document.getElementById('theme-moon-icon');

  const dashboardView = document.getElementById('view-dashboard');
  const detailView = document.getElementById('view-deployment-detail');

  let activeLogsDeploymentId = null;
  let modalLogsInterval = null;
  let currentDetailId = null;
  let detailLogsInterval = null;
  let detailAutoscroll = true;

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
    item.addEventListener('click', (e) => {
      document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
      item.classList.add('active');
      if (window.innerWidth <= 768) {
        sidebar.classList.remove('open');
        backdrop.classList.remove('visible');
      }

      const href = item.getAttribute('href');
      if (href === '#dashboard') {
        window.location.hash = '';
        history.pushState(null, '', '/');
        showDashboardView();
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
    if (modalLogsInterval) {
      clearInterval(modalLogsInterval);
      modalLogsInterval = null;
    }
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
    if (branchInput) branchInput.value = 'main';
    repoInput.focus();
  }

  function closeDeployModal() {
    deployModal.style.display = 'none';
  }

  deployForm?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const url = repoInput.value.trim();
    const branch = branchInput ? (branchInput.value.trim() || 'main') : 'main';
    if (!url) return;

    const submitBtn = document.getElementById('modal-submit-btn');
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Deploying...</span>';
    errorBanner.style.display = 'none';

    try {
      const res = await fetch('/deployments', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ repo_url: url, branch: branch })
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to submit deployment');
      }

      closeDeployModal();
      await fetchDeployments();

      const createdId = data.deployment?.id;
      if (createdId) {
        navigateToDeployment(createdId);
      }
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
      const res = await fetch('/deployments');
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
        ? `<a href="http://localhost:${item.port}" target="_blank" rel="noopener noreferrer" onclick="event.stopPropagation()">http://localhost:${item.port}</a>` 
        : '<span style="color: var(--fg-faint);">—</span>';

      const created = item.created_at ? new Date(item.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '—';
      const branchName = item.branch || 'main';
      const commitDisplay = item.commit_sha ? `<span class="commit-tag" title="Commit">${item.commit_sha.substring(0, 7)}</span>` : '';
      const versionText = item.current_build_id ? `v${item.current_build_id}` : (item.build_number ? `v${item.build_number}` : 'v1');

      return `
        <tr data-id="${item.id}" class="clickable-row" onclick="window.onRowClick(event, ${item.id})">
          <td class="cell-id">#${item.id}</td>
          <td>
            <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
              <a class="cell-repo" href="${item.repo_url}" target="_blank" rel="noopener noreferrer" title="${item.repo_url}" onclick="event.stopPropagation()">
                ${item.repo_url.replace('https://github.com/', '')}
              </a>
              <span class="branch-tag" title="Branch">${branchName}</span>
              ${commitDisplay}
            </div>
          </td>
          <td><span class="version-tag">${versionText}</span></td>
          <td>
            <span class="status-badge ${statusClass}">${item.status}</span>
            ${item.docker_status ? `<span class="docker-status-sub" title="Docker: ${item.docker_status}">${item.docker_status}</span>` : ''}
          </td>
          <td class="cell-port">${portDisplay}</td>
          <td class="cell-url">${liveUrl}</td>
          <td style="color: var(--fg-muted); font-size: 0.8rem; font-family: var(--font-mono);">${created}</td>
          <td style="text-align: right;">
            <div style="display: inline-flex; gap: 4px;" onclick="event.stopPropagation()">
              <button class="btn btn-secondary btn-sm" onclick="window.redeploy(${item.id})" title="Trigger build">Redeploy</button>
              <button class="btn btn-secondary btn-sm" onclick="window.viewLogs(${item.id})">Logs</button>
              <button class="btn btn-danger btn-sm" onclick="window.deleteDeployment(${item.id})">Delete</button>
            </div>
          </td>
        </tr>
      `;
    }).join('');
  }

  window.onRowClick = function(event, id) {
    if (event.target.closest('button') || event.target.closest('a')) return;
    navigateToDeployment(id);
  };

  function navigateToDeployment(id) {
    window.location.hash = `#/deployment/${id}`;
  }

  function showDashboardView() {
    currentDetailId = null;
    if (detailLogsInterval) {
      clearInterval(detailLogsInterval);
      detailLogsInterval = null;
    }
    detailView.style.display = 'none';
    dashboardView.style.display = 'block';
    resizeAllCanvases();
  }

  async function showDeploymentDetail(id) {
    currentDetailId = id;
    dashboardView.style.display = 'none';
    detailView.style.display = 'block';

    window.scrollTo({ top: 0, behavior: 'smooth' });

    await loadDeploymentDetailData(id);
    initDetailCharts();

    if (detailLogsInterval) clearInterval(detailLogsInterval);
    fetchDetailLogs(id);
    detailLogsInterval = setInterval(() => {
      if (currentDetailId === id) {
        fetchDetailLogs(id);
      }
    }, 1500);
  }

  function handleRouting() {
    const hash = window.location.hash;
    const path = window.location.pathname;

    let match = hash.match(/#\/?deployment\/(\d+)/);
    if (!match) {
      match = path.match(/\/deployment\/(\d+)/);
    }

    if (match && match[1]) {
      showDeploymentDetail(parseInt(match[1], 10));
    } else {
      showDashboardView();
    }
  }

  window.addEventListener('hashchange', handleRouting);
  window.addEventListener('popstate', handleRouting);

  document.getElementById('btn-back-to-dashboard')?.addEventListener('click', () => {
    window.location.hash = '';
    history.pushState(null, '', '/');
    showDashboardView();
  });

  async function loadDeploymentDetailData(id) {
    try {
      const res = await fetch(`/deployments/${id}`);
      if (!res.ok) throw new Error('Deployment not found');
      const data = await res.json();

      document.getElementById('detail-id-tag').textContent = `#${data.id}`;
      const repoName = data.repo_url.replace('https://github.com/', '');
      document.getElementById('detail-repo-name').textContent = repoName;
      document.getElementById('detail-branch-tag').textContent = data.branch || 'main';
      const versionText = data.current_build_id ? `v${data.current_build_id}` : (data.build_number ? `v${data.build_number}` : 'v1');
      document.getElementById('detail-version-tag').textContent = versionText;

      const statusBadge = document.getElementById('detail-status-badge');
      statusBadge.className = `status-badge status-${data.status || 'pending'}`;
      statusBadge.textContent = data.status || 'pending';

      document.getElementById('detail-port-val').textContent = data.port ? `:${data.port}` : '—';
      document.getElementById('detail-container-val').textContent = data.container_id ? data.container_id.substring(0, 12) : '—';
      document.getElementById('detail-commit-val').textContent = data.commit_sha ? data.commit_sha.substring(0, 7) : '—';
      const ghLink = document.getElementById('detail-github-link');
      ghLink.href = data.repo_url;
      ghLink.textContent = data.repo_url;
      document.getElementById('detail-created-val').textContent = data.created_at ? new Date(data.created_at).toLocaleString() : '—';
      const dockerVal = document.getElementById('detail-docker-val');
      if (dockerVal) {
        dockerVal.textContent = data.docker_status || (data.status === 'running' ? 'Active' : '—');
      }

      const visitBtn = document.getElementById('detail-visit-btn');
      if (data.port) {
        visitBtn.href = `http://localhost:${data.port}`;
        visitBtn.style.display = 'inline-flex';
      } else {
        visitBtn.style.display = 'none';
      }

      const redeployBtn = document.getElementById('detail-redeploy-btn');
      if (redeployBtn) {
        redeployBtn.onclick = async () => {
          await window.redeploy(data.id);
          loadDeploymentDetailData(data.id);
        };
      }

      const deleteBtn = document.getElementById('detail-delete-btn');
      if (deleteBtn) {
        deleteBtn.onclick = async () => {
          await window.deleteDeployment(data.id);
          window.location.hash = '';
          history.pushState(null, '', '/');
          showDashboardView();
        };
      }

      loadBuildsTable(id);
    } catch (err) {
      console.error('Failed loading detail:', err);
    }
  }

  async function loadBuildsTable(id) {
    try {
      const res = await fetch(`/deployments/${id}/builds`);
      if (!res.ok) return;
      const data = await res.json();
      const builds = data.builds || [];
      document.getElementById('detail-builds-count').textContent = builds.length;

      const tbody = document.getElementById('detail-builds-tbody');
      if (builds.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; color:var(--fg-faint); padding:20px;">No builds recorded yet.</td></tr>';
        return;
      }

      tbody.innerHTML = builds.map(b => {
        const commitTag = b.commit_sha ? `<span class="commit-tag">${b.commit_sha.substring(0, 7)}</span>` : '<span style="color:var(--fg-faint)">—</span>';
        const errText = b.error_message || 'OK';
        const isErr = b.status === 'failed' || !!b.error_message;

        return `
          <tr>
            <td><span class="version-tag">#${b.id}</span></td>
            <td style="font-family:var(--font-mono); font-size:0.75rem;">${b.trigger_type}</td>
            <td>${commitTag}</td>
            <td><span class="status-badge status-${b.status}">${b.status}</span></td>
            <td style="color:${isErr ? 'var(--danger)' : 'var(--fg-muted)'}; font-size:0.75rem; max-width:240px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${errText}">
              ${errText}
            </td>
            <td style="font-family:var(--font-mono); font-size:0.75rem; color:var(--fg-faint);">${b.created_at ? new Date(b.created_at).toLocaleTimeString() : '—'}</td>
          </tr>
        `;
      }).join('');
    } catch (err) {
      console.error('Failed loading builds:', err);
    }
  }

  async function fetchDetailLogs(id) {
    const textEl = document.getElementById('detail-terminal-text');
    const countEl = document.getElementById('detail-log-count');
    const container = document.getElementById('detail-terminal-container');

    try {
      const res = await fetch(`/deployments/${id}/logs`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to fetch logs');

      const logs = data.logs || [];
      if (countEl) countEl.textContent = `${logs.length} lines`;

      if (logs.length === 0) {
        textEl.textContent = `Deployment #${id} queued.\nAwaiting build logs...`;
        return;
      }

      const formatted = logs.map(l => {
        const time = l.created_at ? new Date(l.created_at).toLocaleTimeString() : '';
        return `[${time}] ${l.message}`;
      }).join('\n');

      if (textEl.textContent !== formatted) {
        textEl.textContent = formatted;
        if (detailAutoscroll && container) {
          requestAnimationFrame(() => {
            container.scrollTop = container.scrollHeight;
          });
        }
      } else if (detailAutoscroll && container) {
        container.scrollTop = container.scrollHeight;
      }
    } catch (err) {
      textEl.textContent = `Error loading logs: ${err.message}`;
    }
  }

  document.getElementById('detail-scroll-logs-btn')?.addEventListener('click', (e) => {
    detailAutoscroll = !detailAutoscroll;
    e.target.textContent = `Auto-scroll: ${detailAutoscroll ? 'ON' : 'OFF'}`;
    if (detailAutoscroll) {
      const container = document.getElementById('detail-terminal-container');
      if (container) {
        requestAnimationFrame(() => {
          container.scrollTop = container.scrollHeight;
        });
      }
    }
  });

  document.getElementById('detail-clear-logs-btn')?.addEventListener('click', () => {
    const textEl = document.getElementById('detail-terminal-text');
    if (textEl) textEl.textContent = '';
    const countEl = document.getElementById('detail-log-count');
    if (countEl) countEl.textContent = '0 lines';
  });

  document.getElementById('detail-refresh-logs-btn')?.addEventListener('click', () => {
    if (currentDetailId) fetchDetailLogs(currentDetailId);
  });

  window.redeploy = async function(id) {
    try {
      const res = await fetch(`/deployments/${id}/redeploy`, { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to trigger redeploy');
      await fetchDeployments();
      if (currentDetailId === id) {
        loadDeploymentDetailData(id);
      }
    } catch (err) {
      alert(`Error triggering redeploy: ${err.message}`);
    }
  };

  window.viewLogs = async function(id) {
    activeLogsDeploymentId = id;
    logsTargetId.textContent = `#${id}`;
    terminalText.textContent = 'Loading live logs...';
    logsModal.style.display = 'flex';
    await fetchLogs(id);

    if (modalLogsInterval) clearInterval(modalLogsInterval);
    modalLogsInterval = setInterval(() => {
      if (activeLogsDeploymentId === id && logsModal.style.display !== 'none') {
        fetchLogs(id);
      }
    }, 1500);
  };

  async function fetchLogs(id) {
    try {
      const res = await fetch(`/deployments/${id}/logs`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to fetch logs');

      if (!data.logs || data.logs.length === 0) {
        terminalText.textContent = `Deployment #${id} queued.\nAwaiting build execution...`;
        return;
      }

      const formatted = data.logs.map(l => {
        const time = l.created_at ? new Date(l.created_at).toLocaleTimeString() : '';
        return `[${time}] ${l.message}`;
      }).join('\n');

      if (terminalText.textContent !== formatted) {
        terminalText.textContent = formatted;
        requestAnimationFrame(() => {
          terminalText.scrollTop = terminalText.scrollHeight;
        });
      }
    } catch (err) {
      terminalText.textContent = `Error loading logs: ${err.message}`;
    }
  }

  window.deleteDeployment = async function(id) {
    if (!confirm(`Delete deployment #${id}?`)) return;

    try {
      const res = await fetch(`/deployments/${id}`, { method: 'DELETE' });
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

  let detailChartsInitialized = false;
  function initDetailCharts() {
    if (detailChartsInitialized) return;
    detailChartsInitialized = true;

    setupCanvasChart('detail-cpu-chart-canvas', 'var(--chart-cpu)', 'rgba(6, 182, 212, 0.08)', (val) => {
      const el = document.getElementById('detail-cpu-pct');
      if (el) el.textContent = `${val.toFixed(1)}%`;
    });

    setupCanvasChart('detail-ram-chart-canvas', 'var(--chart-ram)', 'rgba(139, 92, 246, 0.08)', (val) => {
      const el = document.getElementById('detail-ram-val');
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
  handleRouting();
  setInterval(fetchDeployments, 5000);
});
