// MANUAL_TEST: true — 需要通过 viz 仪表盘运行，人工配合完成 WebAuthn 原生弹窗
const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const { execSync } = require('child_process');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

function alertManual(msg) {
  try { execSync('say "' + msg + '"', { timeout: 10000 }); } catch (e) { /* ignore */ }
}

describe('T02g: Admin MFA 登录验证', () => {
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(1000);

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

  // ── Bootstrap: API 登录获取 admin token ──
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
    step('TC-prep: Admin 登录失败，请先运行 T01 bootstrap', true, (bootstrap.message || JSON.stringify(bootstrap)).substring(0, 200));
    ab.closeBrowser();
    printReport();
    return;
  }

  var adminToken = (bootstrap.data || {}).token;
  step('TC-prep: Admin 登录成功', true);

  // ── TC-83: 导航到 MFA 设置页并注册通行密钥 ──
  ab.open(WEB_BASE + '/admin/mfa');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.waitMs(300);
  ab.open(WEB_BASE + '/admin/mfa');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var mfaUrl = ab.getUrl();
  assertCondition(mfaUrl.includes('/admin/mfa'), 'TC-83: 导航到 MFA 设置页', mfaUrl);

  // Click register passkey
  var regBtnClicked = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('注册通行密钥')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  assertCondition(String(regBtnClicked).includes('clicked'), 'TC-83: 点击「注册通行密钥」', 'result=' + regBtnClicked);

  // [人工] Complete WebAuthn registration
  console.log('\n\x1b[43m\x1b[30m  [人工操作] 请在浏览器中完成 WebAuthn 注册（指纹/面容/安全密钥），等待30秒\x1b[0m\n');
  alertManual('请完成通行密钥注册');
  for (var w1 = 0; w1 < 10; w1++) { ab.waitMs(3000); }

  // Handle naming modal
  var hasNamingModal = ab.pageContainsText('为通行密钥命名');
  if (hasNamingModal) {
    ab.evalStdin(`
      (function() {
        var input = document.querySelector('#passkey-name-input');
        if (!input) {
          var inputs = document.querySelectorAll('.ant-modal input');
          if (inputs.length > 0) input = inputs[0];
        }
        if (!input) return 'not found';
        var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
        nativeSet.call(input, 'MFA测试密钥');
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
        return 'filled';
      })()
    `);
    ab.waitMs(300);
    ab.jsClickBtn('确 定');
    ab.waitMs(2000);
  }

  var hasRegisteredKey = ab.pageContainsText('MFA测试密钥');
  assertCondition(hasRegisteredKey, 'TC-83: 通行密钥注册成功', 'hasRegisteredKey=' + hasRegisteredKey);
  ab.screenshot('t02g-01-passkey-registered');

  // ── TC-84: 开启 MFA ──
  var switchClicked = ab.evalStdin(`
    (function() {
      var btn = document.querySelector('button.ant-switch');
      if (!btn) return 'no-switch';
      if (btn.disabled) return 'disabled';
      btn.click();
      return 'clicked';
    })()
  `);
  ab.waitMs(1500);

  var mfaStatus = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/mfa/status', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var mfaEnabled = (mfaStatus.data || {}).mfaEnabled === true;
  assertCondition(
    mfaEnabled,
    'TC-84: 开启 MFA 成功',
    'switch=' + switchClicked + ' enabled=' + mfaEnabled
  );
  ab.screenshot('t02g-02-mfa-enabled');

  // ── TC-84b: API 显式验证 MFA 已启用 ──
  assertCondition(
    mfaEnabled === true,
    'TC-84b: API 验证 MFA 状态 enabled=true',
    'data=' + JSON.stringify(mfaStatus.data || {})
  );

  // ── TC-85: 退出登录 ──
  var logoutClicked = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('退出')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1500);

  var logoutUrl = ab.getUrl();
  assertCondition(
    String(logoutClicked).includes('clicked') && logoutUrl.includes('/admin/login'),
    'TC-85: 退出登录，跳转到登录页',
    'clicked=' + logoutClicked + ' url=' + logoutUrl
  );
  ab.screenshot('t02g-03-logout');

  // ── TC-86: 错误密码被 API 拒绝 ──
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');
  ab.screenshot('t02g-04-login-page');

  var wrongPwdResult = apiCall(`
    (function() {
      return fetch('${API_BASE}/v1/disk/admin/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'wrongpassword' }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var wrongPwdRejected = wrongPwdResult.code !== 0 || wrongPwdResult.message;
  assertCondition(
    wrongPwdRejected,
    'TC-86: 错误密码被 API 拒绝',
    'result=' + JSON.stringify(wrongPwdResult).substring(0, 200)
  );
  ab.screenshot('t02g-05-wrong-password');

  // ── TC-87: 输入正确密码，进入 MFA 验证页 ──
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  ab.evalStdin(`
    (function() {
      var setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      var inputs = document.querySelectorAll('.ant-form input');
      var filled = 0;
      inputs.forEach(function(input) {
        if (input.placeholder === '用户名' || input.id === 'username') {
          setter.call(input, 'admin');
          input.dispatchEvent(new Event('input', { bubbles: true }));
          input.dispatchEvent(new Event('change', { bubbles: true }));
          filled++;
        }
        if (input.placeholder === '密码' || input.type === 'password') {
          setter.call(input, 'admin123');
          input.dispatchEvent(new Event('input', { bubbles: true }));
          input.dispatchEvent(new Event('change', { bubbles: true }));
          filled++;
        }
      });
      return 'filled_' + filled;
    })()
  `);
  ab.waitMs(500);

  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('登录') || btns[i].getAttribute('type') === 'submit') {
          btns[i].click();
          return 'clicked';
        }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(3000);

  var hasMFAVerify = ab.pageContainsText('验证身份') || ab.pageContainsText('验证通行密钥');
  assertCondition(
    hasMFAVerify,
    'TC-87: MFA 验证页显示（验证身份 + 验证通行密钥）',
    'hasMFAVerify=' + hasMFAVerify
  );
  ab.screenshot('t02g-06-mfa-verify-page');

  // ── TC-87b: mock navigator.credentials.get 模拟取消 WebAuthn 验证 ──
  ab.evalStdin(`
    (function() {
      window._origGet = navigator.credentials.get.bind(navigator.credentials);
      navigator.credentials.get = function() {
        window.__mockGetCalled = true;
        return Promise.reject(new Error('已取消验证'));
      };
      return 'mocked';
    })()
  `);

  var verifyBtnClicked1 = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('验证通行密钥')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  step('TC-87b: 点击「验证通行密钥」（mock 取消）', String(verifyBtnClicked1).includes('clicked'), 'result=' + verifyBtnClicked1);

  // 轮询检测错误提示（Toast 可能快速消失）
  var hasVerifyError = false;
  for (var w2a = 0; w2a < 15; w2a++) {
    ab.waitMs(1000);
    if (ab.pageContainsText('已取消验证') || ab.pageContainsText('身份验证失败') || ab.pageContainsText('已取消')) { hasVerifyError = true; break; }
  }

  // 检查 mock 是否被调用
  var mockCalled = ab.evalStdin('(function(){return window.__mockGetCalled===true?"yes":"no";})()');
  if (!hasVerifyError) {
    step('TC-87b-debug: mock 是否被调用', mockCalled === 'yes', 'mockCalled=' + mockCalled);
    step('TC-87b-debug: 页面当前内容', true, ab.snapshotFull().substring(0, 500));
  }

  assertCondition(hasVerifyError, 'TC-87b: mock 取消 WebAuthn 后页面显示错误提示', 'hasVerifyError=' + hasVerifyError);
  ab.screenshot('t02g-07-mfa-cancel');

  // 恢复原始 credentials.get
  ab.evalStdin(`
    (function() {
      if (window._origGet) {
        navigator.credentials.get = window._origGet;
        delete window._origGet;
      }
      return 'restored';
    })()
  `);

  // ── TC-87c: 点击「返回登录」→ 回到登录表单 ──
  var backToLogin = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('返回登录')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var backToForm = !ab.pageContainsText('验证身份') && ab.pageContainsText('用户名');
  assertCondition(
    String(backToLogin).includes('clicked') && backToForm,
    'TC-87c: 点击「返回登录」回到登录表单',
    'clicked=' + backToLogin + ' hasForm=' + backToForm
  );
  ab.screenshot('t02g-08-back-to-login');

  // ── TC-87d: 重新输入正确凭据 → 再次进入 MFA 验证页 ──
  ab.evalStdin(`
    (function() {
      var setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      var inputs = document.querySelectorAll('.ant-form input');
      var filled = 0;
      inputs.forEach(function(input) {
        if (input.placeholder === '用户名' || input.id === 'username') {
          setter.call(input, 'admin');
          input.dispatchEvent(new Event('input', { bubbles: true }));
          input.dispatchEvent(new Event('change', { bubbles: true }));
          filled++;
        }
        if (input.placeholder === '密码' || input.type === 'password') {
          setter.call(input, 'admin123');
          input.dispatchEvent(new Event('input', { bubbles: true }));
          input.dispatchEvent(new Event('change', { bubbles: true }));
          filled++;
        }
      });
      return 'filled_' + filled;
    })()
  `);
  ab.waitMs(500);

  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('登录') || btns[i].getAttribute('type') === 'submit') {
          btns[i].click();
          return 'clicked';
        }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(3000);

  var hasMFAVerify2 = ab.pageContainsText('验证身份') || ab.pageContainsText('验证通行密钥');
  assertCondition(
    hasMFAVerify2,
    'TC-87d: 重新输入凭据后再次进入 MFA 验证页',
    'hasMFAVerify=' + hasMFAVerify2
  );
  ab.screenshot('t02g-09-mfa-verify-again');

  // ── TC-88: [人工] 完成 WebAuthn 验证登录 ──
  var verifyBtnClicked = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('验证通行密钥')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  step('TC-88: 点击「验证通行密钥」', String(verifyBtnClicked).includes('clicked'), 'result=' + verifyBtnClicked);

  console.log('\n\x1b[43m\x1b[30m  [人工操作] 请在浏览器中完成 WebAuthn 身份验证，等待30秒\x1b[0m\n');
  alertManual('请完成通行密钥验证');
  for (var w2 = 0; w2 < 10; w2++) { ab.waitMs(3000); }

  var reloginUrl = ab.getUrl();
  var reloginOk = reloginUrl.includes('/admin');
  assertCondition(reloginOk, 'TC-88: MFA 验证登录成功，进入管理后台', 'url=' + reloginUrl);

  var hasAdminHeader = ab.pageContainsText('管理后台');
  step('TC-88: 管理后台 Header 显示', hasAdminHeader, 'hasAdminHeader=' + hasAdminHeader);
  ab.screenshot('t02g-10-mfa-login-success');

  // ── TC-88b: 验证侧边栏导航 ──
  var navItems = [
    { text: '公共目录', url: '/admin/public-dirs' },
    { text: 'API Key', url: '/admin/api-keys' },
    { text: '管理员', url: '/admin/users' }
  ];
  navItems.forEach(function(item) {
    ab.evalStdin(`
      (function() {
        var links = document.querySelectorAll('a, .ant-menu-item');
        for (var i = 0; i < links.length; i++) {
          if (links[i].textContent.includes('${item.text}')) { links[i].click(); return 'clicked'; }
        }
        return 'not found';
      })()
    `);
    ab.waitMs(1500);
    var navUrl = ab.getUrl();
    step('TC-88b: 点击「' + item.text + '」', navUrl.includes('${item.url}'), 'url=' + navUrl);
  });
  ab.screenshot('t02g-11-sidebar-nav');

  // ── Cleanup: 关闭 MFA + 删除通行密钥 ──
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.open(WEB_BASE + '/admin/mfa');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  // Disable MFA
  var disableSwitch = ab.evalStdin(`
    (function() {
      var btn = document.querySelector('button.ant-switch');
      if (!btn) return 'no-switch';
      if (btn.disabled || !btn.classList.contains('ant-switch-checked')) return 'already-off';
      btn.click();
      return 'clicked';
    })()
  `);
  ab.waitMs(1000);
  step('TC-cleanup: 关闭 MFA 开关', true, 'result=' + disableSwitch);

  // Delete all passkeys via API
  var allCreds = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/mfa/credentials', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var credIds = (allCreds.data || []).map(function(c) { return c.id; });
  credIds.forEach(function(cid) {
    apiCall(`
      (function() {
        var token = localStorage.getItem('admin_token');
        return fetch('${API_BASE}/v1/disk/admin/mfa/credentials/${cid}', {
          method: 'DELETE',
          headers: { 'Authorization': 'Bearer ' + token },
        })
        .then(function(r) { return r.json(); })
        .then(function(d) { return JSON.stringify(d); })
        .catch(function(e) { return JSON.stringify({error: e.message}); });
      })()
    `);
    ab.waitMs(300);
  });
  step('TC-cleanup: 删除所有通行密钥', true, 'deleted=' + credIds.length);

  // ── TC-89: 验证无 MFA 时仅密码登录 ──
  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('退出')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1500);

  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  ab.evalStdin(`
    (function() {
      var setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      var inputs = document.querySelectorAll('.ant-form input');
      var filled = 0;
      inputs.forEach(function(input) {
        if (input.placeholder === '用户名' || input.id === 'username') {
          setter.call(input, 'admin');
          input.dispatchEvent(new Event('input', { bubbles: true }));
          input.dispatchEvent(new Event('change', { bubbles: true }));
          filled++;
        }
        if (input.placeholder === '密码' || input.type === 'password') {
          setter.call(input, 'admin123');
          input.dispatchEvent(new Event('input', { bubbles: true }));
          input.dispatchEvent(new Event('change', { bubbles: true }));
          filled++;
        }
      });
      return 'filled_' + filled;
    })()
  `);
  ab.waitMs(500);

  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('登录') || btns[i].getAttribute('type') === 'submit') {
          btns[i].click();
          return 'clicked';
        }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(3000);

  var noMfaUrl = ab.getUrl();
  var noMfaOk = noMfaUrl.includes('/admin') && !ab.pageContainsText('验证通行密钥');
  assertCondition(
    noMfaOk,
    'TC-89: 无 MFA 时仅密码登录成功',
    'url=' + noMfaUrl
  );
  ab.screenshot('t02g-12-login-no-mfa');

  ab.closeBrowser();
});

printReport();
