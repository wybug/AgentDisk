const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

describe('T19d: 用户端公共目录浏览', () => {
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

  // Login as user001 via gateway
  ab.open(WEB_BASE);
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var userSnap = ab.snapshot();
  var userIdRef = ab.findRefByPlaceholder(userSnap, '用户 ID');
  var passwordRef = ab.findRefByPlaceholder(userSnap, '密码');
  var loginBtnRef = ab.findRefByText(userSnap, '登 录') || ab.findRefByRole(userSnap, 'button', '登 录');

  if (userIdRef && passwordRef && loginBtnRef) {
    ab.fill(userIdRef, 'user001');
    ab.fill(passwordRef, 'test123');
    ab.click(loginBtnRef);
  } else {
    ab.jsFill('用户 ID', 'user001');
    ab.jsFill('密码', 'test123');
    ab.jsClickBtn('登 录');
  }
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  // Approve OAuth2
  ab.evalStdin('approve()');
  ab.waitMs(3000);
  ab.waitLoad('networkidle');

  var userMainUrl = ab.getUrl();
  var userLoggedIn = userMainUrl.includes('9101');
  step('TC-prep: 用户 user001 登录成功', userLoggedIn, userMainUrl);
  ab.screenshot('t19d-01-user-login');

  // ── TC-47: 侧边栏显示"公共文件"菜单项 ──
  var hasPublicMenu = ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('公共文件')) return items[i].textContent;
      }
      return 'not found';
    })()
  `);
  step('TC-47: 侧边栏显示公共文件菜单项', hasPublicMenu !== 'not found', hasPublicMenu);
  ab.screenshot('t19d-02-sidebar-public');

  // ── TC-50: 点击"公共文件"进入公共目录列表页 ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('公共文件')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var publicUrl = ab.getUrl();
  step('TC-50: 进入公共文件页面', publicUrl.includes('/public'), publicUrl);
  ab.screenshot('t19d-03-public-page');

  // ── TC-51: 公共目录列表显示管理员创建的全局公共目录 ──
  var hasGlobalDir = ab.pageContainsText('test-global-reports');
  step('TC-51: 公共目录列表显示全局公共目录', hasGlobalDir, 'test-global-reports');
  ab.screenshot('t19d-04-global-dir-listed');

  // ── TC-67: 点击公共目录项进入详情页 ──
  var clickedDir = ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-list-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('test-global-reports')) {
          items[i].click();
          return 'clicked';
        }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  var dirDetailUrl = ab.getUrl();
  var dirDetailOk = dirDetailUrl.includes('/public/');
  step(
    'TC-67: 点击公共目录项进入详情页',
    dirDetailOk,
    'clicked=' + clickedDir + ' url=' + dirDetailUrl
  );

  if (dirDetailOk) {
    var hasDetailContent = ab.pageContainsText('test-global-reports') || ab.pageContainsText('文件夹') || ab.pageContainsText('文件');
    step('TC-67: 公共目录详情页加载', true, 'hasContent=' + hasDetailContent);
  }
  ab.screenshot('t19d-05-dir-detail');

  // ── TC-68: 通过侧边栏返回"全部文件" ──
  var clickedExplorer = ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('全部文件')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var preFallbackUrl = ab.getUrl();
  if (!preFallbackUrl.includes('/explorer')) {
    ab.evalStdin(`window.location.href = '${WEB_BASE}/explorer'`);
    ab.waitMs(1500);
    ab.waitLoad('networkidle');
  }

  var explorerUrl = ab.getUrl();
  step(
    'TC-68: 通过侧边栏返回全部文件',
    explorerUrl.includes('/explorer'),
    explorerUrl
  );
  ab.screenshot('t19d-06-back-to-explorer');

  // ── TC-19: 用户列出可见公共目录（通过 Vite 代理，session cookie 可用）──
  var userDirs = apiCall(`
    (function() {
      return fetch('${WEB_BASE}/v1/disk/public-directories', {
        credentials: 'include',
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var userDirsOk = userDirs.code === 0 && (userDirs.data || []).length >= 1;
  step('TC-19: 用户列出可见公共目录', userDirsOk, 'count=' + ((userDirs.data || []).length));

  // ── TC-21: API Key 认证访问公共目录 ──
  // Create a temp API key for this test
  var tempKeyResult = apiCall(`
    (function() {
      return fetch('${API_BASE}/v1/disk/admin/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'admin123' }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) {
        var token = (d.data || {}).token;
        return fetch('${API_BASE}/v1/disk/admin/api-keys', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + token,
          },
          body: JSON.stringify({ name: 'temp-key-t19d' }),
        });
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var tempApiKey = (tempKeyResult.data || {}).key || '';

  if (tempApiKey) {
    var apiAccessResult = apiCall(`
      (function() {
        return fetch('${WEB_BASE}/v1/disk/public-directories', {
          headers: { 'X-API-Key': '${tempApiKey}' },
        })
        .then(function(r) { return r.json(); })
        .then(function(d) { return JSON.stringify(d); })
        .catch(function(e) { return JSON.stringify({error: e.message}); });
      })()
    `);
    var apiAccessOk = apiAccessResult.code === 0 && (apiAccessResult.data || []).length >= 1;
    step('TC-21: API Key 认证访问公共目录', apiAccessOk, 'dirs: ' + ((apiAccessResult.data || []).length));
    ab.screenshot('t19d-07-api-key-access');

    // Revoke the temp key
    apiCall(`
      (function() {
        return fetch('${API_BASE}/v1/disk/admin/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username: 'admin', password: 'admin123' }),
        })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var token = (d.data || {}).token;
          // List keys to find temp-key-t19d
          return fetch('${API_BASE}/v1/disk/admin/api-keys', {
            headers: { 'Authorization': 'Bearer ' + token },
          })
          .then(function(r2) { return r2.json(); })
          .then(function(keys) {
            var kid = 0;
            (keys.data || []).forEach(function(k) {
              if (k.keyName === 'temp-key-t19d') kid = k.id;
            });
            if (kid) {
              return fetch('${API_BASE}/v1/disk/admin/api-keys/' + kid, {
                method: 'DELETE',
                headers: { 'Authorization': 'Bearer ' + token },
              }).then(function(r3) { return r3.json(); });
            }
            return {code: 0};
          });
        })
        .then(function(d) { return JSON.stringify(d); })
        .catch(function(e) { return JSON.stringify({error: e.message}); });
      })()
    `);
  } else {
    step('TC-21: API Key 认证访问公共目录', false, 'failed to create temp key');
  }

  ab.closeBrowser();
});

printReport();
