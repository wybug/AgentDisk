const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

// Helper: evalStdin returning a JS expression that resolves to JSON string
// agent-browser wraps the result in an array, so we parse accordingly.
function apiCall(code) {
  var raw = ab.evalStdin(code);
  ab.waitMs(2000);
  if (!raw) return {};
  try {
    var parsed = JSON.parse(raw);
    if (typeof parsed === 'string') return JSON.parse(parsed);
    return parsed;
  } catch (e) {
    return { error: String(e), raw: raw };
  }
}

describe('T18: Admin Bootstrap', () => {
  ab.closeBrowser();

  // Open admin login page to establish browser context
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(3000);
  ab.waitLoad('networkidle');

  // ── TC-44.1: 调用 bootstrap API 创建首个 admin（已存在则跳过）──
  var bootstrap = apiCall(`
    (function() {
      return fetch('${API_BASE}/v1/disk/admin/bootstrap', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'admin123', displayName: 'Admin' }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var bootstrapOk = bootstrap.code === 0;
  var alreadyExists = bootstrap.code === 403 || String(bootstrap.message || '').includes('already');
  assertCondition(
    bootstrapOk || alreadyExists,
    'TC-44.1: Admin Bootstrap（创建或已存在）',
    bootstrapOk ? 'created' : (alreadyExists ? 'already_exists' : JSON.stringify(bootstrap).substring(0, 200))
  );
  ab.screenshot('t18-01-bootstrap');

  // ── TC-44.2: Admin 登录验证 ──
  var login = apiCall(`
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

  var loginOk = login.code === 0 && (login.data || {}).token;
  assertCondition(loginOk, 'TC-44.2: Admin 登录成功', 'token length: ' + ((login.data || {}).token || '').length);
  ab.screenshot('t18-02-login');

  // ── TC-44.3: Admin 登录页 UI 显示 ──
  var loginPageUrl = ab.getUrl();
  assertCondition(
    loginPageUrl.includes('/admin/login'),
    'TC-44.3: Admin 登录页显示',
    loginPageUrl
  );

  var hasAdminTitle = ab.pageContainsText('管理后台');
  assertCondition(hasAdminTitle, 'TC-44.3: 登录页显示管理后台标题');
  ab.screenshot('t18-03-login-page');

  // ── TC-44.4: UI 登录成功跳转 ──
  // Use React-compatible value setting via native input setter + synthetic events
  var uiFillResult = ab.evalStdin(`
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

  // Click submit button via DOM
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
  ab.waitMs(4000);
  ab.waitLoad('networkidle');

  var afterLoginUrl = ab.getUrl();
  var adminPageOk = afterLoginUrl.includes('/admin') && !afterLoginUrl.includes('/login');
  assertCondition(
    adminPageOk,
    'TC-44.4: UI 登录成功跳转到管理后台',
    'fill=' + uiFillResult + ' url=' + afterLoginUrl
  );
  ab.screenshot('t18-04-admin-page');

  // ── TC-44.5: 管理后台 Header 显示正常 ──
  if (adminPageOk) {
    var hasHeader = ab.pageContainsText('管理后台');
    assertCondition(hasHeader, 'TC-44.5: 管理后台 Header 显示正常');

    var hasLogoutBtn = ab.pageContainsText('退出');
    assertCondition(hasLogoutBtn, 'TC-44.5: 显示"退出"按钮');
  }
  ab.screenshot('t18-05-admin-header');

  // Cleanup: clear admin token and close browser
  ab.evalStdin(`localStorage.removeItem('admin_token')`);
  ab.closeBrowser();
});

printReport();
