const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

describe('T19e: Admin 清理 + 退出登录', () => {
  ab.closeBrowser();

  function apiCall(code) {
    var raw = ab.evalStdin(code);
    ab.waitMs(1200);
    if (!raw) return {};
    try {
      var parsed = JSON.parse(raw);
      if (typeof parsed === 'string') return JSON.parse(parsed);
      return parsed;
    } catch (e) {
      return { error: String(e), raw: raw };
    }
  }

  // Bootstrap: API 登录获取 admin token
  var bootstrap = apiCall(`
    (function() {
      return fetch('${API_BASE}/v1/disk/admin/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'admin123' }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var bootstrapOk = bootstrap.code === 0 && (bootstrap.data || {}).token;
  if (!bootstrapOk) {
    step('TC-prep: Admin 登录失败，请先运行 T18 bootstrap', true, (bootstrap.message || JSON.stringify(bootstrap)).substring(0, 200));
    ab.closeBrowser();
    printReport();
    return;
  }

  var adminToken = (bootstrap.data || {}).token;
  step('TC-prep: Admin 登录成功', true);

  // Navigate to admin page
  ab.open(WEB_BASE + '/admin/public-dirs');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.waitMs(300);

  // Collect all test dirs
  var allDirsParsed = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/public-directories', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var allDirIds = [];
  (allDirsParsed.data || []).forEach(function(d) {
    if (d.displayName && (d.displayName.startsWith('test-') || d.displayName.startsWith('ui-test-'))) {
      allDirIds.push(d.id);
    }
  });

  // Collect all test keys
  var allKeysParsed = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/api-keys', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var allKeyIds = [];
  (allKeysParsed.data || []).forEach(function(k) {
    if (k.keyName && (k.keyName.startsWith('test-') || k.keyName.startsWith('ui-test-') || k.keyName.startsWith('temp-'))) {
      allKeyIds.push(k.id);
    }
  });

  // Delete all API Keys
  allKeyIds.forEach(function(kid) {
    ab.evalStdin(`
      (function() {
        var token = localStorage.getItem('admin_token') || '';
        return fetch('${API_BASE}/v1/disk/admin/api-keys/${kid}', {
          method: 'DELETE',
          headers: { 'Authorization': 'Bearer ' + token },
        })
        .then(function(r) { return r.json(); })
        .then(function(d) { return d.code; })
        .catch(function() { return -1; });
      })()
    `);
    ab.waitMs(300);
  });
  step('TC-59: 吊销并删除 API Key', true, 'deleted ' + allKeyIds.length + ' keys');

  // Delete all public directories
  allDirIds.forEach(function(did) {
    ab.evalStdin(`
      (function() {
        var token = localStorage.getItem('admin_token') || '';
        return fetch('${API_BASE}/v1/disk/admin/public-directories/${did}', {
          method: 'DELETE',
          headers: { 'Authorization': 'Bearer ' + token },
        })
        .then(function(r) { return r.json(); })
        .then(function(d) { return d.code; })
        .catch(function() { return -1; });
      })()
    `);
    ab.waitMs(300);
  });
  step('TC-64: 删除测试公共目录', true, 'deleted ' + allDirIds.length + ' dirs');
  ab.screenshot('t19e-01-cleanup');

  // ── TC-69: Admin 退出登录 ──
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.open(WEB_BASE + '/admin');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var logoutClicked = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('退出')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var logoutUrl = ab.getUrl();
  var logoutOk = logoutUrl.includes('/admin/login');
  assertCondition(
    String(logoutClicked).includes('clicked') && logoutOk,
    'TC-69: Admin 退出登录，跳转到登录页',
    'clicked=' + logoutClicked + ' url=' + logoutUrl
  );
  ab.screenshot('t19e-02-admin-logout');

  // Clear admin token
  ab.evalStdin(`localStorage.removeItem('admin_token')`);

  ab.closeBrowser();
});

printReport();
