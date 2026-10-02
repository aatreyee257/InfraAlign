(function () {
  "use strict";

  var DEFAULTS = { apiBase: "http://localhost:8080", apiKey: "" };
  var STATUSES = ["compliant", "drifted", "missing_in_cloud"];

  function loadConfig() {
    var cfg = Object.assign({}, DEFAULTS);
    try {
      var raw = localStorage.getItem("infraalign.config");
      if (raw) Object.assign(cfg, JSON.parse(raw));
    } catch (e) { /* private mode / blocked storage — fall back to defaults */ }
    return cfg;
  }

  function saveConfig(cfg) {
    try { localStorage.setItem("infraalign.config", JSON.stringify(cfg)); }
    catch (e) { /* best effort only */ }
  }

  var state = {
    config: loadConfig(),
    differences: null,
    lastError: null,
    remediating: {},        // bucketName -> true while a remediation call is in flight
    logs: {},               // bucketName -> {ok: bool, text: string}
    statusFilter: null,     // null = all, or one of STATUSES
    searchQuery: "",
    collapsedOverride: {}   // bucketName -> bool, only set once the user clicks a card
  };

  var autoRefresh = { enabled: false, intervalSec: 30, remaining: 30, timer: null };

  var app = document.getElementById("app");
  var connDot = document.getElementById("connDot");
  var connText = document.getElementById("connText");
  var refreshBtn = document.getElementById("refreshBtn");
  var settingsBtn = document.getElementById("settingsBtn");
  var settingsPanel = document.getElementById("settingsPanel");
  var apiBaseInput = document.getElementById("apiBaseInput");
  var apiKeyInput = document.getElementById("apiKeyInput");
  var searchInput = document.getElementById("searchInput");
  var autoRefreshToggle = document.getElementById("autoRefreshToggle");
  var autoRefreshInterval = document.getElementById("autoRefreshInterval");
  var countdownEl = document.getElementById("countdown");
  var toastStack = document.getElementById("toastStack");

  function pushToast(ok, text) {
    var el = document.createElement("div");
    el.className = "toast " + (ok ? "ok" : "err");
    el.textContent = text;
    toastStack.appendChild(el);
    setTimeout(function () {
      el.classList.add("leaving");
      setTimeout(function () { el.remove(); }, 180);
    }, 4000);
  }

  function setConn(mode, text) {
    connDot.className = "dot " + mode;
    connText.textContent = text;
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function statusLabel(status) {
    if (status === "compliant") return "Compliant";
    if (status === "drifted") return "Drifted";
    if (status === "missing_in_cloud") return "Missing";
    return status;
  }

  function groupByBucket(diffs) {
    var order = [];
    var map = {};
    diffs.forEach(function (d) {
      if (!map[d.bucket_name]) {
        map[d.bucket_name] = [];
        order.push(d.bucket_name);
      }
      map[d.bucket_name].push(d);
    });
    return order.map(function (name) { return { bucket: name, checks: map[name] }; });
  }

  function bucketWorstStatus(checks) {
    if (checks.some(function (c) { return c.status === "missing_in_cloud"; })) return "missing_in_cloud";
    if (checks.some(function (c) { return c.status === "drifted"; })) return "drifted";
    return "compliant";
  }

  function renderLoading() {
    app.innerHTML =
      '<div class="state-panel" id="stateLoading">' +
      '<div class="big-icon">···</div>' +
      '<h2>Scanning</h2>' +
      '<p>Parsing Terraform, querying AWS, comparing state.</p>' +
      '</div>';
  }

  function renderUnreachable(errMsg) {
    app.innerHTML =
      '<div class="state-panel">' +
      '<div class="big-icon">⚠</div>' +
      '<h2>Can’t reach the InfraAlign API</h2>' +
      '<p>Checked <code>' + escapeHtml(state.config.apiBase) + '/api/drift</code> and got no response. ' +
      'Start the backend, then hit Refresh.</p>' +
      '<pre>go run ./backend/cmd/server serve</pre>' +
      '<p style="margin-top:-8px">' + (errMsg ? escapeHtml(errMsg) : "") + '</p>' +
      '</div>';
  }

  function renderEmpty() {
    app.innerHTML =
      '<div class="state-panel">' +
      '<div class="big-icon">–</div>' +
      '<h2>No resources found</h2>' +
      '<p>The API responded, but the drift report is empty. Check that <code>terraform-samples</code> has resources and the AWS credentials the server is using can see your bucket.</p>' +
      '</div>';
  }

  function renderData() {
    var diffs = state.differences;
    var counts = { compliant: 0, drifted: 0, missing_in_cloud: 0 };
    diffs.forEach(function (d) { counts[d.status] = (counts[d.status] || 0) + 1; });

    var allGroups = groupByBucket(diffs);

    var query = state.searchQuery.trim().toLowerCase();
    var groups = allGroups.filter(function (g) {
      if (query && g.bucket.toLowerCase().indexOf(query) === -1) return false;
      if (state.statusFilter && bucketWorstStatus(g.checks) !== state.statusFilter) return false;
      return true;
    });

    var html = "";

    html += '<div class="summary">';
    STATUSES.forEach(function (s) {
      var cls = s === "compliant" ? "ok" : s === "drifted" ? "warn" : "crit";
      var active = state.statusFilter === s;
      html += '<button class="stat ' + cls + (active ? " active" : "") + '" data-stat-filter="' + s + '" ' +
        'aria-pressed="' + active + '">' +
        '<div class="n">' + counts[s] + '</div><div class="l">' + statusLabel(s) + (s === "missing_in_cloud" ? " in AWS" : "") + '</div>' +
        '</button>';
    });
    html += '</div>';

    if (groups.length === 0) {
      html += '<div class="no-match">No buckets match the current filter.</div>';
    }

    groups.forEach(function (g) {
      var worst = bucketWorstStatus(g.checks);
      var hasDrifted = g.checks.some(function (c) { return c.status === "drifted"; });
      var isRemediating = !!state.remediating[g.bucket];
      var log = state.logs[g.bucket];
      var override = state.collapsedOverride[g.bucket];
      var collapsed = override !== undefined ? override : (worst === "compliant");

      html += '<div class="bucket-card' + (collapsed ? " collapsed" : "") + '" data-bucket="' + escapeHtml(g.bucket) + '">';
      html += '<div class="bucket-head" data-toggle="' + escapeHtml(g.bucket) + '" role="button" tabindex="0" aria-expanded="' + (!collapsed) + '">';
      html += '<div class="bucket-head-left"><span class="chevron">▾</span>';
      html += '<div class="bucket-name">' + escapeHtml(g.bucket) + '</div></div>';
      html += '<div class="bucket-head-right">';
      html += '<span class="pill ' + worst + '">' + statusLabel(worst) + '</span>';
      if (hasDrifted) {
        html += '<button data-remediate="' + escapeHtml(g.bucket) + '" class="danger-ready" ' +
          (isRemediating ? "disabled" : "") + '>' +
          (isRemediating ? "Remediating…" : "Remediate") + '</button>';
      }
      html += '</div></div>';

      html += '<div class="bucket-checks"><div class="inner">';
      g.checks.forEach(function (c) {
        var mismatch = c.expected_val !== c.actual_val;
        html += '<div class="check-row">';
        html += '<div class="check-name">' + escapeHtml(c.attribute_name) + '</div>';
        html += '<div class="check-val"><span class="lbl">expected</span>' + escapeHtml(c.expected_val) + '</div>';
        html += '<div class="check-val actual ' + (mismatch ? "mismatch" : "match") + '">' +
          '<span class="lbl">actual</span>' + escapeHtml(c.actual_val) + '</div>';
        html += '<span class="pill ' + c.status + '">' + statusLabel(c.status) + '</span>';
        html += '</div>';
      });
      if (log) {
        html += '<div class="remediate-log ' + (log.ok ? "ok" : "err") + '">' + escapeHtml(log.text) + '</div>';
      }
      html += '</div></div>';

      html += '</div>';
    });

    app.innerHTML = html;

    app.querySelectorAll("[data-remediate]").forEach(function (btn) {
      btn.addEventListener("click", function (ev) {
        ev.stopPropagation();
        remediate(btn.getAttribute("data-remediate"));
      });
    });

    app.querySelectorAll("[data-stat-filter]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var s = btn.getAttribute("data-stat-filter");
        state.statusFilter = state.statusFilter === s ? null : s;
        render();
      });
    });

    app.querySelectorAll("[data-toggle]").forEach(function (headEl) {
      function toggle() {
        var bucket = headEl.getAttribute("data-toggle");
        var card = headEl.closest(".bucket-card");
        var currentlyCollapsed = card.classList.contains("collapsed");
        state.collapsedOverride[bucket] = !currentlyCollapsed;
        render();
      }
      headEl.addEventListener("click", toggle);
      headEl.addEventListener("keydown", function (ev) {
        if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); toggle(); }
      });
    });
  }

  function render() {
    if (state.lastError) { renderUnreachable(state.lastError); return; }
    if (state.differences === null) { renderLoading(); return; }
    if (state.differences.length === 0) { renderEmpty(); return; }
    renderData();
  }

  function fetchDrift() {
    setConn("pending", "scanning…");
    refreshBtn.disabled = true;

    fetch(state.config.apiBase + "/api/drift")
      .then(function (res) {
        if (!res.ok) throw new Error("HTTP " + res.status);
        return res.json();
      })
      .then(function (data) {
        state.differences = data || [];
        state.lastError = null;
        setConn("up", "connected");
        render();
      })
      .catch(function (err) {
        state.lastError = err.message || String(err);
        setConn("down", "unreachable");
        render();
      })
      .finally(function () {
        refreshBtn.disabled = false;
      });
  }

  function remediate(bucketName) {
    if (!state.config.apiKey) {
      state.logs[bucketName] = { ok: false, text: "No API key set. Open Settings (⚙) and enter INFRAALIGN_API_KEY." };
      render();
      return;
    }

    state.remediating[bucketName] = true;
    delete state.logs[bucketName];
    render();

    fetch(state.config.apiBase + "/api/remediate/" + encodeURIComponent(bucketName), {
      method: "POST",
      headers: { "X-API-Key": state.config.apiKey }
    })
      .then(function (res) {
        if (res.status === 401) throw new Error("Unauthorized — check the API key in Settings.");
        if (!res.ok) return res.text().then(function (t) { throw new Error(t || ("HTTP " + res.status)); });
        return res.json();
      })
      .then(function (fixed) {
        var n = (fixed || []).length;
        var text = n > 0
          ? "Fixed " + n + " attribute" + (n === 1 ? "" : "s") + " on " + bucketName
          : "Nothing to remediate on " + bucketName;
        state.logs[bucketName] = { ok: true, text: text + " — re-scanning…" };
        pushToast(true, text);
        delete state.remediating[bucketName];
        render();
        fetchDrift();
      })
      .catch(function (err) {
        var msg = err.message || String(err);
        state.logs[bucketName] = { ok: false, text: msg };
        pushToast(false, bucketName + ": " + msg);
        delete state.remediating[bucketName];
        render();
      });
  }

  // ---------- settings panel ----------

  function openSettings() {
    apiBaseInput.value = state.config.apiBase;
    apiKeyInput.value = state.config.apiKey;
    settingsPanel.hidden = false;
  }
  function closeSettings() { settingsPanel.hidden = true; }

  settingsBtn.addEventListener("click", function () {
    settingsPanel.hidden ? openSettings() : closeSettings();
  });
  document.getElementById("settingsCancel").addEventListener("click", closeSettings);
  document.getElementById("settingsSave").addEventListener("click", function () {
    state.config.apiBase = (apiBaseInput.value || DEFAULTS.apiBase).trim().replace(/\/+$/, "");
    state.config.apiKey = apiKeyInput.value;
    saveConfig(state.config);
    closeSettings();
    fetchDrift();
  });

  refreshBtn.addEventListener("click", function () {
    resetCountdown();
    fetchDrift();
  });

  // ---------- search ----------

  var searchDebounce = null;
  searchInput.addEventListener("input", function () {
    clearTimeout(searchDebounce);
    var value = searchInput.value;
    searchDebounce = setTimeout(function () {
      state.searchQuery = value;
      render();
    }, 120);
  });

  // ---------- auto-refresh ----------

  function resetCountdown() {
    autoRefresh.remaining = autoRefresh.intervalSec;
    updateCountdownDisplay();
  }

  function updateCountdownDisplay() {
    countdownEl.textContent = autoRefresh.enabled ? autoRefresh.remaining + "s" : "";
  }

  function tick() {
    if (!autoRefresh.enabled) return;
    autoRefresh.remaining -= 1;
    if (autoRefresh.remaining <= 0) {
      resetCountdown();
      fetchDrift();
    } else {
      updateCountdownDisplay();
    }
  }

  autoRefreshToggle.addEventListener("change", function () {
    autoRefresh.enabled = autoRefreshToggle.checked;
    resetCountdown();
  });

  autoRefreshInterval.addEventListener("change", function () {
    autoRefresh.intervalSec = parseInt(autoRefreshInterval.value, 10) || 30;
    resetCountdown();
  });

  if (autoRefresh.timer) clearInterval(autoRefresh.timer);
  autoRefresh.timer = setInterval(tick, 1000);

  // ---------- keyboard shortcuts ----------
  // "r" re-runs the scan, "/" focuses search — both skipped while typing in a field.

  document.addEventListener("keydown", function (ev) {
    var tag = (document.activeElement && document.activeElement.tagName) || "";
    var typing = tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA";

    if (!typing && (ev.key === "r" || ev.key === "R")) {
      ev.preventDefault();
      resetCountdown();
      fetchDrift();
    } else if (!typing && ev.key === "/") {
      ev.preventDefault();
      searchInput.focus();
    } else if (ev.key === "Escape" && document.activeElement === searchInput) {
      searchInput.value = "";
      state.searchQuery = "";
      searchInput.blur();
      render();
    }
  });

  fetchDrift();
})();
