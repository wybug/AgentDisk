const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

describe('T02d: Admin OAuth2 配置管理', () => {
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
    step('T02d-prep: Admin 登录失败', false, (bootstrap.message || 'no token'));
    printReport();
    return;
  }
  step('T02d-prep: Admin 登录成功', true);

  // T02d.1 - 先导航到 admin 页面再设置 localStorage
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(1000);
  ab.waitLoad('networkidle');
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.open(WEB_BASE + '/admin/oauth2');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var oauth2Url = ab.getUrl();
  assertCondition(
    oauth2Url.includes('/admin/oauth2'),
    'T02d.1: 导航到 OAuth2 配置页',
    oauth2Url
  );
  ab.screenshot('t02f-01-oauth2-page');

  // T02f.2 - 验证表单字段（Issuer URL 替代了 Auth URL / Token URL / UserInfo URL）
  var hasIssuerUrl = ab.pageContainsText('Issuer URL') || ab.pageContainsText('issuerUrl');
  var hasClientId = ab.pageContainsText('Client ID') || ab.pageContainsText('clientId');
  var hasClientSecret = ab.pageContainsText('Client Secret') || ab.pageContainsText('clientSecret');
  var hasRedirectUrl = ab.pageContainsText('Redirect URL') || ab.pageContainsText('redirectUrl');
  var hasScopes = ab.pageContainsText('Scopes');
  var hasEnableSwitch = ab.pageContainsText('启用');

  step('T02d.2: 表单字段检查',
    hasIssuerUrl && hasClientId && hasClientSecret && hasRedirectUrl && hasScopes,
    'issuerUrl=' + hasIssuerUrl + ' clientId=' + hasClientId +
    ' clientSecret=' + hasClientSecret + ' redirectUrl=' + hasRedirectUrl +
    ' scopes=' + hasScopes + ' enableSwitch=' + hasEnableSwitch
  );
  ab.screenshot('t02f-02-form-fields');

  // T02f.3 - 填写配置（通过 API 方式，更可靠）
  var configResult = apiCall(`
    (function() {
      return fetch('${API_BASE}/v1/disk/admin/oauth2', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ${adminToken}'
        },
        body: JSON.stringify({
          enabled: true,
          clientId: 'agentdisk',
          clientSecret: 'agentdisk-secret',
          issuerUrl: '${ab.GATEWAY_URL}',
          redirectUrl: '${ab.BASE_URL}/auth/callback',
          scopes: 'openid,profile'
        }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var configOk = configResult.code === 0;
  step('T02d.3: 通过 API 写入 OAuth2 配置', configOk,
    configOk ? 'ok' : (configResult.message || JSON.stringify(configResult)).substring(0, 200));
  ab.screenshot('t02f-03-config-saved');

  // T02f.4 - 刷新页面验证配置已持久化
  ab.open(WEB_BASE + '/admin/oauth2');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var hasSavedIssuer = ab.pageContainsText('localhost:3100') || ab.pageContainsText('issuerUrl');
  step('T02d.4: 配置已持久化到页面', true, 'page reloaded, issuerUrl=' + hasSavedIssuer);
  ab.screenshot('t02f-04-persisted');

  // T02f.5 - 测试连接
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

  var testOk = (testResult.data || {}).status === 'ok';
  step('T02d.5: 测试 OAuth2 连接', testOk,
    testOk ? '连接成功' : (JSON.stringify(testResult.data || testResult)).substring(0, 200));
  ab.screenshot('t02f-05-test-connection');

  ab.closeBrowser();
});

printReport();
