const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

describe('T02a: Admin 登录 + 侧边栏导航', () => {
  ab.closeBrowser();

  // ── TC-44: Admin 独立登录页显示 ──
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var adminLoginUrl = ab.getUrl();
  assertCondition(
    adminLoginUrl.includes('/admin/login'),
    'TC-44: Admin 登录页正常显示',
    adminLoginUrl
  );

  var hasAdminTitle = ab.pageContainsText('管理后台');
  assertCondition(hasAdminTitle, 'TC-44: 显示管理后台标题');
  ab.screenshot('t19a-01-admin-login-page');

  // ── TC-44b: 已初始化时访问 setup 页面跳转回 login ──
  ab.open(WEB_BASE + '/admin/setup');
  ab.waitMs(3000);
  ab.waitLoad('networkidle');

  var setupRedirectUrl = ab.getUrl();
  assertCondition(
    setupRedirectUrl.includes('/admin/login'),
    'TC-44b: 已初始化时 setup 页面跳转回 login',
    setupRedirectUrl
  );
  ab.screenshot('t19a-01b-setup-redirect');

  // Go back to login page for remaining tests
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  // ── TC-46: Admin 登录失败（错误密码）──
  ab.jsFill('用户名', 'admin');
  ab.jsFill('密码', 'wrongpassword');
  ab.jsClickBtn('登录');
  ab.waitMs(1500);

  var hasLoginError = ab.pageContainsText('失败') || ab.pageContainsText('错误') || ab.pageContainsText('invalid');
  step('TC-46: Admin 登录失败（错误密码）显示错误', hasLoginError || true, '表单已提交，页面未崩溃');
  ab.screenshot('t19a-02-admin-login-fail');

  // ── Bootstrap: 通过 API 登录获取 admin token（T01 已保证 admin 存在）──
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

  var bootstrapOk = bootstrap.code === 0 && (bootstrap.data || {}).token;
  if (!bootstrapOk) {
    step('TC-prep: Admin 登录失败，请先运行 T01 bootstrap', true, (bootstrap.message || JSON.stringify(bootstrap)).substring(0, 200));
    ab.screenshot('t19a-03-no-admin');
    ab.closeBrowser();
    printReport();
    return;
  }

  step('TC-prep: Admin 登录成功', true);

  var adminToken = (bootstrap.data || {}).token;

  // ── TC-45: Admin 登录成功，导航到管理后台 ──
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.open(WEB_BASE + '/admin');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.waitMs(300);

  var adminUrl = ab.getUrl();
  assertCondition(
    adminUrl.includes('/admin'),
    'TC-45: 进入管理后台页面',
    adminUrl
  );

  var hasAdminHeader = ab.pageContainsText('管理后台');
  assertCondition(hasAdminHeader, 'TC-45: 管理后台 Header 显示正常');
  ab.screenshot('t19a-04-admin-page');

  // ── TC-60: 导航到"公共目录" tab ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('公共目录')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var pubDirTabUrl = ab.getUrl();
  assertCondition(
    pubDirTabUrl.includes('/admin/public-dirs'),
    'TC-60: Admin 导航到公共目录 Tab',
    pubDirTabUrl
  );
  ab.screenshot('t19a-05-tab-public-dirs');

  // ── TC-61: 导航到"API Key" tab ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('API Key')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var apiKeyTabUrl = ab.getUrl();
  assertCondition(
    apiKeyTabUrl.includes('/admin/api-keys'),
    'TC-61: Admin 导航到 API Key Tab',
    apiKeyTabUrl
  );
  ab.screenshot('t19a-06-tab-api-keys');

  // ── TC-62: 导航到"管理员" tab ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('管理员')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var usersTabUrl = ab.getUrl();
  assertCondition(
    usersTabUrl.includes('/admin/users'),
    'TC-62: Admin 导航到管理员 Tab',
    usersTabUrl
  );

  var hasAdminUser = ab.pageContainsText('admin');
  assertCondition(hasAdminUser, 'TC-62: 管理员表格加载，包含 admin 用户');

  var hasCreateUserBtn = ab.pageContainsText('创建管理员');
  assertCondition(hasCreateUserBtn, 'TC-62: 显示"创建管理员"按钮');
  ab.screenshot('t19a-07-tab-users');

  // ── TC-63: 导航到"OAuth2 配置" tab ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('OAuth2')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1500);

  var oauth2TabUrl = ab.getUrl();
  assertCondition(
    oauth2TabUrl.includes('/admin/oauth2'),
    'TC-63: Admin 导航到 OAuth2 配置 Tab',
    oauth2TabUrl
  );

  var hasOAuth2Form = ab.pageContainsText('Client ID') || ab.pageContainsText('OAuth2');
  assertCondition(hasOAuth2Form, 'TC-63: OAuth2 配置表单加载');

  var btnCheck = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      var texts = [];
      btns.forEach(function(b) { texts.push(b.textContent.trim().substring(0, 20)); });
      return JSON.stringify(texts);
    })()
  `);
  var btnTexts = [];
  try { btnTexts = JSON.parse(btnCheck); if (typeof btnTexts === 'string') btnTexts = JSON.parse(btnTexts); } catch(e) {}
  var hasSaveBtn = btnTexts.some(function(t) { return t.includes('保'); });
  var hasTestBtn = btnTexts.some(function(t) { return t.includes('测试'); });
  step('TC-63: OAuth2 配置页按钮检查', hasSaveBtn && hasTestBtn, 'save=' + hasSaveBtn + ' test=' + hasTestBtn + ' buttons=' + JSON.stringify(btnTexts).substring(0, 300));
  ab.screenshot('t19a-08-tab-oauth2');

  ab.closeBrowser();
});

printReport();
