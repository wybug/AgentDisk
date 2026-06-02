const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

describe('T02g: OAuth2 连接验证（只读）', () => {
  ab.closeBrowser();

  // Admin 登录获取 token
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

  var adminToken = (bootstrap.data || {}).token;
  if (!adminToken) {
    step('T02g-prep: Admin 登录失败', false, (bootstrap.message || 'no token'));
    printReport();
    return;
  }
  step('T02g-prep: Admin 登录成功', true);

  // T02g.1 - 验证 /auth/status 返回 oauth2=true
  var statusResult = apiCall(`
    (function() {
      return fetch('${API_BASE}/auth/status')
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var oauth2Enabled = (statusResult.data || {}).oauth2 === true;
  step('T02g.1: /auth/status 返回 oauth2=true', oauth2Enabled,
    'oauth2=' + ((statusResult.data || {}).oauth2));

  // T02g.2 - 访问前端首页验证重定向到网关登录页
  ab.open(WEB_BASE);
  ab.waitMs(3000);
  ab.waitLoad('networkidle');

  var currentUrl = ab.getUrl();
  var redirectedToGateway = currentUrl.includes('3100') && currentUrl.includes('login');
  var noDeadLoop = !currentUrl.includes('unavailable');
  step('T02g.2: 前端重定向到网关登录页（无死循环）', redirectedToGateway && noDeadLoop,
    'url=' + currentUrl);
  ab.screenshot('t02g-02-frontend-redirect');

  // T02g.3 - 测试 OAuth2 连接
  var testResult = apiCall(`
    (function() {
      return fetch('${API_BASE}/v1/disk/admin/oauth2/test', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ${adminToken}' },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  step('T02g.3: OAuth2 连接测试通过',
    (testResult.data || {}).status === 'ok',
    (JSON.stringify(testResult.data || testResult)).substring(0, 200));

  ab.closeBrowser();
});

printReport();
